package main

import (
	"testing"
	"time"

	"github.com/akhenakh/sgp4"
)

// tleLinesWithCatalog substitutes the catalog field of the demo ISS element set
// and recomputes the checksums. The field is encoded by the library, so these
// fixtures also exercise the Alpha-5 writer against the parser.
func tleLinesWithCatalog(t *testing.T, norad int) (string, string) {
	t.Helper()
	field, err := sgp4.FormatAlpha5(norad)
	if err != nil {
		t.Fatalf("format catalog number %d: %v", norad, err)
	}
	l1 := demoISS1[:2] + field + demoISS1[7:]
	l2 := demoISS2[:2] + field + demoISS2[7:]
	return fixChecksum(l1), fixChecksum(l2)
}

// A TLE-text source can carry catalog numbers above 99999 in Alpha-5 form (a
// letter, then four digits: A0000 is 100000). CelesTrak's CSV/OMM path already
// handles six-digit numbers; this guards the plain-TLE fallback used for AMSAT
// and any user-added 3LE source.
func TestParseOrbitalDataAlpha5TLE(t *testing.T) {
	data := "SARAMAGO\n" +
		"1 A0000U 26067CY  26195.90649229  .00004770  00000+0  22159-3 0  9994\n" +
		"2 A0000  97.4593 154.0970 0005590 270.5113  89.5482 15.20467281 15911\n"
	tles, names, err := parseOrbitalData([]byte(data))
	if err != nil {
		t.Fatalf("parse Alpha-5 TLE: %v", err)
	}
	tle := tles[100000]
	if tle == nil {
		t.Fatalf("Alpha-5 catalog number not parsed: %v", tles)
	}
	if names["SARAMAGO"] != 100000 {
		t.Fatalf("name index: %v", names)
	}
	if _, err := tle.FindPosition(0); err != nil {
		t.Fatalf("propagate Alpha-5 set: %v", err)
	}
}

// The catalog field holds five characters, so every value above 99999 lands on
// a letter prefix (A0000 is 100000, Z9999 is 339999) and the boundary cases also
// cross digit carries and the letters I and O, which Alpha-5 skips. Each set is
// encoded by the library, goes through the app's TLE parser, so checksum
// validation covers the field too, and then through a full position computation.
func TestAlpha5CatalogRange(t *testing.T) {
	start := time.Date(2025, 9, 5, 22, 0, 0, 0, time.UTC)
	cases := []struct {
		field string
		want  int
	}{
		{"99999", 99999}, // last plain-digit number
		{"A0000", 100000},
		{"A9999", 109999},
		{"B0000", 110000},
		{"C3456", 123456},
		{"E8493", 148493},
		{"J2931", 182931}, // H, I skipped: J continues the sequence
		{"P4018", 234018}, // N, O skipped
		{"W1928", 301928},
		{"Z0000", 330000},
		{"Z9999", 339999}, // ceiling of the five-character field
	}
	for _, tc := range cases {
		field, err := sgp4.FormatAlpha5(tc.want)
		if err != nil {
			t.Fatalf("%d: format: %v", tc.want, err)
		}
		if field != tc.field {
			t.Fatalf("%d: encoded as %q, want %q", tc.want, field, tc.field)
		}
		l1, l2 := tleLinesWithCatalog(t, tc.want)
		if len(l1) != 69 || len(l2) != 69 {
			t.Fatalf("%s: generated %d/%d-char lines", tc.field, len(l1), len(l2))
		}
		tles, names, err := parseOrbitalData([]byte("HIGH\n" + l1 + "\n" + l2 + "\n"))
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.field, err)
		}
		tle := tles[tc.want]
		if tle == nil {
			t.Fatalf("%s: catalog number %d not parsed: %v", tc.field, tc.want, tles)
		}
		if names["HIGH"] != tc.want {
			t.Fatalf("%s: name index: %v", tc.field, names)
		}
		lat, lng, _, ok := livePosition(tle, start)
		if !ok {
			t.Fatalf("%s: no position", tc.field)
		}
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			t.Fatalf("%s: position out of range: %f %f", tc.field, lat, lng)
		}
	}

	// A two-line block without a name line is accepted too.
	l1, l2 := tleLinesWithCatalog(t, 123456)
	if tles, _, err := parseOrbitalData([]byte(l1 + "\n" + l2 + "\n")); err != nil {
		t.Fatalf("parse nameless Alpha-5 set: %v", err)
	} else if tles[123456] == nil {
		t.Fatalf("nameless Alpha-5 set not parsed: %v", tles)
	}
}

// OMM/GP carries the catalog number as a number, not as a five-character field,
// so it goes past the Alpha-5 ceiling that applies to TLE text.
func TestParseOMMBeyondAlpha5Ceiling(t *testing.T) {
	data := `[{"OBJECT_NAME":"FAR-SAT","OBJECT_ID":"2026-001A","EPOCH":"2026-10-05T06:00:02.999808","MEAN_MOTION":15.33987296,"ECCENTRICITY":0.00014793,"INCLINATION":53.1614,"RA_OF_ASC_NODE":120.4004,"ARG_OF_PERICENTER":41.522,"MEAN_ANOMALY":36.3784,"EPHEMERIS_TYPE":0,"CLASSIFICATION_TYPE":"U","NORAD_CAT_ID":400001,"ELEMENT_SET_NO":999,"REV_AT_EPOCH":576,"BSTAR":0.001629853,"MEAN_MOTION_DOT":0.00054371,"MEAN_MOTION_DDOT":0}]`
	tles, names, err := parseOrbitalData([]byte(data))
	if err != nil {
		t.Fatalf("parse OMM JSON: %v", err)
	}
	if tles[400001] == nil {
		t.Fatalf("OMM catalog number above the Alpha-5 ceiling not parsed: %v", tles)
	}
	if names["FAR-SAT"] != 400001 {
		t.Fatalf("name index: %v", names)
	}
}

// A satellite above 99999 must behave like any other through the app: it merges
// into the satellite store, gets live positions, and predicts passes and detail
// points.
func TestAlpha5SatelliteEndToEnd(t *testing.T) {
	prev := st
	defer func() { st = prev }()
	st = newState()
	st.cfg = defaultConfig()
	st.cfg.Location = &LocationConfig{Name: "Zurich, Switzerland", Lat: 47.3769, Lng: 8.5417, Alt: 408}

	l1, l2 := tleLinesWithCatalog(t, 339999)
	tles, names, err := parseOrbitalData([]byte("HIGH-SAT\n" + l1 + "\n" + l2 + "\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	st.mergeSatellites(tles, names, nil)

	sat := st.sats[339999]
	if sat == nil {
		t.Fatalf("high catalog number missing from the satellite store: %v", st.sats)
	}
	if sat.Name != "HIGH-SAT" || sat.TLE == nil || sat.TLE.SatelliteNumber != 339999 {
		t.Fatalf("bad satellite: %+v", sat)
	}
	start := time.Date(2025, 9, 5, 22, 0, 0, 0, time.UTC)
	if lat, lng, _, ok := livePosition(sat.TLE, start); !ok {
		t.Fatalf("no live position for %d", sat.NoradID)
	} else if lat == 0 && lng == 0 {
		t.Fatalf("degenerate live position for %d", sat.NoradID)
	}

	passes := computePasses(st.location(), 0, tles, start, start.Add(predictionWindowHours*time.Hour))
	var pass *Pass
	for _, p := range passes {
		if p.NoradID == 339999 {
			pass = p
			break
		}
	}
	if pass == nil {
		t.Fatalf("no passes for the high catalog number: %d passes total", len(passes))
	}
	if pass.Name != "HIGH-SAT" {
		t.Fatalf("pass name: %q", pass.Name)
	}
	if pts := buildDetailPoints(sat.TLE, st.location(), pass); len(pts) == 0 {
		t.Fatal("no detail points for the high catalog number")
	}
}

// The cache stores parsed element sets, not TLE text, so a catalog number with
// no TLE representation must survive a save/load unchanged.
func TestTLECacheKeepsCatalogBeyondAlpha5(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	tle, err := sgp4.ParseTLELines([]string{demoISSName, demoISS1, demoISS2})
	if err != nil {
		t.Fatalf("parse demo TLE: %v", err)
	}
	far := *tle
	far.Name = "FAR-SAT"
	far.SatelliteNumber = 400001
	if err := saveTLECache(map[int]*sgp4.TLE{400001: &far}, nil); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	tles, names, _, _, err := loadTLECache()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if got := tles[400001]; got == nil || got.Name != "FAR-SAT" {
		t.Fatalf("high catalog number lost in the cache: %v", tles)
	}
	if names["FAR-SAT"] != 400001 {
		t.Fatalf("name index: %v", names)
	}
}

// Tracked satellite ids are plain ints in the config, with no 5-digit cap.
func TestConfigKeepsHighCatalogIDs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := defaultConfig()
	cfg.Location = &LocationConfig{Name: "Test", Lat: 1.5, Lng: -2.5, Alt: 12}
	cfg.TrackedSats = []int{25544, 100000, 339999}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, _, warning := loadConfig()
	if warning != nil {
		t.Fatalf("unexpected warning: %v", warning)
	}
	if len(got.TrackedSats) != 3 || got.TrackedSats[1] != 100000 || got.TrackedSats[2] != 339999 {
		t.Fatalf("high catalog ids not persisted: %v", got.TrackedSats)
	}
}
