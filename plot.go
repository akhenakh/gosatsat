package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"
	"time"

	"github.com/akhenakh/sgp4"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// plotColors mirrors the historical SatSat sky view: a white canvas, dashed
// grey range rings, a grey crosshair, red cardinal letters, a pass path that
// fades from green at AOS to red at LOS, and blue live-satellite markers.
type plotColors struct {
	bg, grid, cross, cardinal, text, dim color.RGBA
	pathStart, pathEnd                   color.RGBA
	marker, markerLabel                  color.RGBA
	aos, los, max                        color.RGBA
}

func plotColorsFor(dark bool) plotColors {
	if dark {
		return plotColors{
			bg:          color.RGBA{17, 21, 30, 255},
			grid:        color.RGBA{110, 120, 140, 255},
			cross:       color.RGBA{80, 92, 112, 255},
			cardinal:    color.RGBA{235, 45, 36, 255},
			text:        color.RGBA{220, 228, 240, 255},
			dim:         color.RGBA{140, 152, 170, 255},
			pathStart:   color.RGBA{90, 225, 120, 255},
			pathEnd:     color.RGBA{240, 70, 60, 255},
			marker:      color.RGBA{120, 180, 255, 255},
			markerLabel: color.RGBA{150, 200, 255, 255},
			aos:         color.RGBA{90, 225, 120, 255},
			los:         color.RGBA{240, 70, 60, 255},
			max:         color.RGBA{255, 255, 255, 255},
		}
	}
	return plotColors{
		bg:          color.RGBA{255, 255, 255, 255},
		grid:        color.RGBA{85, 85, 85, 255},
		cross:       color.RGBA{85, 85, 85, 255},
		cardinal:    color.RGBA{255, 0, 0, 255},
		text:        color.RGBA{35, 35, 40, 255},
		dim:         color.RGBA{110, 110, 115, 255},
		pathStart:   color.RGBA{0, 200, 0, 255},
		pathEnd:     color.RGBA{235, 45, 36, 255},
		marker:      color.RGBA{0, 0, 235, 255},
		markerLabel: color.RGBA{0, 0, 220, 255},
		aos:         color.RGBA{0, 185, 0, 255},
		los:         color.RGBA{225, 40, 30, 255},
		max:         color.RGBA{40, 40, 40, 255},
	}
}

var (
	faceMu    sync.Mutex
	faceCache = map[int]font.Face{}
)

func goFace(size float64) font.Face {
	key := int(size * 4)
	faceMu.Lock()
	defer faceMu.Unlock()
	if f, ok := faceCache[key]; ok {
		return f
	}
	src, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil
	}
	faceCache[key] = f
	return f
}

func newPlot(size int, c plotColors) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.NewUniform(c.bg), image.Point{}, draw.Src)
	return img
}

func drawDisc(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	if r < 0.5 {
		r = 0.5
	}
	x0 := int(math.Floor(cx - r))
	x1 := int(math.Ceil(cx + r))
	y0 := int(math.Floor(cy - r))
	y1 := int(math.Ceil(cy + r))
	r2 := r * r
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 float64, c color.RGBA, width float64) {
	dx := x1 - x0
	dy := y1 - y0
	steps := int(math.Hypot(dx, dy))
	if steps <= 0 {
		drawDisc(img, x0, y0, width/2, c)
		return
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		drawDisc(img, x0+dx*t, y0+dy*t, width/2, c)
	}
}

func drawCircle(img *image.RGBA, cx, cy, r float64, c color.RGBA, width float64) {
	steps := int(math.Max(48, r))
	for i := 0; i <= steps; i++ {
		a := 2 * math.Pi * float64(i) / float64(steps)
		drawDisc(img, cx+r*math.Cos(a), cy+r*math.Sin(a), width/2, c)
	}
}

// drawDashedCircle draws a circle in short dashes, like the SatSat range rings.
func drawDashedCircle(img *image.RGBA, cx, cy, r float64, c color.RGBA, width float64, on, off int) {
	steps := int(math.Max(96, r*2))
	period := on + off
	for i := 0; i < steps; i++ {
		if i%period >= on {
			continue
		}
		a := 2 * math.Pi * float64(i) / float64(steps)
		drawDisc(img, cx+r*math.Cos(a), cy+r*math.Sin(a), width/2, c)
	}
}

func drawText(img *image.RGBA, x, y float64, size float64, c color.RGBA, text string) {
	f := goFace(size)
	if f == nil {
		return
	}
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f}
	d.Dot = fixed.P(int(x), int(y))
	d.DrawString(text)
}

func drawTextCentered(img *image.RGBA, cx, cy, size float64, c color.RGBA, text string) {
	f := goFace(size)
	if f == nil {
		return
	}
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f}
	w := d.MeasureString(text).Ceil()
	m := f.Metrics()
	baseline := cy + float64(m.Ascent-m.Descent)/2/64
	drawText(img, cx-float64(w)/2, baseline, size, c, text)
}

func polarXY(cx, cy, radius, az, el float64) (float64, float64) {
	r := radius * (1 - el/90)
	if el < 0 {
		r = radius * (1 + math.Abs(el)/90)
	}
	a := az * math.Pi / 180
	return cx + r*math.Sin(a), cy - r*math.Cos(a)
}

// fadeRGB interpolates between two colors (SatSat's green→red pass fade).
func fadeRGB(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	lerp := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.RGBA{lerp(a.R, b.R), lerp(a.G, b.G), lerp(a.B, b.B), 255}
}

// drawSkyGrid draws the SatSat polar grid: dashed rings at 1/3, 2/3 and the
// horizon, a crosshair spanning the whole plot, and red cardinal letters.
func drawSkyGrid(img *image.RGBA, size int, c plotColors) {
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 2
	for i := 1; i <= 3; i++ {
		drawDashedCircle(img, cx, cy, radius*float64(i)/3, c.grid, 1, 2, 2)
	}
	drawLine(img, 0, cy, float64(size), cy, c.cross, 1)
	drawLine(img, cx, 0, cx, float64(size), c.cross, 1)

	drawTextCentered(img, cx, radius*0.06+8, 15, c.cardinal, "N")
	drawTextCentered(img, cx, float64(size)-6, 15, c.cardinal, "S")
	drawTextCentered(img, 8, cy, 15, c.cardinal, "W")
	drawTextCentered(img, float64(size)-8, cy, 15, c.cardinal, "E")
}

// passPlotImage renders a polar plot of a single pass in the SatSat style.
func passPlotImage(p *Pass, size int, dark bool) *image.RGBA {
	c := plotColorsFor(dark)
	img := newPlot(size, c)
	if size < 80 {
		return img
	}
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 2

	drawSkyGrid(img, size, c)

	pts := p.DataPoints
	if len(pts) < 2 {
		drawTextCentered(img, cx, cy, 14, c.text, "No data points")
		return img
	}

	for i := 1; i < len(pts); i++ {
		a := pts[i-1]
		b := pts[i]
		t := float64(i) / float64(len(pts)-1)
		col := fadeRGB(c.pathStart, c.pathEnd, t)
		x0, y0 := polarXY(cx, cy, radius, a.Azimuth, a.Elevation)
		x1, y1 := polarXY(cx, cy, radius, b.Azimuth, b.Elevation)
		drawLine(img, x0, y0, x1, y1, col, 3)
	}

	first, last := pts[0], pts[len(pts)-1]
	ax, ay := polarXY(cx, cy, radius, first.Azimuth, first.Elevation)
	lx, ly := polarXY(cx, cy, radius, last.Azimuth, last.Elevation)
	mx, my := polarXY(cx, cy, radius, p.Details.MaxElevationAz, p.Details.MaxElevation)
	drawDisc(img, ax, ay, 5, c.aos)
	drawDisc(img, lx, ly, 5, c.los)
	drawDisc(img, mx, my, 4, c.max)

	drawText(img, 8, 18, 12, c.text, fmt.Sprintf("%s  max %.1f°", p.Name, p.Details.MaxElevation))
	drawText(img, 8, float64(size)-8, 11, c.dim, "AOS "+first.Timestamp.Local().Format("15:04:05")+"   LOS "+last.Timestamp.Local().Format("15:04:05"))
	return img
}

// skyPlotImage renders the live sky in the SatSat style: blue × markers with
// labels for every tracked satellite above the horizon.
func skyPlotImage(size int, loc sgp4.Location, sats []*Sat, now time.Time, dark bool) *image.RGBA {
	c := plotColorsFor(dark)
	img := newPlot(size, c)
	if size < 80 {
		return img
	}
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 2

	drawSkyGrid(img, size, c)

	entries := visibleSatellites(loc, sats, now)
	for _, e := range entries {
		x, y := polarXY(cx, cy, radius, e.az, e.el)
		// Blue "×" marker, as in the original live view.
		drawLine(img, x-4, y-4, x+4, y+4, c.marker, 2)
		drawLine(img, x-4, y+4, x+4, y-4, c.marker, 2)
		drawText(img, x+8, y+4, 11, c.markerLabel, e.sat.Name)
	}

	noun := "satellites"
	if len(entries) == 1 {
		noun = "satellite"
	}
	drawText(img, 8, 18, 12, c.text, fmt.Sprintf("%d %s above horizon", len(entries), noun))
	drawText(img, 8, float64(size)-8, 11, c.dim, "Azimuth / Elevation at "+now.Local().Format("15:04:05"))
	return img
}
