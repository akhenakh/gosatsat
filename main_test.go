package main

import (
	"image"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akhenakh/sgp4"
	shirei "go.hasen.dev/shirei"
)

func TestDemoRenderMain(t *testing.T) {
	loadDemoData()
	img := shirei.RenderToImage(900, 620, RootView)
	if img == nil {
		t.Fatal("render returned nil")
	}
	if img.Bounds().Dx() < 800 {
		t.Fatalf("unexpected render size: %v", img.Bounds())
	}
}

func TestPassDetailPlot(t *testing.T) {
	loadDemoData()
	var pass *Pass
	for _, p := range st.sortedPasses() {
		if p.Details.AOS.After(st.now) {
			pass = p
			break
		}
	}
	if pass == nil {
		t.Fatal("no upcoming pass in demo data")
	}
	plot := ensurePassPlot(pass, 300)
	if plot == nil || plot.Bounds() != image.Rect(0, 0, 300, 300) {
		t.Fatalf("bad plot bounds: %v", plot)
	}
	if len(pass.DataPoints) < 2 {
		t.Fatalf("expected sampled data points, got %d", len(pass.DataPoints))
	}
}

func TestMapBaseNotRerendered(t *testing.T) {
	loadDemoData()
	st.mapBase = image.NewRGBA(image.Rect(0, 0, 64, 36))
	st.mapInit = true
	st.mapCenterLat = st.cfg.Location.Lat
	st.mapCenterLng = st.cfg.Location.Lng
	st.mapBaseCenterLat = st.mapCenterLat
	st.mapBaseCenterLng = st.mapCenterLng
	st.mapBaseZoom = st.mapZoom
	st.mapBaseKey = mapKey(st.mapCenterLat, st.mapCenterLng, st.mapZoom)
	st.tab = tabMap

	before := mapRenderCalls.Load()
	for i := 0; i < 3; i++ {
		shirei.RenderToImage(800, 500, RootView)
	}
	if got := mapRenderCalls.Load() - before; got != 0 {
		t.Fatalf("base map was re-rendered %d time(s) across frames", got)
	}
}

func TestSkyPlot(t *testing.T) {
	loadDemoData()
	img := skyPlotImage(240, st.location(), st.trackedSats(), st.now, false)
	if img == nil || img.Bounds() != image.Rect(0, 0, 240, 240) {
		t.Fatalf("bad sky plot: %v", img)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := defaultConfig()
	cfg.Location = &LocationConfig{Name: "Test", Lat: 1.5, Lng: -2.5, Alt: 12}
	cfg.TrackedSats = []int{25544}
	cfg.MinElevation = 15
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Fatal(err)
	}

	got, onboarding, warning := loadConfig()
	if warning != nil {
		t.Fatalf("unexpected warning: %v", warning)
	}
	if onboarding {
		t.Fatal("config with a location should not need onboarding")
	}
	if got.Location == nil || got.Location.Lat != 1.5 || got.MinElevation != 15 {
		t.Fatalf("config not round-tripped: %+v", got)
	}
	if len(got.TrackedSats) != 1 || got.TrackedSats[0] != 25544 {
		t.Fatalf("tracked sats not persisted: %+v", got.TrackedSats)
	}
}

func TestTLECacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	tle, err := sgp4.ParseTLELines([]string{demoISSName, demoISS1, demoISS2})
	if err != nil {
		t.Fatalf("parse demo TLE: %v", err)
	}
	tles := map[int]*sgp4.TLE{tle.SatelliteNumber: tle}
	tps := map[int][]Transponder{25544: {{Mode: "FM", Downlink: "145.800", Status: "active"}}}
	if err := saveTLECache(tles, tps); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	gotTLEs, names, gotTps, savedAt, err := loadTLECache()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if len(gotTLEs) != 1 || gotTLEs[25544] == nil {
		t.Fatalf("cached TLEs not restored: %v", gotTLEs)
	}
	if names[strings.ToUpper(demoISSName)] != 25544 {
		t.Fatalf("name index not restored: %v", names)
	}
	if len(gotTps[25544]) != 1 {
		t.Fatalf("transponders not restored: %v", gotTps)
	}
	if savedAt.IsZero() {
		t.Fatal("saved timestamp missing")
	}
	if got := gotTLEs[25544]; got.EpochYear != tle.EpochYear || got.SatelliteNumber != 25544 {
		t.Fatalf("cached TLE fields differ: %+v", got)
	}
}

func TestParseGPCSVSixDigit(t *testing.T) {
	data := `OBJECT_NAME,OBJECT_ID,EPOCH,MEAN_MOTION,ECCENTRICITY,INCLINATION,RA_OF_ASC_NODE,ARG_OF_PERICENTER,MEAN_ANOMALY,EPHEMERIS_TYPE,CLASSIFICATION_TYPE,NORAD_CAT_ID,ELEMENT_SET_NO,REV_AT_EPOCH,BSTAR,MEAN_MOTION_DOT,MEAN_MOTION_DDOT
TEST-SAT,2026-001A,2026-10-05T06:00:02.999808,15.33987296,.00014793,53.1614,120.4004,41.5220,36.3784,0,U,100953,999,576,.1629853E-2,.54371E-3,0
`
	tles, names, err := parseOrbitalData([]byte(data))
	if err != nil {
		t.Fatalf("parse GP CSV: %v", err)
	}
	tle := tles[100953]
	if tle == nil {
		t.Fatalf("6-digit catalog number not parsed: %v", tles)
	}
	if tle.SatelliteNumber != 100953 {
		t.Fatalf("catalog number: got %d", tle.SatelliteNumber)
	}
	if names["TEST-SAT"] != 100953 {
		t.Fatalf("name index: %v", names)
	}
	if tle.EpochYear != 2026 {
		t.Fatalf("epoch year: got %d", tle.EpochYear)
	}
}

func TestParseOMMJSONSixDigit(t *testing.T) {
	data := `[{"OBJECT_NAME":"JSON-SAT","OBJECT_ID":"2026-001A","EPOCH":"2026-10-05T06:00:02.999808","MEAN_MOTION":15.33987296,"ECCENTRICITY":0.00014793,"INCLINATION":53.1614,"RA_OF_ASC_NODE":120.4004,"ARG_OF_PERICENTER":41.522,"MEAN_ANOMALY":36.3784,"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":100954,"ELEMENT_SET_NO":999,"REV_AT_EPOCH":576,"BSTAR":0.001629853,"MEAN_MOTION_DOT":0.00054371,"MEAN_MOTION_DDOT":0}]`
	tles, _, err := parseOrbitalData([]byte(data))
	if err != nil {
		t.Fatalf("parse OMM JSON: %v", err)
	}
	if tles[100954] == nil {
		t.Fatalf("JSON OMM 6-digit catalog not parsed: %v", tles)
	}
}

func TestParseOrbitalDataTLE(t *testing.T) {
	data := demoISSName + "\n" + demoISS1 + "\n" + demoISS2 + "\n"
	tles, names, err := parseOrbitalData([]byte(data))
	if err != nil {
		t.Fatalf("parse TLE: %v", err)
	}
	if tles[25544] == nil {
		t.Fatalf("TLE source not parsed: %v", tles)
	}
	if names[strings.ToUpper(demoISSName)] != 25544 {
		t.Fatalf("name index: %v", names)
	}
}

func TestMigrateTleSource(t *testing.T) {
	old := "https://celestrak.org/NORAD/elements/gp.php?GROUP=active&FORMAT=tle"
	got := migrateTleSource(old)
	if !strings.Contains(got, "FORMAT=csv") {
		t.Fatalf("CelesTrak TLE source not migrated: %s", got)
	}
	same := migrateTleSource("https://www.amsat.org/tle/dailytle.txt")
	if same != "https://www.amsat.org/tle/dailytle.txt" {
		t.Fatalf("non-CelesTrak source changed: %s", same)
	}
}

func TestCloneTLEChecksums(t *testing.T) {
	l1, l2 := cloneTLE(demoISS1, demoISS2, 90001, 335.9, 200.0)
	tle, err := sgp4.ParseTLELines([]string{"CLONE", l1, l2})
	if err != nil {
		t.Fatalf("clone did not parse: %v", err)
	}
	if tle.SatelliteNumber != 90001 {
		t.Fatalf("wrong sat number: %d", tle.SatelliteNumber)
	}
}

func TestCountdownUsesStateClock(t *testing.T) {
	loadDemoData()
	future := st.now.Add(2*time.Hour + 5*time.Minute)
	if got := countdown(future); got != "2h 05m" {
		t.Fatalf("countdown: got %q", got)
	}
	if got := countdown(st.now.Add(-time.Hour)); got != "passed" {
		t.Fatalf("past countdown: got %q", got)
	}
}
