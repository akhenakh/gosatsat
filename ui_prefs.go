package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	. "go.hasen.dev/shirei"
	. "go.hasen.dev/shirei/widgets"
)

// openPrefs builds an editable draft from the current configuration.
func (s *State) openPrefs() {
	d := &prefsDraft{
		minElevation: s.cfg.MinElevation,
		refreshHours: s.cfg.RefreshHours,
		darkMode:     s.cfg.DarkMode,
		tleSources:   append([]string(nil), s.cfg.TleSources...),
		xpSources:    append([]string(nil), s.cfg.TransponderSources...),
		tracked:      map[int]bool{},
	}
	for _, id := range s.cfg.TrackedSats {
		d.tracked[id] = true
	}
	if s.cfg.Location != nil {
		d.locationName = s.cfg.Location.Name
		d.lat = strconv.FormatFloat(s.cfg.Location.Lat, 'f', 5, 64)
		d.lng = strconv.FormatFloat(s.cfg.Location.Lng, 'f', 5, 64)
		d.alt = strconv.FormatFloat(s.cfg.Location.Alt, 'f', 0, 64)
	} else {
		d.lat, d.lng, d.alt = "0", "0", "0"
	}
	s.prefs = d
	s.view = viewPrefs
}

func (s *State) savePrefs() {
	d := s.prefs
	if d == nil {
		return
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(d.lat), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(d.lng), 64)
	alt, err3 := strconv.ParseFloat(strings.TrimSpace(d.alt), 64)
	if err1 != nil || err2 != nil {
		d.err = "Latitude and longitude must be numbers."
		return
	}
	if err3 != nil {
		alt = 0
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		d.err = "Latitude must be -90..90 and longitude -180..180."
		return
	}

	tracked := make([]int, 0, len(d.tracked))
	for id, on := range d.tracked {
		if on {
			tracked = append(tracked, id)
		}
	}
	sort.Ints(tracked)

	s.cfg.Location = &LocationConfig{Name: d.locationName, Lat: lat, Lng: lng, Alt: alt}
	s.cfg.MinElevation = d.minElevation
	s.cfg.RefreshHours = d.refreshHours
	s.cfg.DarkMode = d.darkMode
	s.cfg.TleSources = nonEmpty(d.tleSources)
	s.cfg.TransponderSources = nonEmpty(d.xpSources)
	s.cfg.TrackedSats = tracked

	if err := saveConfig(s.cfg); err != nil {
		d.err = err.Error()
		return
	}
	s.prefs = nil
	s.view = viewMain
	s.passesDirty = true
	recomputePasses()
	startFetch()
}

func nonEmpty(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func prefsView() {
	if st.prefs == nil {
		st.openPrefs()
	}
	d := st.prefs
	Container(Attrs(Viewport), func() {
		Container(Attrs(Row, Expand, FixHeight(52), UseSurface(SurfaceToolbar), Pad2(0, 14), Gap(10), CrossMid), func() {
			if Button(TypArrowLeft, "Back") {
				st.prefs = nil
				st.view = viewMain
			}
			Label("Preferences", FontSize(17), FontWeight(WeightBold))
			Filler(1)
			if d.err != "" {
				Label(d.err, FontSize(12), warnText())
			}
			NextButtonType(ButtonPrimary)
			if Button(SymPass, "Save") {
				st.savePrefs()
			}
		})
		Container(Attrs(Viewport, Pad(16)), func() {
			ScrollOnInput()
			ScrollBars()
			avail := GetContentWidth()
			if avail >= 1000 {
				Container(Attrs(Row, Gap(28), CrossAlign(AlignStart)), func() {
					Container(Attrs(Grow(1), Gap(22)), func() {
						locationSection(d)
						elevationSection(d)
						appearanceSection(d)
					})
					Container(Attrs(Grow(1), Gap(22)), func() {
						trackingSection(d)
						sourcesSection(d)
					})
				})
				return
			}
			Container(Attrs(Gap(22), MaxWidth(1000)), func() {
				locationSection(d)
				trackingSection(d)
				elevationSection(d)
				sourcesSection(d)
				appearanceSection(d)
			})
		})
	})
}

func locationSection(d *prefsDraft) {
	sectionHeader("Location")
	Container(Attrs(Gap(8)), func() {
		Container(Attrs(Row, Gap(8), CrossMid), func() {
			Container(Attrs(Grow(1)), func() {
				a := DefaultTextInputAttrs()
				a.Placeholder = "Search a city…"
				a.NoAutoFocus = true
				TextInputExt(&d.cityQuery, a)
			})
			if d.locationName != "" {
				Label("current: "+d.locationName, FontSize(12), muted())
			}
		})
		if q := strings.TrimSpace(d.cityQuery); q != "" {
			matches := matchCities(q)
			Container(Attrs(Gap(2)), func() {
				for _, c := range matches {
					if clickableRow(c.Name) {
						d.locationName = c.Name
						d.lat = strconv.FormatFloat(c.Lat, 'f', 5, 64)
						d.lng = strconv.FormatFloat(c.Lng, 'f', 5, 64)
						d.alt = strconv.FormatFloat(c.Alt, 'f', 0, 64)
						d.cityQuery = ""
					}
				}
			})
		}
		Container(Attrs(Row, Gap(10), CrossMid), func() {
			numberField("Latitude", &d.lat)
			numberField("Longitude", &d.lng)
			numberField("Altitude (m)", &d.alt)
		})
		Container(Attrs(Gap(3)), func() {
			Label("Location name (optional)", FontSize(12), muted())
			a := DefaultTextInputAttrs()
			a.Placeholder = "e.g. Home"
			a.NoAutoFocus = true
			TextInputExt(&d.locationName, a)
		})
	})
}

func numberField(label string, buf *string) {
	Container(Attrs(Gap(3)), func() {
		Label(label, FontSize(12), muted())
		a := DefaultTextInputAttrs()
		a.MinWidth = 130
		a.FixedWidth = true
		a.NoAutoFocus = true
		TextInputExt(buf, a)
	})
}

func clickableRow(label string) bool {
	clicked := false
	Container(Attrs(Row, Expand, FixHeight(26), CrossMid, Pad2(0, 8), Corners(4)), func() {
		if IsHovered() {
			ModAttrs(Background(210, 40, 90, 1))
		}
		if PressAction() {
			clicked = true
		}
		Label(label, FontSize(13))
	})
	return clicked
}

func matchCities(q string) []City {
	q = strings.ToUpper(q)
	var out []City
	for _, c := range cities {
		if strings.Contains(strings.ToUpper(c.Name), q) {
			out = append(out, c)
			if len(out) >= 12 {
				break
			}
		}
	}
	return out
}

func trackingSection(d *prefsDraft) {
	sectionHeader(fmt.Sprintf("Tracked satellites (%d)", countTrue(d.tracked)))
	Container(Attrs(Gap(8)), func() {
		Container(Attrs(Row, Gap(10), CrossMid), func() {
			Container(Attrs(Grow(1)), func() {
				a := DefaultTextInputAttrs()
				a.Placeholder = "Filter by name or NORAD ID…"
				a.NoAutoFocus = true
				TextInputExt(&d.satQuery, a)
			})
			CheckBox(&d.activeOnly, "Active transponders only")
			Label(fmt.Sprintf("%d available", len(st.satOrder)), FontSize(12), muted())
		})
		Container(Attrs(FixHeight(280), Expand, Clip, Corners(6), BorderWidth(1), BorderColor(0, 0, 80, 1)), func() {
			sats := filteredSats(d.satQuery, d.activeOnly)
			if len(sats) == 0 {
				Container(Attrs(Viewport, Center), func() { Label("No satellites match the filter.") })
				return
			}
			satelliteList("prefs-sats", sats, d.tracked)
		})
	})
}

func filteredSats(query string, activeOnly bool) []*Sat {
	q := strings.ToUpper(strings.TrimSpace(query))
	var out []*Sat
	for _, sat := range st.satOrder {
		if q != "" && !strings.Contains(strings.ToUpper(sat.Name), q) && !strings.Contains(strconv.Itoa(sat.NoradID), q) {
			continue
		}
		if activeOnly && len(sat.Transponders) == 0 {
			continue
		}
		out = append(out, sat)
	}
	return out
}

func satelliteList(listKey any, sats []*Sat, tracked map[int]bool) {
	VirtualListView(listKey, len(sats),
		func(i int) any { return sats[i] },
		func(i int, w float32) float32 { return 30 },
		func(i int, w float32) {
			sat := sats[i]
			Container(Attrs(Row, Expand, FixHeight(30), CrossMid, Gap(8), Pad2(0, 8)), func() {
				if IsHovered() {
					ModAttrs(Background(210, 40, 92, 1))
				}
				sel := tracked[sat.NoradID]
				CheckBox(&sel, "")
				if sel != tracked[sat.NoradID] {
					tracked[sat.NoradID] = sel
				}
				Label(fmt.Sprintf("%s  (%d)", sat.Name, sat.NoradID), FontSize(13))
				Filler(1)
				if n := len(sat.Transponders); n > 0 {
					Label(fmt.Sprintf("%d tp", n), FontSize(11), muted())
				}
			})
		},
	)
}

func countTrue(m map[int]bool) int {
	n := 0
	for _, v := range m {
		if v {
			n++
		}
	}
	return n
}

func elevationSection(d *prefsDraft) {
	sectionHeader("Minimum peak elevation")
	Container(Attrs(Row, Gap(12), CrossMid), func() {
		v := float32(d.minElevation)
		Slider(&v, SliderAttrs{Min: 0, Max: 90, Step: 1, Width: 260})
		d.minElevation = float64(v)
		Label(fmt.Sprintf("%.0f°", d.minElevation), FontWeight(WeightBold), FontSize(14))
		Label("Passes below this peak elevation are hidden.", FontSize(12), muted())
	})
}

func sourcesSection(d *prefsDraft) {
	sectionHeader("Orbital data sources (TLE / GP CSV / OMM JSON)")
	urlListEditor(&d.tleSources, "tle-source")
	sectionHeader("Transponder sources (CSV)")
	urlListEditor(&d.xpSources, "xp-source")
	Container(Attrs(Row, Gap(10), CrossMid), func() {
		Label(fmt.Sprintf("%d satellites loaded · last update %s", len(st.sats), lastFetchLabel()), FontSize(12), muted())
		Filler(1)
		if Button(SymRefresh, "Refresh now") {
			requestRefresh()
		}
	})
}

func lastFetchLabel() string {
	if st.lastFetch.IsZero() {
		return "never"
	}
	return st.lastFetch.Local().Format("15:04:05")
}

func urlListEditor(list *[]string, keyPrefix string) {
	del := -1
	Container(Attrs(Gap(6)), func() {
		for i := range *list {
			ContainerWithKey(fmt.Sprintf("%s-%d", keyPrefix, i), Attrs(Row, Gap(6), CrossMid), func() {
				Container(Attrs(Grow(1)), func() {
					a := DefaultTextInputAttrs()
					a.NoAutoFocus = true
					TextInputExt(&(*list)[i], a)
				})
				if Button(SymDelete, "") {
					del = i
				}
			})
		}
	})
	if del >= 0 && del < len(*list) {
		*list = append((*list)[:del], (*list)[del+1:]...)
	}
	if Button(SymPlus, "Add source") {
		*list = append(*list, "")
	}
}

func appearanceSection(d *prefsDraft) {
	sectionHeader("Appearance")
	CheckBox(&d.darkMode, "Dark mode")
	Container(Attrs(Row, Gap(10), CrossMid), func() {
		Label("Refresh data every", FontSize(13))
		SegmentedControl(&d.refreshHours, func() {
			for _, h := range []int{2, 3, 6, 12, 24} {
				SegmentedCell(fmt.Sprintf("%dh", h), h)
			}
		})
	})
}
