package main

import (
	"fmt"

	. "go.hasen.dev/shirei"
	app "go.hasen.dev/shirei/app"
	. "go.hasen.dev/shirei/widgets"
)

func mapView() {
	Container(Attrs(Viewport), func() {
		mapToolbar()
		Container(Attrs(Grow(1), Expand, Clip, Center, Pad(8)), func() {
			base := ensureMapBase()
			if base == nil {
				Container(Attrs(Center, Gap(8)), func() {
					Label("Rendering map…", FontSize(15))
					if st.mapErr != "" {
						Label(st.mapErr, FontSize(12), warnText())
					}
				})
				return
			}
			availW := GetContentWidth()
			availH := GetContentHeight()
			if availW < 32 || availH < 32 {
				return
			}
			bw := float32(base.Bounds().Dx())
			bh := float32(base.Bounds().Dy())
			disp := RestrictedSize(Vec2{bw, bh}, Vec2{availW, availH})
			dw, dh := disp[0], disp[1]
			id := UseImage("worldmap", base)
			ContainerWithKey("map-canvas", Attrs(FixSize(dw, dh), Clip, NoAnimate), func() {
				ImageViewAt(id, Vec2{dw, dh})
				mapOverlay(bw, bh, dw, dh)
			})
			if st.mapErr != "" {
				Container(Attrs(Float(6, 6), ClickThrough), func() {
					Label(st.mapErr, FontSize(11), warnText())
				})
			}
		})
	})
}

// mapOverlay draws the observer and each tracked satellite as cheap widgets on
// top of the cached base map. It only reads the live positions, so it can run
// every frame while the expensive maprender base stays untouched.
func mapOverlay(bw, bh, dw, dh float32) {
	sx := dw / bw
	sy := dh / bh
	clat, clng, zoom := st.mapBaseCenterLat, st.mapBaseCenterLng, st.mapBaseZoom

	if st.cfg.Location != nil {
		ox, oy := projectToMap(st.cfg.Location.Lat, st.cfg.Location.Lng, clat, clng, zoom, int(bw), int(bh))
		observerMarker(float32(ox)*sx, float32(oy)*sy)
	}

	for _, sat := range st.trackedSats() {
		if !sat.HasPos {
			continue
		}
		x, y := projectToMap(sat.Lat, sat.Lng, clat, clng, zoom, int(bw), int(bh))
		fx, fy := float32(x)*sx, float32(y)*sy
		if fx < -30 || fy < -30 || fx > dw+30 || fy > dh+30 {
			continue
		}
		satelliteMarker(sat, fx, fy)
	}
}

func observerMarker(x, y float32) {
	Container(Attrs(Float(x-7, y-7), FixSize(14, 14), Corners(7), Background(224, 70, 45, 1), BorderWidth(2), BorderColor(0, 0, 100, 1), ClickThrough), func() {})
	Container(Attrs(Float(x+10, y-8), ClickThrough), func() {
		Label("You", FontSize(11), TextColor(224, 70, 45, 1))
	})
}

func satelliteMarker(sat *Sat, x, y float32) {
	Container(Attrs(Float(x-14, y-19), ClickThrough), func() {
		Image(app.ResourcePath("sat_marker.png"), Vec2{28, 38})
	})
	Container(Attrs(Float(x+14, y-8), ClickThrough), func() {
		Label(sat.Name, FontSize(11), TextColor(223, 50, 20, 1))
	})
}

func mapToolbar() {
	Container(Attrs(Row, Expand, FixHeight(40), UseSurface(SurfacePanel), Pad2(0, 12), Gap(8), CrossMid), func() {
		Icon(TypGlobe, FontSize(18))
		Label("World map", FontWeight(WeightBold))
		Label(fmt.Sprintf("center %.1f, %.1f · zoom %d", st.mapCenterLat, st.mapCenterLng, st.mapZoom), FontSize(12), muted())
		Filler(1)
		if Button(SymZoomOut, "") && st.mapZoom > 0 {
			st.mapZoom--
			st.invalidateMap()
		}
		if Button(SymZoomIn, "") && st.mapZoom < 6 {
			st.mapZoom++
			st.invalidateMap()
		}
		if Button(TypLocation, "Center on me") {
			if st.cfg.Location != nil {
				st.mapCenterLat = st.cfg.Location.Lat
				st.mapCenterLng = st.cfg.Location.Lng
				st.invalidateMap()
			}
		}
	})
}

// invalidateMap forces the base map to be re-rendered at the new center/zoom.
func (s *State) invalidateMap() {
	s.mapBaseKey = ""
	s.requestMapRender()
}
