package main

import (
	"fmt"
	"strconv"
	"strings"

	. "go.hasen.dev/shirei"
	app "go.hasen.dev/shirei/app"
	. "go.hasen.dev/shirei/widgets"
)

func (s *State) initOnboarding() {
	if s.obLat == "" {
		s.obLat = "40.7128"
		s.obLng = "-74.0060"
		s.obAlt = "0"
	}
	if s.obTracked == nil {
		s.obTracked = map[int]bool{}
	}
}

func onboardingView() {
	st.initOnboarding()
	Container(Attrs(Viewport), func() {
		Container(Attrs(Row, Expand, FixHeight(64), UseSurface(SurfaceToolbar), Pad2(0, 20), Gap(12), CrossMid), func() {
			Image(app.ResourcePath("sat_picto.png"), Vec2{52, 34})
			Container(Attrs(Gap(2)), func() {
				Label("SatSat", FontSize(20), FontWeight(WeightBold))
				Label("Native satellite pass tracker", FontSize(12), TextColor(0, 0, 100, 0.8))
			})
			Filler(1)
			stepPill("1  Location", st.onboardStep == 0)
			stepPill("2  Satellites", st.onboardStep == 1)
		})
		if st.onboardStep == 0 {
			onboardLocation()
		} else {
			onboardSatellites()
		}
	})
}

func stepPill(text string, active bool) {
	bg := Vec4{0, 0, 70, 1}
	if active {
		bg = Vec4{210, 70, 45, 1}
	}
	Container(Attrs(Pad2(4, 12), Corners(12), BackgroundVec(bg)), func() {
		Label(text, FontSize(12), TextColor(0, 0, 100, 1))
	})
}

func onboardLocation() {
	Container(Attrs(Grow(1), Expand, Clip, Pad(20)), func() {
		avail := GetContentWidth()
		if avail < 760 {
			Container(Attrs(Gap(16), Grow(1), Expand, Clip), func() {
				Container(Attrs(Grow(1), Expand, Clip), onboardCityPicker)
				Container(Attrs(Expand), func() { onboardCoordsPanel(false) })
			})
			return
		}
		Container(Attrs(Row, Grow(1), Expand, Clip, Gap(24)), func() {
			Container(Attrs(Grow(1), Expand, Clip), onboardCityPicker)
			Container(Attrs(FixWidth(340), Expand), func() { onboardCoordsPanel(true) })
		})
	})
}

func onboardCityPicker() {
	Container(Attrs(Grow(1), Expand, Clip, Gap(8)), func() {
		Label("Pick a city", FontSize(16), FontWeight(WeightBold))
		Container(Attrs(Expand), func() {
			a := DefaultTextInputAttrs()
			a.Placeholder = "Search cities…"
			a.NoAutoFocus = true
			TextInputExt(&st.obCityQuery, a)
		})
		Container(Attrs(Grow(1), Expand, Clip, Corners(6), BorderWidth(1), BorderColor(0, 0, 80, 1)), func() {
			list := matchCities(st.obCityQuery)
			if len(list) == 0 {
				Container(Attrs(Viewport, Center), func() { Label("No matching city.") })
				return
			}
			VirtualListView("ob-cities", len(list), func(i int) any { return i }, func(i int, w float32) float32 { return 30 }, func(i int, w float32) {
				c := list[i]
				Container(Attrs(Row, Expand, FixHeight(30), CrossMid, Gap(8), Pad2(0, 10)), func() {
					if IsHovered() {
						ModAttrs(Background(223, 40, 92, 1))
					}
					if PressAction() {
						st.obLat = strconv.FormatFloat(c.Lat, 'f', 5, 64)
						st.obLng = strconv.FormatFloat(c.Lng, 'f', 5, 64)
						st.obAlt = strconv.FormatFloat(c.Alt, 'f', 0, 64)
						st.obCityName = cityLabel(c)
						st.obUseCity = true
					}
					Label(cityLabel(c), FontSize(13))
				})
			})
		})
	})
}

func onboardCoordsPanel(fixed bool) {
	var attrs AttrSet
	if fixed {
		attrs = Attrs(FixWidth(340), Gap(10), Pad(16), Corners(8), UseSurface(SurfacePanel))
	} else {
		attrs = Attrs(Expand, Gap(10), Pad(16), Corners(8), UseSurface(SurfacePanel))
	}
	Container(attrs, func() {
		Label("Or enter coordinates", FontSize(16), FontWeight(WeightBold))
		numberField("Latitude (-90..90)", &st.obLat)
		numberField("Longitude (-180..180)", &st.obLng)
		numberField("Altitude (m)", &st.obAlt)
		latSt, lngSt, ok := st.obCoords()
		if ok {
			Label(formatLatLng(latSt, lngSt), FontSize(12), muted())
		} else {
			Label("Enter valid coordinates to continue.", FontSize(12), warnText())
		}
		NextButtonType(ButtonPrimary)
		NextButtonDisabled(!ok)
		if Button(TypArrowRight, "Continue") {
			st.onboardStep = 1
			st.obSatQuery = ""
		}
	})
}

func onboardSatellites() {
	Container(Attrs(Grow(1), Expand, Clip, Pad(20), Gap(14)), func() {
		Container(Attrs(Row, CrossMid, Gap(12)), func() {
			Container(Attrs(Gap(2)), func() {
				Label("Choose satellites to track", FontSize(18), FontWeight(WeightBold))
				Label("You can change this later in Preferences.", FontSize(12), muted())
			})
			Filler(1)
			if st.fetching {
				Label("Loading satellite catalogue…", FontSize(12))
			} else {
				Label(fmt.Sprintf("%d satellites loaded", len(st.satOrder)), FontSize(12))
			}
		})
		Container(Attrs(Row, Gap(12), CrossMid), func() {
			Container(Attrs(Grow(1)), func() {
				a := DefaultTextInputAttrs()
				a.Placeholder = "Filter by name or NORAD ID…"
				a.NoAutoFocus = true
				TextInputExt(&st.obSatQuery, a)
			})
			CheckBox(&st.obActiveOnly, "Active transponders only")
			Filler(1)
			if Button(NoIcon, "Select all shown") {
				for _, sat := range filteredSats(st.obSatQuery, st.obActiveOnly) {
					if len(st.obTracked) >= maxTrackedSats {
						break
					}
					st.obTracked[sat.NoradID] = true
				}
			}
			if Button(NoIcon, "Clear") {
				st.obTracked = map[int]bool{}
			}
		})
		Container(Attrs(Grow(1), Expand, Clip, Corners(6), BorderWidth(1), BorderColor(0, 0, 80, 1)), func() {
			sats := filteredSats(st.obSatQuery, st.obActiveOnly)
			if len(sats) == 0 {
				Container(Attrs(Viewport, Center), func() {
					if st.fetching {
						Label("Fetching satellite catalogue…")
					} else {
						Label("No satellites available. Check your network or TLE sources.")
					}
				})
				return
			}
			satelliteList("ob-sats", sats, st.obTracked)
		})
		Container(Attrs(Row, CrossMid, Gap(10)), func() {
			if Button(TypArrowLeft, "Back") {
				st.onboardStep = 0
			}
			Label(fmt.Sprintf("%d selected (max %d)", countTrue(st.obTracked), maxTrackedSats), FontSize(12), muted())
			Filler(1)
			NextButtonType(ButtonPrimary)
			if Button(TypRadar, "Start tracking") {
				st.finishOnboarding()
			}
		})
	})
}

func (s *State) obCoords() (float64, float64, bool) {
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(s.obLat), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(s.obLng), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}

func (s *State) finishOnboarding() {
	lat, lng, ok := s.obCoords()
	if !ok {
		s.onboardStep = 0
		return
	}
	alt, _ := strconv.ParseFloat(strings.TrimSpace(s.obAlt), 64)

	name := ""
	if s.obUseCity {
		name = s.obCityName
	}
	s.cfg.Location = &LocationConfig{Name: name, Lat: lat, Lng: lng, Alt: alt}

	var tracked []int
	for _, sat := range s.satOrder {
		if s.obTracked[sat.NoradID] {
			tracked = append(tracked, sat.NoradID)
		}
	}
	s.cfg.TrackedSats = tracked

	if err := saveConfig(s.cfg); err != nil {
		s.cfgWarn = err.Error()
	}
	s.view = viewMain
	recomputePasses()
}
