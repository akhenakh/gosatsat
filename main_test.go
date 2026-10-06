package main

import (
	"image"
	"math"
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

// TestSkyVisibilityIsTimezoneIndependent guards the sgp4 look-angle contract:
// GetLookAngle derives Greenwich sidereal time from the wall-clock fields of
// the time it is handed, so the sky view must normalize to UTC. Otherwise a
// local time value rotates the observer by the UTC offset and the Live tab
// lists satellites that are not actually above the horizon.
func TestSkyVisibilityIsTimezoneIndependent(t *testing.T) {
	loadDemoData()
	loc := st.location()
	sats := st.trackedSats()

	// Pick an instant where at least one tracked satellite is up.
	var utc time.Time
	for _, p := range st.passes {
		if len(visibleSatellites(loc, sats, p.Details.MaxElevationTime)) > 0 {
			utc = p.Details.MaxElevationTime
			break
		}
	}
	if utc.IsZero() {
		t.Fatal("no visible satellite found in demo passes")
	}

	// The same instant expressed in a non-UTC zone must yield the same result.
	other := utc.In(time.FixedZone("EDT", -4*3600))

	a := visibleSatellites(loc, sats, utc)
	b := visibleSatellites(loc, sats, other)
	if len(a) != len(b) {
		t.Fatalf("visible set depends on time zone: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].sat != b[i].sat || math.Abs(a[i].az-b[i].az) > 1e-9 || math.Abs(a[i].el-b[i].el) > 1e-9 {
			t.Fatalf("look angles depend on time zone:\n utc   %+v\n local %+v", a[i], b[i])
		}
	}
}

// TestOpenSatDetail covers resolving a live satellite to its pass: prefer the
// pass in progress, otherwise the next upcoming one, and do nothing when the
// satellite has no known pass.
func TestOpenSatDetail(t *testing.T) {
	loadDemoData()
	sat := st.sats[25544]
	if sat == nil {
		t.Fatal("demo ISS missing")
	}
	now := st.now
	mk := func(norad int, aos, los time.Time) *Pass {
		return &Pass{NoradID: norad, Name: sat.Name, Details: sgp4.PassDetails{AOS: aos, LOS: los}}
	}
	sooner := mk(sat.NoradID, now.Add(10*time.Minute), now.Add(20*time.Minute))
	later := mk(sat.NoradID, now.Add(40*time.Minute), now.Add(50*time.Minute))
	inProgress := mk(sat.NoradID, now.Add(-5*time.Minute), now.Add(5*time.Minute))
	otherSat := mk(99999, now.Add(-1*time.Minute), now.Add(9*time.Minute))
	st.passes = []*Pass{later, sooner, otherSat}

	// No pass for this satellite -> no-op.
	st.view = viewMain
	st.openSatDetail(nil)
	st.openSatDetail(&Sat{NoradID: 12345})
	if st.view != viewMain {
		t.Fatal("a satellite without a pass should not open the detail view")
	}

	// No in-progress pass -> the next upcoming pass.
	st.selPass = nil
	st.openSatDetail(sat)
	if st.view != viewPassDetail || st.selPass != sooner {
		t.Fatalf("expected the next pass, got view=%d sel=%+v", st.view, st.selPass)
	}

	// In-progress pass wins over the next one.
	st.passes = append(st.passes, inProgress)
	st.selPass = nil
	st.view = viewMain
	st.openSatDetail(sat)
	if st.view != viewPassDetail || st.selPass != inProgress {
		t.Fatalf("expected the in-progress pass, got view=%d sel=%+v", st.view, st.selPass)
	}
}

// TestGoBack covers closing the top of the view stack: the detail view and
// preferences return to the main view, anything else is a no-op.
func TestGoBack(t *testing.T) {
	st.view = viewPassDetail
	st.selPass = &Pass{}
	if !st.goBack() || st.view != viewMain || st.selPass != nil {
		t.Fatal("goBack should close the detail view")
	}

	st.openPrefs()
	if !st.goBack() || st.view != viewMain || st.prefs != nil {
		t.Fatal("goBack should close preferences")
	}

	st.view = viewMain
	if st.goBack() {
		t.Fatal("goBack on the main view should be a no-op")
	}
}

// TestEscapeGoesBack drives a real frame with the Escape key so the RootView
// wiring, not just goBack, is covered.
func TestEscapeGoesBack(t *testing.T) {
	loadDemoData()

	st.selPass = st.passes[0]
	st.view = viewPassDetail

	shirei.ResetInputSession()
	h := shirei.GetHost()
	h.WindowSize = shirei.Vec2{900, 620}
	h.WindowScale = 1
	h.ComfortScale = 1
	h.HardwareKeyboard = true

	shirei.GetFrameInput().Key = shirei.KeyEscape
	shirei.RunFrameFn(RootView)
	if st.view != viewMain || st.selPass != nil {
		t.Fatalf("Escape should close the detail view, got view=%d", st.view)
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

func TestDuePassNotification(t *testing.T) {
	loadDemoData()
	st.cfg.Notifications = boolPtr(true)
	now := st.now
	mk := func(aos time.Time) *Pass {
		return &Pass{NoradID: 1, Name: "TEST", Details: sgp4.PassDetails{AOS: aos, LOS: aos.Add(10 * time.Minute)}}
	}
	soon := mk(now.Add(4 * time.Minute))
	later := mk(now.Add(30 * time.Minute))
	st.passes = []*Pass{soon, later}

	n := st.duePassNotification(now)
	if n == nil || !n.aos.Equal(soon.Details.AOS) {
		t.Fatalf("expected the soon pass to be announced, got %+v", n)
	}
	if again := st.duePassNotification(now); again != nil {
		t.Fatalf("a pass should only be announced once: %+v", again)
	}
	if n2 := st.duePassNotification(now.Add(2 * time.Minute)); n2 != nil {
		t.Fatalf("the later pass is not due yet: %+v", n2)
	}

	st.cfg.Notifications = boolPtr(false)
	st.passes = []*Pass{mk(now.Add(time.Minute))}
	if st.duePassNotification(now) != nil {
		t.Fatal("notifications are disabled")
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
