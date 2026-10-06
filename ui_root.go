package main

import (
	"fmt"

	. "go.hasen.dev/shirei"
	app "go.hasen.dev/shirei/app"
	. "go.hasen.dev/shirei/widgets"
)

// RootView is the single frame function passed to app.Run.
func RootView() {
	SetDarkMode(st.cfg.DarkMode)
	ModAttrs(UseSurface(SurfaceCanvas))

	// Escape goes back: it closes the top of the view stack, matching the
	// Back buttons (detail view, preferences).
	if GetFrameInput().Key == KeyEscape {
		st.goBack()
	}

	switch st.view {
	case viewLoading:
		loadingView()
	case viewOnboarding:
		onboardingView()
	case viewPrefs:
		prefsView()
	case viewPassDetail:
		passDetailView()
	default:
		mainView()
	}
}

// goBack closes the top of the view stack, mirroring the Back button. It
// reports whether a view was closed.
func (s *State) goBack() bool {
	switch s.view {
	case viewPassDetail:
		s.selPass = nil
		s.view = viewMain
		return true
	case viewPrefs:
		s.prefs = nil
		s.view = viewMain
		return true
	}
	return false
}

func loadingView() {
	Container(Attrs(Viewport, Center, Gap(12)), func() {
		Image(app.ResourcePath("sat_picto.png"), Vec2{150, 96})
		Label("SatSat", FontSize(26), FontWeight(WeightBold))
		Label("Fetching orbital data…")
	})
}

func mainView() {
	Container(Attrs(Viewport), func() {
		headerBar()
		tabBar()
		Container(Attrs(Grow(1), Expand, Clip), func() {
			switch st.tab {
			case tabMap:
				mapView()
			case tabSky:
				skyView()
			default:
				passesView()
			}
		})
		statusBar()
	})
}

func headerBar() {
	Container(Attrs(Row, Expand, FixHeight(52), UseSurface(SurfaceToolbar), Pad2(0, 14), Gap(12), CrossMid, Clip), func() {
		Image(app.ResourcePath("sat_picto.png"), Vec2{42, 28})
		Label("SatSat", FontSize(17), FontWeight(WeightBold))
		Label("•", TextColor(0, 0, 100, 0.5))
		Icon(TypLocation, FontSize(15))
		Label(st.locationName())
		Filler(1)
		if st.fetching {
			if st.fromCache {
				Label("cached · refreshing…", FontSize(12), TextColor(0, 0, 100, 0.7))
			} else {
				Label("refreshing…", FontSize(12), TextColor(0, 0, 100, 0.7))
			}
		} else if !st.lastFetch.IsZero() {
			Label("updated "+st.lastFetch.Local().Format("15:04"), FontSize(12), TextColor(0, 0, 100, 0.7))
		}
		if Button(SymRefresh, "Refresh") {
			requestRefresh()
		}
		if Button(TypCog, "Settings") {
			st.openPrefs()
		}
	})
}

func tabBar() {
	TabStrip(func() {
		if TabItem(tabPasses, "Calendar", st.tab == tabPasses, nil) {
			st.tab = tabPasses
		}
		if TabItem(tabMap, "Map", st.tab == tabMap, nil) {
			st.tab = tabMap
		}
		if TabItem(tabSky, "Live", st.tab == tabSky, nil) {
			st.tab = tabSky
		}
	}, func() {
		Label(fmt.Sprintf("%d tracked", len(st.cfg.TrackedSats)), FontSize(12))
	})
}

func statusBar() {
	Container(Attrs(Row, Expand, FixHeight(28), UseSurface(SurfacePanel), Pad2(0, 14), Gap(14), CrossMid, Clip), func() {
		if st.fetching {
			if st.fromCache {
				Label("Using cached elements while refreshing…", FontSize(12))
			} else {
				Label("Fetching TLEs and transponders…", FontSize(12))
			}
		} else if len(st.fetchErrs) > 0 {
			Label(fmt.Sprintf("%d source error(s): %s", len(st.fetchErrs), st.fetchErrs[0]), FontSize(12), warnText())
		} else if st.cfgWarn != "" {
			Label(st.cfgWarn, FontSize(12), warnText())
		} else if st.fetchNotice != "" {
			Label(st.fetchNotice, FontSize(12), muted())
		} else if !st.lastFetch.IsZero() {
			Label(fmt.Sprintf("Data OK · %d satellites", len(st.sats)), FontSize(12))
		} else {
			Label("Waiting for data…", FontSize(12))
		}
		Filler(1)
		Label(fmt.Sprintf("Min elevation %.0f°", st.cfg.MinElevation), FontSize(12))
		Label(st.now.Local().Format("Mon 02 Jan 15:04:05"), FontSize(12))
	})
}

func sectionHeader(title string) {
	Container(Attrs(Row, Expand, CrossMid, Gap(8), Pad2(8, 0)), func() {
		Label(title, FontSize(14), FontWeight(WeightBold))
		Element(Attrs(Grow(1), FixHeight(1), Background(0, 0, 70, 1)))
	})
}
