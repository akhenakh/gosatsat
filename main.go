package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	shirei "go.hasen.dev/shirei"
	"go.hasen.dev/shirei/app"
)

func renderPNG(path string, w, h int) error {
	return shirei.RenderToPNG(path, w, h, RootView)
}

func main() {
	png := flag.String("png", "", "render one settled frame to PATH and exit")
	width := flag.Int("w", 1180, "window/PNG width")
	height := flag.Int("h", 780, "window/PNG height")
	demo := flag.Bool("demo", false, "use built-in demo data instead of fetching (offline)")
	dark := flag.Bool("dark", false, "start in dark mode")
	tab := flag.String("tab", "passes", "demo tab to show: passes|map|sky")
	detail := flag.Bool("detail", false, "open the first upcoming pass detail (with --demo)")
	view := flag.String("view", "", "force a view for snapshots: prefs|onboarding")
	step := flag.Int("step", 0, "onboarding step for snapshots")
	flag.Parse()

	cfg, needsOnboarding, warning := loadConfig()
	if *dark {
		cfg.DarkMode = true
	}
	st.cfg = cfg
	setupTheme()
	if warning != nil {
		st.cfgWarn = warning.Error()
	}
	app.SetupIcon(app.ResourcePath("icon.png"))

	if *demo {
		loadDemoData()
		switch *tab {
		case "map":
			st.tab = tabMap
		case "sky":
			st.tab = tabSky
		default:
			st.tab = tabPasses
		}
		if *detail {
			for _, p := range st.sortedPasses() {
				if p.Details.AOS.After(st.now) {
					st.selPass = p
					st.view = viewPassDetail
					break
				}
			}
		}
		if st.tab == tabMap {
			prepareDemoMap()
		}
		if *view == "prefs" {
			st.openPrefs()
		} else if *view == "onboarding" {
			st.view = viewOnboarding
			st.onboardStep = *step
			st.initOnboarding()
		}
		if *dark {
			st.cfg.DarkMode = true
		}
	} else {
		if needsOnboarding {
			st.view = viewOnboarding
		} else {
			st.view = viewMain
		}
		// Start from the last download so the app is usable immediately. Only
		// hit the network when the cache is missing or older than the CelesTrak
		// minimum interval; otherwise the periodic refresh handles it later.
		if tles, names, tps, savedAt, err := loadTLECache(); err == nil && len(tles) > 0 {
			st.mergeSatellites(tles, names, tps)
			st.lastFetch = savedAt
			st.lastAttempt = savedAt
			st.fromCache = true
			requestPassRecompute()
		}
		if st.lastAttempt.IsZero() || time.Since(st.lastAttempt) >= minRefreshInterval {
			startFetch()
		}
		startNowTicker()
		startRefreshLoop()
		startPassRefresh()
	}

	if *png != "" {
		if err := renderPNG(*png, *width, *height); err != nil {
			fmt.Fprintln(os.Stderr, "render failed:", err)
			os.Exit(1)
		}
		return
	}

	app.SetupWindow("SatSat", *width, *height)
	app.Run(RootView)
}
