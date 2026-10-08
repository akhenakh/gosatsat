package main

import (
	"context"
	"fmt"
	"image"
	"math"
	"time"

	"github.com/akhenakh/sgp4"
	. "go.hasen.dev/shirei"
)

// st is the single, global application state. It is only touched on the UI
// thread or under WithFrameLock.
var st = newState()

func formatLatLng(lat, lng float64) string {
	ns := "N"
	if lat < 0 {
		ns = "S"
	}
	ew := "E"
	if lng < 0 {
		ew = "W"
	}
	return fmt.Sprintf("%.3f°%s %.3f°%s", math.Abs(lat), ns, math.Abs(lng), ew)
}

// minRefreshInterval is the shortest interval between two network refreshes of
// the same data. CelesTrak asks that General Perturbations queries are made no
// more than once every two hours; the periodic refresh already exceeds this,
// and this guard also covers restarts and manual refreshes.
const minRefreshInterval = 2 * time.Hour

// requestRefresh is the UI entry point: it respects the minimum interval and
// tells the user when the cached elements are still fresh.
func requestRefresh() {
	if !st.lastAttempt.IsZero() {
		if age := time.Since(st.lastAttempt); age < minRefreshInterval {
			st.fetchNotice = fmt.Sprintf("Elements were requested %s ago; CelesTrak asks for at most one request every %s.", age.Truncate(time.Minute), minRefreshInterval)
			RequestNextFrame()
			return
		}
	}
	st.fetchNotice = ""
	startFetch()
}

// maybeFetch starts a fetch only when the minimum interval has elapsed.
func maybeFetch() {
	need := false
	WithFrameLock(func() {
		need = st.lastAttempt.IsZero() || time.Since(st.lastAttempt) >= minRefreshInterval
	})
	if need {
		startFetch()
	}
}

// startFetch downloads all TLE and transponder sources, merges them into the
// stable store and recomputes passes. It runs both from the UI thread and from
// the periodic refresh goroutine, so the fetching guard and the config snapshot
// are taken under the frame lock.
func startFetch() {
	var cfg Config
	started := false
	WithFrameLock(func() {
		if st.fetching {
			return
		}
		st.fetching = true
		st.fetchErrs = nil
		st.fetchNotice = ""
		cfg = st.cfg
		started = true
	})
	if !started {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		tles, names, errs := fetchTLEs(ctx, cfg.TleSources)
		tps, tpErrs := fetchTransponders(ctx, cfg.TransponderSources)
		errs = append(errs, tpErrs...)

		var effectiveXp map[int][]Transponder
		WithFrameLock(func() {
			st.mergeSatellites(tles, names, tps)
			st.fetching = false
			st.fetchErrs = errs
			st.lastFetch = time.Now()
			st.lastAttempt = st.lastFetch
			if st.view == viewLoading {
				st.view = viewOnboarding
			}
			if len(tles) > 0 {
				st.fromCache = false
				effectiveXp = make(map[int][]Transponder)
				for id, sat := range st.sats {
					if len(sat.Transponders) > 0 {
						effectiveXp[id] = sat.Transponders
					}
				}
			}
		})
		// Persist the freshly downloaded elements so the next start can show
		// data while this fetch runs. Only replace the cache when the fetch
		// actually produced TLEs.
		if len(tles) > 0 {
			cacheErr := saveTLECache(tles, effectiveXp)
			if cacheErr != nil {
				WithFrameLock(func() {
					st.fetchErrs = append(st.fetchErrs, "cache: "+cacheErr.Error())
				})
			}
		}
		RequestNextFrame()
		requestPassRecompute()
	}()
}

// startRefreshLoop refetches the orbital data periodically.
func startRefreshLoop() {
	go func() {
		hours := st.cfg.RefreshHours
		if hours <= 0 {
			hours = defaultRefreshHours
		}
		ticker := time.NewTicker(time.Duration(hours) * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			maybeFetch()
		}
	}()
}

// startPassRefresh periodically recomputes passes so the 72h window slides
// forward as time passes.
func startPassRefresh() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			requestPassRecompute()
		}
	}()
}

// startNowTicker keeps the wall clock and live satellite positions fresh.
func startNowTicker() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			var notice *passNotice
			WithFrameLock(func() {
				st.now = time.Now()
				for _, sat := range st.sats {
					if sat.TLE == nil {
						continue
					}
					lat, lng, alt, ok := livePosition(sat.TLE, st.now)
					if ok {
						sat.Lat, sat.Lng, sat.Alt = lat, lng, alt
						sat.HasPos = true
						sat.PosTime = st.now
					}
				}
				notice = st.duePassNotification(st.now)
			})
			if notice != nil {
				announcePass(notice)
			}
			RequestNextFrame()
		}
	}()
}

// requestPassRecompute recomputes passes from a background context, taking the
// frame lock to snapshot the inputs.
func requestPassRecompute() {
	WithFrameLock(func() { recomputePasses() })
}

// recomputePasses starts a background pass computation for the tracked sats.
// The caller must hold the frame lock (the UI thread does; background callers
// use requestPassRecompute). The computation itself runs off the lock.
func recomputePasses() {
	if st.cfg.Location == nil {
		return
	}
	st.passGen++
	gen := st.passGen

	loc := st.location()
	minEl := st.cfg.MinElevation
	tles := make(map[int]*sgp4.TLE)
	for _, id := range st.cfg.TrackedSats {
		if sat := st.sats[id]; sat != nil && sat.TLE != nil {
			tles[id] = sat.TLE
		}
	}
	if len(tles) == 0 {
		st.passes = nil
		RequestNextFrame()
		return
	}

	start := time.Now().UTC()
	stop := start.Add(predictionWindowHours * time.Hour)

	go func() {
		passes := computePasses(loc, minEl, tles, start, stop)
		WithFrameLock(func() {
			if gen != st.passGen {
				return
			}
			st.passes = mergePasses(st.passes, passes)
			st.passesDirty = false
		})
		RequestNextFrame()
	}()
}

// ensurePassPlot lazily fills a pass's fine-grained data points and renders its
// polar plot. Must be called from the UI thread.
func ensurePassPlot(p *Pass, size int) *image.RGBA {
	if p == nil {
		return nil
	}
	if !p.DataLoaded {
		if sat := st.sats[p.NoradID]; sat != nil && sat.TLE != nil {
			p.DataPoints = buildDetailPoints(sat.TLE, st.location(), p)
		}
		p.DataLoaded = true
	}
	if p.plot == nil || p.plotSize != size || p.plotDark != st.cfg.DarkMode {
		p.plot = passPlotImage(p, size, st.cfg.DarkMode)
		p.plotSize = size
		p.plotDark = st.cfg.DarkMode
	}
	return p.plot
}

// ensureSkyPlot returns the live sky polar image, regenerated about once a second.
func ensureSkyPlot(size int) *image.RGBA {
	key := st.now.Unix()
	if st.skyImg == nil || st.skyPx != size || st.skyKey != key || st.skyDark != st.cfg.DarkMode {
		st.skyImg = skyPlotImage(size, st.location(), st.trackedSats(), st.now, st.cfg.DarkMode)
		st.skyPx = size
		st.skyKey = key
		st.skyDark = st.cfg.DarkMode
	}
	return st.skyImg
}

// ensureMapBase returns the cached maprender base image and asks for a new
// render only when the center/zoom (or the base itself) changes. It never
// re-renders the map per frame; the satellite/observer overlay is drawn by the
// map view as cheap widgets on top of the cached base.
func ensureMapBase() *image.RGBA {
	if st.cfg.Location != nil && !st.mapInit {
		st.mapCenterLat = st.cfg.Location.Lat
		st.mapCenterLng = st.cfg.Location.Lng
		st.mapInit = true
	}
	key := mapKey(st.mapCenterLat, st.mapCenterLng, st.mapZoom)
	if st.mapBase == nil || st.mapBaseKey != key {
		st.requestMapRender()
	}
	return st.mapBase
}
