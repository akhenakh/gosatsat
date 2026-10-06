package main

import (
	"fmt"
	"time"

	. "go.hasen.dev/shirei"
	. "go.hasen.dev/shirei/widgets"
)

func (s *State) openPassDetail(p *Pass) {
	s.selPass = p
	s.view = viewPassDetail
}

// openSatDetail opens the detail of the pass currently in progress for sat, or
// failing that the next upcoming pass. The detail view is pass-centric, so a
// satellite with no known pass is a no-op.
func (s *State) openSatDetail(sat *Sat) {
	if sat == nil {
		return
	}
	var next *Pass
	for _, p := range s.passes {
		if p.NoradID != sat.NoradID {
			continue
		}
		if !p.Details.AOS.After(s.now) && p.Details.LOS.After(s.now) {
			s.openPassDetail(p)
			return
		}
		if p.Details.AOS.After(s.now) && (next == nil || p.Details.AOS.Before(next.Details.AOS)) {
			next = p
		}
	}
	if next != nil {
		s.openPassDetail(next)
	}
}

func passDetailView() {
	p := st.selPass
	if p == nil {
		st.view = viewMain
		return
	}
	Container(Attrs(Viewport), func() {
		Container(Attrs(Row, Expand, FixHeight(52), UseSurface(SurfaceToolbar), Pad2(0, 14), Gap(10), CrossMid), func() {
			if Button(TypArrowLeft, "Back") {
				st.goBack()
			}
			Icon(TypCalendar, FontSize(18))
			Label(fmt.Sprintf("%s (%d)", p.Name, p.NoradID), FontSize(17), FontWeight(WeightBold))
			Filler(1)
			Label(p.Details.AOS.Local().Format("Mon 02 Jan 2006 15:04:05 MST"), FontSize(12), muted())
		})
		Container(Attrs(Viewport, Pad(16)), func() {
			ScrollOnInput()
			ScrollBars()
			avail := GetContentWidth()
			plotSize := avail
			if plotSize > 520 {
				plotSize = 520
			}
			wide := avail >= 960 && avail-plotSize-24 >= 420
			if wide {
				Container(Attrs(Row, Gap(24), CrossAlign(AlignStart)), func() {
					Container(Attrs(FixWidth(plotSize), Gap(10)), func() {
						passPlot(p, plotSize)
					})
					Container(Attrs(Grow(1), Gap(18)), func() {
						factsGrid(p)
						transponderSection(p)
						dataPointSection(p)
					})
				})
				return
			}
			Container(Attrs(Gap(18)), func() {
				Container(Attrs(Expand, Center), func() {
					passPlot(p, plotSize)
				})
				factsGrid(p)
				transponderSection(p)
				dataPointSection(p)
			})
		})
	})
}

// passPlot renders and displays the polar pass plot at the given logical size.
func passPlot(p *Pass, size float32) {
	if size < 120 {
		return
	}
	plot := ensurePassPlot(p, int(size))
	id := UseImage(fmt.Sprintf("pass-%d-%d", p.NoradID, p.Details.AOS.Unix()), plot)
	ImageViewAt(id, Vec2{size, size})
}

func fact(label, value string) {
	Container(Attrs(Gap(2), Pad(10), Corners(6), UseSurface(SurfacePanel)), func() {
		Label(label, FontSize(11), muted())
		Label(value, FontSize(15), FontWeight(WeightSemibold))
	})
}

func factsGrid(p *Pass) {
	sectionHeader("Pass details")
	Container(Attrs(Row, Wrap, Gap(10)), func() {
		fact("AOS (local)", p.Details.AOS.Local().Format("15:04:05"))
		fact("Max elevation", fmt.Sprintf("%.1f°", p.Details.MaxElevation))
		fact("LOS (local)", p.Details.LOS.Local().Format("15:04:05"))
		fact("Duration", p.Details.Duration.Truncate(time.Second).String())
		fact("AOS azimuth", fmt.Sprintf("%.1f°", p.Details.AOSAzimuth))
		fact("Max azimuth", fmt.Sprintf("%.1f°", p.Details.MaxElevationAz))
		fact("LOS azimuth", fmt.Sprintf("%.1f°", p.Details.LOSAzimuth))
		fact("Range at AOS", fmt.Sprintf("%.0f km", p.Details.AOSObservation.LookAngles.Range))
	})
}

func transponderSection(p *Pass) {
	sat := st.sats[p.NoradID]
	sectionHeader("Transponder frequencies (with Doppler)")
	if sat == nil || len(sat.Transponders) == 0 {
		Label("No transponder information available for this satellite.", FontSize(13), muted())
		return
	}
	aosRate := p.Details.AOSObservation.LookAngles.RangeRate
	losRate := p.Details.LOSObservation.LookAngles.RangeRate
	Container(Attrs(Gap(8)), func() {
		for i := range sat.Transponders {
			tp := sat.Transponders[i]
			ContainerWithKey(fmt.Sprintf("tp-%d-%d", p.NoradID, i), Attrs(Gap(6), Pad(12), Corners(6), UseSurface(SurfacePanel)), func() {
				Container(Attrs(Row, CrossMid, Gap(10)), func() {
					Label(tp.Mode, FontWeight(WeightBold))
					if tp.Callsign != "" {
						Label(tp.Callsign, FontSize(12), muted())
					}
					Filler(1)
					Label(tp.Status, FontSize(11))
				})
				Container(Attrs(Row, Wrap, Gap(14)), func() {
					freqFact("Uplink (nominal)", tp.Uplink)
					freqFact("Uplink TX @AOS", formatFrequency(uplinkCorrection(parseFrequencyMHz(tp.Uplink), aosRate)))
					freqFact("Uplink TX @LOS", formatFrequency(uplinkCorrection(parseFrequencyMHz(tp.Uplink), losRate)))
					freqFact("Downlink (nominal)", tp.Downlink)
					freqFact("Downlink RX @AOS", formatFrequency(calculateDopplerShift(parseFrequencyMHz(tp.Downlink), aosRate)))
					freqFact("Downlink RX @LOS", formatFrequency(calculateDopplerShift(parseFrequencyMHz(tp.Downlink), losRate)))
					if tp.Beacon != "" {
						freqFact("Beacon (nominal)", tp.Beacon)
						freqFact("Beacon RX @AOS", formatFrequency(calculateDopplerShift(parseFrequencyMHz(tp.Beacon), aosRate)))
						freqFact("Beacon RX @LOS", formatFrequency(calculateDopplerShift(parseFrequencyMHz(tp.Beacon), losRate)))
					}
				})
			})
		}
	})
}

func freqFact(label, value string) {
	Container(Attrs(Gap(1), MinWidth(150)), func() {
		Label(label, FontSize(11), muted())
		Label(value, FontSize(13), FontWeight(WeightSemibold))
	})
}

func dataPointSection(p *Pass) {
	sectionHeader(fmt.Sprintf("Pass data points (%d)", len(p.DataPoints)))
	if len(p.DataPoints) == 0 {
		Label("No sampled data points.", FontSize(13), muted())
		return
	}
	Container(Attrs(FixHeight(240), Expand, Clip, Corners(6), BorderWidth(1), BorderColor(0, 0, 80, 1)), func() {
		pts := p.DataPoints
		VirtualListView("pass-points", len(pts), func(i int) any { return &pts[i] }, func(i int, w float32) float32 { return 22 }, func(i int, w float32) {
			pt := pts[i]
			Container(Attrs(Row, Expand, FixHeight(22), CrossMid), func() {
				cell(fmt.Sprintf("%-12s", pt.Timestamp.Local().Format("15:04:05.000")), 150)
				cell(fmt.Sprintf("az %6.2f°", pt.Azimuth), 110)
				cell(fmt.Sprintf("el %6.2f°", pt.Elevation), 110)
				cell(fmt.Sprintf("%8.1f km", pt.Range), 120)
				cell(fmt.Sprintf("%7.3f km/s", pt.RangeRate), 0)
			})
		})
	})
}

func cell(text string, width float32) {
	var attrs AttrSet
	if width > 0 {
		attrs = Attrs(FixWidth(width), Pad2(0, 8), Clip)
	} else {
		attrs = Attrs(Grow(1), Pad2(0, 8), Clip)
	}
	Container(attrs, func() {
		Label(text, FontSize(12), Fonts(Monospace...))
	})
}
