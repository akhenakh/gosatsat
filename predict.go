package main

import (
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/akhenakh/sgp4"
)

// computePasses runs SGP4 over the given window for every satellite, filters by
// peak elevation and returns passes sorted by AOS. It is safe to call from a
// background goroutine (inputs are plain values).
func computePasses(loc sgp4.Location, minElevation float64, tles map[int]*sgp4.TLE, start, stop time.Time) []*Pass {
	ids := make([]int, 0, len(tles))
	for id := range tles {
		ids = append(ids, id)
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	results := make(chan []*Pass, len(ids))
	var wg sync.WaitGroup

	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tle := tles[id]
			passes, err := tle.GeneratePasses(loc.Latitude, loc.Longitude, loc.Altitude, start, stop, predictionStepSeconds)
			if err != nil {
				return
			}
			local := make([]*Pass, 0, len(passes))
			for _, p := range passes {
				if p.MaxElevation < minElevation {
					continue
				}
				local = append(local, &Pass{
					NoradID: id,
					Name:    tleName(tle),
					Details: p,
				})
			}
			results <- local
		}(id)
	}
	wg.Wait()
	close(results)

	var all []*Pass
	for r := range results {
		all = append(all, r...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Details.AOS.Before(all[j].Details.AOS)
	})
	return all
}

func tleName(tle *sgp4.TLE) string {
	if tle == nil {
		return ""
	}
	return tle.Name
}

// mergePasses reuses existing Pass objects that match by (NORAD, AOS) so the
// current selection and any generated plot survive a refresh.
func mergePasses(old, fresh []*Pass) []*Pass {
	index := make(map[[2]int64]*Pass, len(old))
	for _, p := range old {
		index[passKey(p)] = p
	}
	for _, p := range fresh {
		k := passKey(p)
		if prev, ok := index[k]; ok {
			// keep cached data/plot, refresh geometry
			prev.Name = p.Name
			prev.Details = p.Details
			p = prev
		}
		index[k] = p
	}
	out := make([]*Pass, 0, len(fresh))
	for _, p := range fresh {
		out = append(out, index[passKey(p)])
	}
	return out
}

func passKey(p *Pass) [2]int64 {
	return [2]int64{int64(p.NoradID), p.Details.AOS.Unix()}
}

// buildDetailPoints samples the pass at a fine step for the polar plot.
func buildDetailPoints(tle *sgp4.TLE, loc sgp4.Location, p *Pass) []sgp4.PassDataPoint {
	if tle == nil {
		return nil
	}
	var pts []sgp4.PassDataPoint
	step := time.Duration(detailStepSeconds) * time.Second
	for t := p.Details.AOS; t.Before(p.Details.LOS) || t.Equal(p.Details.LOS); t = t.Add(step) {
		eci, err := tle.FindPositionAtTime(t)
		if err != nil {
			continue
		}
		sv := &sgp4.StateVector{
			X: eci.Position.X, Y: eci.Position.Y, Z: eci.Position.Z,
			VX: eci.Velocity.X, VY: eci.Velocity.Y, VZ: eci.Velocity.Z,
		}
		obs, err := sv.GetLookAngle(&loc, t)
		if err != nil {
			continue
		}
		pts = append(pts, sgp4.PassDataPoint{
			Timestamp: t.UTC(),
			Azimuth:   obs.LookAngles.Azimuth,
			Elevation: obs.LookAngles.Elevation,
			Range:     obs.LookAngles.Range,
			RangeRate: obs.LookAngles.RangeRate,
		})
	}
	return pts
}

// livePosition computes the sub-satellite point for a TLE at time t.
func livePosition(tle *sgp4.TLE, t time.Time) (lat, lng, alt float64, ok bool) {
	if tle == nil {
		return 0, 0, 0, false
	}
	eci, err := tle.FindPositionAtTime(t)
	if err != nil {
		return 0, 0, 0, false
	}
	la, lo, al := eci.ToGeodetic()
	return la, lo, al, true
}
