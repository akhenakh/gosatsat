package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akhenakh/maprender"
	. "go.hasen.dev/shirei"
)

const mapTileSize = 256

// mapRenderW/mapRenderH is the working resolution of the base map. It is wider
// than tall so the map fills a typical desktop window.
const (
	mapRenderW = 1440
	mapRenderH = 810
)

var (
	mapStyleMu     sync.Mutex
	cachedStyle    *maprender.MapStyle
	mapStyleErr    error
	mapStyleURL    = "https://tiles.openfreemap.org/styles/liberty"
	mapRenderCalls atomic.Int64 // number of base-map renders (tests)
)

func worldMapStyle() (*maprender.MapStyle, error) {
	mapStyleMu.Lock()
	defer mapStyleMu.Unlock()
	if cachedStyle != nil || mapStyleErr != nil {
		return cachedStyle, mapStyleErr
	}
	st, err := maprender.FetchStyle(mapStyleURL)
	if err != nil {
		mapStyleErr = err
		return nil, err
	}
	cachedStyle = st
	return st, nil
}

func mercatorWorld(lat, lng float64, zoom int) (x, y float64) {
	world := float64(mapTileSize) * math.Pow(2, float64(zoom))
	lat = math.Max(-85.05112878, math.Min(85.05112878, lat))
	latRad := lat * math.Pi / 180
	x = (lng + 180) / 360 * world
	y = (1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * world
	return x, y
}

// projectToMap maps lat/lng to a pixel in a map image of w x h centered at
// centerLat/centerLng and rendered at the given zoom.
func projectToMap(lat, lng, centerLat, centerLng float64, zoom, w, h int) (float64, float64) {
	gx, gy := mercatorWorld(lat, lng, zoom)
	cx, cy := mercatorWorld(centerLat, centerLng, zoom)
	return gx - cx + float64(w)/2, gy - cy + float64(h)/2
}

// renderWorldMap renders the base map (no markers) using maprender.
func renderWorldMap(ctx context.Context, centerLat, centerLng float64, zoom, w, h int) (*image.RGBA, error) {
	mapRenderCalls.Add(1)
	style, err := worldMapStyle()
	if err != nil {
		return nil, err
	}
	req := maprender.RenderRequest{
		CenterLat:        centerLat,
		CenterLng:        centerLng,
		Zoom:             zoom,
		Width:            w,
		Height:           h,
		DevicePixelRatio: 1,
		Style:            style,
	}
	return maprender.Render(ctx, req)
}

// simpleWorldFallback draws a graticule-only world map when tile rendering is
// unavailable, so the map tab still conveys satellite positions.
func simpleWorldFallback(centerLat, centerLng float64, zoom, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{22, 34, 48, 255}), image.Point{}, draw.Src)

	grid := color.RGBA{44, 66, 88, 255}
	for lng := -180; lng <= 180; lng += 30 {
		x, _ := projectToMap(0, float64(lng), centerLat, centerLng, zoom, w, h)
		drawLine(img, x, 0, x, float64(h), grid, 1)
	}
	for lat := -80; lat <= 80; lat += 20 {
		_, y := projectToMap(float64(lat), 0, centerLat, centerLng, zoom, w, h)
		drawLine(img, 0, y, float64(w), y, grid, 1)
	}
	return img
}

// requestMapRender starts a background base-map render if one is not running.
// The base is what is expensive (tiles, fonts, labels); satellite positions
// are drawn as an overlay by the view and never trigger this.
func (s *State) requestMapRender() {
	if s.mapRendering {
		return
	}
	s.mapRendering = true
	s.mapErr = ""

	centerLat := s.mapCenterLat
	centerLng := s.mapCenterLng
	zoom := s.mapZoom

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		img, err := renderWorldMap(ctx, centerLat, centerLng, zoom, mapRenderW, mapRenderH)
		if err != nil {
			img = simpleWorldFallback(centerLat, centerLng, zoom, mapRenderW, mapRenderH)
		}
		WithFrameLock(func() {
			s.mapBase = img
			s.mapBaseKey = mapKey(centerLat, centerLng, zoom)
			s.mapBaseCenterLat = centerLat
			s.mapBaseCenterLng = centerLng
			s.mapBaseZoom = zoom
			s.mapRendering = false
			if err != nil {
				s.mapErr = fmt.Sprintf("map tiles unavailable (%v); showing simple grid", err)
			}
		})
		RequestNextFrame()
	}()
}

func mapKey(lat, lng float64, zoom int) string {
	return fmt.Sprintf("%.3f/%.3f/%d", lat, lng, zoom)
}

// prepareDemoMap renders the base map synchronously for `--demo --tab map`
// snapshots, falling back to the graticule when tiles are unreachable.
func prepareDemoMap() {
	lat, lng, zoom := st.mapCenterLat, st.mapCenterLng, st.mapZoom
	if st.cfg.Location != nil {
		lat, lng = st.cfg.Location.Lat, st.cfg.Location.Lng
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base, err := renderWorldMap(ctx, lat, lng, zoom, mapRenderW, mapRenderH)
	if err != nil {
		base = simpleWorldFallback(lat, lng, zoom, mapRenderW, mapRenderH)
	}
	st.mapCenterLat, st.mapCenterLng, st.mapInit = lat, lng, true
	st.mapBase = base
	st.mapBaseKey = mapKey(lat, lng, zoom)
	st.mapBaseCenterLat, st.mapBaseCenterLng, st.mapBaseZoom = lat, lng, zoom
}
