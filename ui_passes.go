package main

import (
	"fmt"
	"time"

	. "go.hasen.dev/shirei"
	. "go.hasen.dev/shirei/widgets"
)

func passesView() {
	passes := st.sortedPasses()
	Container(Attrs(Viewport, Pad(10), Gap(10)), func() {
		nextPassCard(passes)
		Container(Attrs(Grow(1), Expand, Clip, Corners(6), BorderWidth(1), BorderColor(0, 0, 80, 1)), func() {
			if len(passes) == 0 {
				emptyPasses()
				return
			}
			table := []TableColumn[*Pass]{
				{
					Label: "Satellite", Width: 0,
					Cell: func(p *Pass) {
						Label(fmt.Sprintf("%s (%d)", p.Name, p.NoradID))
					},
					Less: func(a, b *Pass) bool { return a.Name < b.Name },
				},
				{
					Label: "AOS (local)", Width: 170, DefaultDesc: false,
					Cell: func(p *Pass) { Label(p.Details.AOS.Local().Format("Mon 02 Jan 15:04:05")) },
					Less: func(a, b *Pass) bool { return a.Details.AOS.Before(b.Details.AOS) },
				},
				{
					Label: "Max el.", Width: 80, Alignment: AlignEnd,
					Cell: func(p *Pass) { Label(fmt.Sprintf("%.1f°", p.Details.MaxElevation)) },
					Less: func(a, b *Pass) bool { return a.Details.MaxElevation < b.Details.MaxElevation },
				},
				{
					Label: "Duration", Width: 90, Alignment: AlignEnd,
					Cell: func(p *Pass) { Label(p.Details.Duration.Truncate(time.Second).String()) },
					Less: func(a, b *Pass) bool { return a.Details.Duration < b.Details.Duration },
				},
				{
					Label: "In", Width: 110, Alignment: AlignEnd,
					Cell: func(p *Pass) { Label(countdown(p.Details.AOS)) },
					Less: func(a, b *Pass) bool { return a.Details.AOS.Before(b.Details.AOS) },
				},
			}
			attrs := TableAttrs[*Pass]{
				RowHeight:         32,
				DefaultSortColumn: 1,
				OnRow: func(i int, p *Pass) {
					// SatSat-style row tinting: in-progress green, finished grey.
					switch {
					case !p.Details.AOS.After(st.now) && p.Details.LOS.After(st.now):
						if st.cfg.DarkMode {
							ModAttrs(Background(120, 40, 22, 1))
						} else {
							ModAttrs(Background(120, 60, 88, 1))
						}
					case p.Details.LOS.Before(st.now):
						if st.cfg.DarkMode {
							ModAttrs(Background(0, 0, 22, 1))
						} else {
							ModAttrs(Background(0, 0, 93, 1))
						}
					}
					if IsHovered() {
						ModAttrs(Background(223, 40, 95, 1))
					}
					if PressAction() {
						st.openPassDetail(p)
					}
				},
			}
			TableExt[*Pass]("passes-table", attrs, table, passes, func(p *Pass) any { return p })
		})
	})
}

func emptyPasses() {
	Container(Attrs(Viewport, Center, Gap(8)), func() {
		if len(st.cfg.TrackedSats) == 0 {
			Label("No satellites tracked yet.", FontSize(15))
			Label("Open Preferences to choose which satellites to follow.", FontSize(13), muted())
			if Button(NoIcon, "Open Preferences") {
				st.openPrefs()
			}
			return
		}
		if st.fetching {
			Label("Computing passes…", FontSize(15))
			return
		}
		Label("No passes match your criteria in the next 72 hours.", FontSize(15))
		Label("Try lowering the minimum elevation in Preferences.", FontSize(13), muted())
	})
}

func nextPassCard(passes []*Pass) {
	var next *Pass
	for _, p := range passes {
		if p.Details.AOS.After(st.now) {
			next = p
			break
		}
	}
	Container(Attrs(Row, Expand, CrossMid, Gap(16), Pad2(14, 16), Corners(6), UseSurface(SurfacePanel)), func() {
		Icon(TypCalendar, FontSize(26))
		if next == nil {
			Container(Attrs(Gap(2)), func() {
				Label("Upcoming passes", FontWeight(WeightBold))
				Label("Nothing scheduled in the current window.", FontSize(13), muted())
			})
			return
		}
		Container(Attrs(Gap(2)), func() {
			Label("Next pass", FontSize(12), muted())
			Label(fmt.Sprintf("%s — max %.1f°", next.Name, next.Details.MaxElevation), FontSize(17), FontWeight(WeightBold))
			Label(next.Details.AOS.Local().Format("Monday 02 Jan 2006 at 15:04:05 MST"), FontSize(13))
		})
		Filler(1)
		Container(Attrs(Gap(2), CrossAlign(AlignEnd)), func() {
			Label(countdown(next.Details.AOS), FontSize(17), FontWeight(WeightBold))
			Label(fmt.Sprintf("duration %s", next.Details.Duration.Truncate(time.Second)), FontSize(12), muted())
		})
	})
}

func countdown(t time.Time) string {
	d := t.Sub(st.now)
	if d <= 0 {
		if d > -15*time.Minute {
			return "now"
		}
		return "passed"
	}
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %02dh", days, h)
	case h > 0:
		return fmt.Sprintf("%dh %02dm", h, m)
	default:
		return fmt.Sprintf("%dm %02ds", m, int(d.Seconds())%60)
	}
}
