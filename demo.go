package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/akhenakh/sgp4"
)

// Demo TLEs. These are the ISS element set used in the sgp4 test-suite; the
// clones differ only in NORAD id and orbit plane so the demo has several
// distinct tracks without needing a network fetch.
const (
	demoISSName = "ISS (ZARYA)"
	demoISS1    = "1 25544U 98067A   25247.10182809  .00011777  00000-0  21333-3 0  9997"
	demoISS2    = "2 25544  51.6327 275.9345 0004179 299.5263  60.5309 15.50088696528307"
)

// loadDemoData populates the app with deterministic data and computes passes
// synchronously, so `--demo --png` renders a fully populated interface offline.
func loadDemoData() {
	st.cfg = defaultConfig()
	st.cfg.Location = &LocationConfig{Name: "Zurich, Switzerland", Lat: 47.3769, Lng: 8.5417, Alt: 408}
	st.cfg.MinElevation = 10
	st.cfg.DarkMode = false
	st.now = time.Date(2025, 9, 5, 22, 0, 0, 0, time.UTC)

	tles := map[int]*sgp4.TLE{}
	add := func(name string, norad int, raan, ma float64) {
		l1, l2 := cloneTLE(demoISS1, demoISS2, norad, raan, ma)
		tle, err := sgp4.ParseTLELines([]string{name, l1, l2})
		if err != nil {
			return
		}
		tles[norad] = tle
	}
	add(demoISSName, 25544, -1, -1)
	add("SATELLITE-A", 90001, 335.9345, 200.0)
	add("SATELLITE-B", 90002, 35.9345, 300.0)

	transponders := map[int][]Transponder{
		25544: {
			{SatelliteName: demoISSName, NoradID: 25544, Uplink: "145.990", Downlink: "437.800", Mode: "FM", Callsign: "RS0ISS", Status: "active"},
			{SatelliteName: demoISSName, NoradID: 25544, Downlink: "145.800", Mode: "SSTV", Callsign: "RS0ISS", Status: "active"},
		},
	}
	st.mergeSatellites(tles, nil, transponders)

	st.cfg.TrackedSats = []int{25544, 90001, 90002}

	for _, sat := range st.sats {
		if lat, lng, alt, ok := livePosition(sat.TLE, st.now); ok {
			sat.Lat, sat.Lng, sat.Alt, sat.HasPos, sat.PosTime = lat, lng, alt, true, st.now
		}
	}

	passes := computePasses(st.location(), st.cfg.MinElevation, tles, st.now, st.now.Add(predictionWindowHours*time.Hour))
	st.passes = passes
	st.lastFetch = st.now
	st.view = viewMain
	st.tab = tabPasses
}

// cloneTLE rewrites the satellite number and optionally the RAAN and mean
// anomaly of a TLE pair, recomputing the checksums.
func cloneTLE(line1, line2 string, norad int, raan, meanAnomaly float64) (string, string) {
	num := fmt.Sprintf("%05d", norad)
	l1 := line1[:2] + num + line1[7:]
	l2 := line2[:2] + num + line2[7:]
	if raan >= 0 {
		l2 = l2[:17] + fmt.Sprintf("%8.4f", raan) + l2[25:]
	}
	if meanAnomaly >= 0 {
		l2 = l2[:43] + fmt.Sprintf("%8.4f", meanAnomaly) + l2[51:]
	}
	return fixChecksum(l1), fixChecksum(l2)
}

func fixChecksum(line string) string {
	if len(line) < 69 {
		return line
	}
	sum := 0
	for i := 0; i < 68; i++ {
		c := line[i]
		switch {
		case c >= '0' && c <= '9':
			sum += int(c - '0')
		case c == '-':
			sum++
		}
	}
	return line[:68] + strconv.Itoa(sum%10)
}
