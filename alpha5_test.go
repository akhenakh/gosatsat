package main

import "testing"

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
