package main

import (
	"image"
	"sort"
	"time"

	"github.com/akhenakh/sgp4"
)

const (
	predictionWindowHours = 72
	predictionStepSeconds = 60
	detailStepSeconds     = 5
	maxTrackedSats        = 50
)

// UI views.
const (
	viewLoading = iota
	viewOnboarding
	viewMain
	viewPrefs
	viewPassDetail
)

// Main tabs.
const (
	tabPasses = iota
	tabMap
	tabSky
)

// Transponder holds data for a single satellite transponder (ported from websat).
type Transponder struct {
	SatelliteName string
	NoradID       int
	Uplink        string
	Downlink      string
	Beacon        string
	Mode          string
	Callsign      string
	Status        string
}

// Sat is an app-owned stable satellite object. It survives TLE refreshes and is
// used as the row/selection identity.
type Sat struct {
	NoradID      int
	Name         string
	TLE          *sgp4.TLE
	Transponders []Transponder

	// Live sub-satellite point, refreshed by the ticker.
	Lat, Lng, Alt float64
	HasPos        bool
}

// Pass is an app-owned stable predicted pass.
type Pass struct {
	NoradID int
	Name    string
	Details sgp4.PassDetails

	// Lazily generated detail plot data.
	DataPoints []sgp4.PassDataPoint
	DataLoaded bool

	// notified is set once the "pass soon" OS notification has been sent.
	notified bool

	plot     *image.RGBA
	plotSize int
	plotDark bool
}

// State is the whole application state. It is only mutated on the UI thread or
// under WithFrameLock from background goroutines.
type State struct {
	cfg     Config
	cfgWarn string

	view int
	tab  int

	sats     map[int]*Sat
	satOrder []*Sat

	passes      []*Pass
	passesDirty bool
	passGen     int

	fetching    bool
	fetchErrs   []string
	fetchNotice string
	lastFetch   time.Time
	lastAttempt time.Time
	fromCache   bool

	now time.Time

	selPass *Pass
	selSat  *Sat

	// Onboarding wizard state.
	onboardStep  int
	obCityQuery  string
	obLat        string
	obLng        string
	obAlt        string
	obCityName   string
	obUseCity    bool
	obTracked    map[int]bool
	obActiveOnly bool
	obSatQuery   string
	obErr        string

	// Preferences draft.
	prefs *prefsDraft

	// Map tab.
	mapBase          *image.RGBA
	mapBaseKey       string
	mapBaseCenterLat float64
	mapBaseCenterLng float64
	mapBaseZoom      int
	mapCenterLat     float64
	mapCenterLng     float64
	mapZoom          int
	mapInit          bool
	mapRendering     bool
	mapErr           string

	// Sky (live polar) tab.
	skyImg  *image.RGBA
	skyPx   int
	skyKey  int64
	skyDark bool
}

// prefsDraft is a mutable copy of the config being edited in the preferences view.
type prefsDraft struct {
	locationName  string
	lat, lng, alt string
	minElevation  float64
	tleSources    []string
	xpSources     []string
	refreshHours  int
	darkMode      bool
	notifications bool
	cityQuery     string
	satQuery      string
	activeOnly    bool
	tracked       map[int]bool
	err           string
}

func newState() *State {
	return &State{
		sats:      map[int]*Sat{},
		obTracked: map[int]bool{},
		view:      viewLoading,
		now:       time.Now(),
		mapZoom:   2,
	}
}

func (s *State) location() sgp4.Location {
	if s.cfg.Location == nil {
		return sgp4.Location{}
	}
	return sgp4.Location{Latitude: s.cfg.Location.Lat, Longitude: s.cfg.Location.Lng, Altitude: s.cfg.Location.Alt}
}

func (s *State) locationName() string {
	if s.cfg.Location == nil {
		return "No location set"
	}
	if s.cfg.Location.Name != "" {
		return s.cfg.Location.Name
	}
	return formatLatLng(s.cfg.Location.Lat, s.cfg.Location.Lng)
}

func (s *State) trackedIDs() []int {
	out := make([]int, 0, len(s.cfg.TrackedSats))
	out = append(out, s.cfg.TrackedSats...)
	return out
}

func (s *State) isTracked(norad int) bool {
	for _, id := range s.cfg.TrackedSats {
		if id == norad {
			return true
		}
	}
	return false
}

func (s *State) trackedSats() []*Sat {
	var out []*Sat
	for _, id := range s.cfg.TrackedSats {
		if sat, ok := s.sats[id]; ok {
			out = append(out, sat)
		}
	}
	return out
}

func (s *State) getSat(norad int) *Sat {
	return s.sats[norad]
}

// sortedPasses returns passes filtered to tracked satellites, sorted by AOS.
func (s *State) sortedPasses() []*Pass {
	out := make([]*Pass, 0, len(s.passes))
	for _, p := range s.passes {
		if s.isTracked(p.NoradID) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Details.AOS.Before(out[j].Details.AOS)
	})
	return out
}
