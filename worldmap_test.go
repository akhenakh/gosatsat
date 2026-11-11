package main

import (
	"math"
	"testing"

	"github.com/akhenakh/maprender"
)

func TestProjectToMapMatchesRendererScale(t *testing.T) {
	const zoom = 2
	const w, h = 1440, 810

	// The center maps to the middle of the image.
	x, y := projectToMap(0, 0, 0, 0, zoom, w, h)
	if math.Abs(x-float64(w)/2) > 1e-9 || math.Abs(y-float64(h)/2) > 1e-9 {
		t.Fatalf("center maps to (%.3f, %.3f), want (%.1f, %.1f)", x, y, float64(w)/2, float64(h)/2)
	}

	// One full world east must be exactly TileSize*2^zoom pixels away, the scale
	// the base map is rendered at.
	want := float64(maprender.TileSize) * math.Pow(2, zoom)
	xe, _ := projectToMap(0, 360, 0, 0, zoom, w, h)
	if got := xe - x; math.Abs(got-want) > 1e-9 {
		t.Fatalf("one world is %.3f px wide, want %.3f", got, want)
	}
}

// TestISSMarkerIsOverThePacific is the reported case: with the map centered on
// Québec, the ISS at 10N 161.6W must land on the Pacific, not the US.
func TestISSMarkerIsOverThePacific(t *testing.T) {
	const (
		lat, lng     = 10.0015, -161.5984 // trusted ISS position
		centerLat    = 46.81228           // Québec
		centerLng    = -71.21454
		zoom         = 2
		w, h         = 1440, 810
		pacificLimit = -130.0 // anything east of this is the US west coast
	)

	x, _ := projectToMap(lat, lng, centerLat, centerLng, zoom, w, h)
	world := float64(maprender.TileSize) * math.Pow(2, zoom)
	markerLng := (x-float64(w)/2)/world*360 + centerLng

	if markerLng > pacificLimit {
		t.Fatalf("ISS marker at pixel %.1f is longitude %.2f, over land; want the Pacific", x, markerLng)
	}
	if math.Abs(markerLng-lng) > 0.01 {
		t.Fatalf("marker longitude %.3f, want %.3f", markerLng, lng)
	}
}
