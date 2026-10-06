package main

import (
	"fmt"
	"time"

	"github.com/akhenakh/sgp4"
	. "go.hasen.dev/shirei"
	. "go.hasen.dev/shirei/widgets"
)

type skyEntry struct {
	sat    *Sat
	az, el float64
	rng    float64
}

func skyView() {
	entries := visibleSatellites(st.location(), st.trackedSats(), st.now)
	Container(Attrs(Viewport), func() {
		Container(Attrs(Row, Expand, FixHeight(40), UseSurface(SurfacePanel), Pad2(0, 12), Gap(8), CrossMid), func() {
			Icon(TypRadar, FontSize(18))
			Label("Live sky", FontWeight(WeightBold))
			Label(fmt.Sprintf("%d above horizon at %s", len(entries), st.now.Local().Format("15:04:05")), FontSize(12), muted())
			Filler(1)
			Label(st.locationName(), FontSize(12))
		})
		Container(Attrs(Row, Grow(1), Expand, Clip, Pad(10), Gap(14)), func() {
			availW := GetContentWidth()
			availH := GetContentHeight()
			plotSize := availH
			if max := availW * 0.62; max < plotSize {
				plotSize = max
			}
			if plotSize < 160 {
				plotSize = 160
			}
			Container(Attrs(FixWidth(plotSize), Expand, Clip, Center), func() {
				img := ensureSkyPlot(int(plotSize))
				id := UseImage("skyplot", img)
				ImageViewAt(id, Vec2{plotSize, plotSize})
			})
			Container(Attrs(Grow(1), Expand, Clip, Corners(6), BorderWidth(1), BorderColor(0, 0, 80, 1)), func() {
				Container(Attrs(Viewport, Pad(6)), func() {
					ScrollOnInput()
					ScrollBars()
					if len(entries) == 0 {
						Label("No tracked satellite is above the horizon right now.", FontSize(13), muted())
						return
					}
					for _, e := range entries {
						ContainerWithKey(e.sat, Attrs(Row, Expand, FixHeight(44), CrossMid, Gap(8), Pad2(4, 6), Corners(4)), func() {
							drawElevationBadge(e.el)
							Container(Attrs(Grow(1), Gap(1)), func() {
								Label(e.sat.Name, FontSize(13), FontWeight(WeightSemibold))
								Label(fmt.Sprintf("az %.0f° · el %.1f° · %.0f km", e.az, e.el, e.rng), FontSize(11), muted())
							})
						})
					}
				})
			})
		})
	})
}

func drawElevationBadge(el float64) {
	Container(Attrs(FixSize(34, 34), Corners(17), Center, Background(230, 55, 46, 1)), func() {
		Label(fmt.Sprintf("%.0f°", el), FontSize(11), FontWeight(WeightBold), TextColor(0, 0, 100, 1))
	})
}

// visibleSatellites returns the tracked satellites above the horizon at now.
//
// sgp4 derives Greenwich sidereal time (and therefore the observer's
// Earth-fixed position) from the wall-clock fields of the time it is given, so
// it must receive UTC. Passing a local time rotates the observer by the local
// UTC offset and reports the wrong satellites; normalize here at the boundary.
func visibleSatellites(loc sgp4.Location, sats []*Sat, now time.Time) []skyEntry {
	now = now.UTC()
	var out []skyEntry
	for _, sat := range sats {
		if sat.TLE == nil {
			continue
		}
		eci, err := sat.TLE.FindPositionAtTime(now)
		if err != nil {
			continue
		}
		sv := &sgp4.StateVector{
			X: eci.Position.X, Y: eci.Position.Y, Z: eci.Position.Z,
			VX: eci.Velocity.X, VY: eci.Velocity.Y, VZ: eci.Velocity.Z,
		}
		obs, err := sv.GetLookAngle(&loc, now)
		if err != nil {
			continue
		}
		if obs.LookAngles.Elevation < 0 {
			continue
		}
		out = append(out, skyEntry{sat: sat, az: obs.LookAngles.Azimuth, el: obs.LookAngles.Elevation, rng: obs.LookAngles.Range})
	}
	return out
}
