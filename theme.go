package main

import (
	. "go.hasen.dev/shirei"
	. "go.hasen.dev/shirei/widgets"
)

// Palette inspired by the historical SatSat iOS app: a dark navy chrome, a
// vivid red accent, white content, and blue live markers.
var (
	satsatNavy     = Vec4{223, 49, 27, 1} // #223668
	satsatNavyDeep = Vec4{223, 52, 19, 1}
	satsatRed      = Vec4{356, 82, 52, 1} // #e91e2b-ish
	satsatRedSoft  = Vec4{356, 70, 90, 1}
)

// setupTheme installs the SatSat-inspired light and dark schemes. Call once
// before the first frame, and before SetDarkMode in RootView.
func setupTheme() {
	SetLightColorScheme(satsatLightScheme())
	SetDarkColorScheme(satsatDarkScheme())
}

// muted returns the secondary text style for the active theme.
func muted() TextStyleFn {
	if st.cfg.DarkMode {
		return TextColorVec(Vec4{220, 12, 68, 1})
	}
	return TextColorVec(Vec4{220, 14, 42, 1})
}

// warnText returns the alert text style for the active theme.
func warnText() TextStyleFn {
	if st.cfg.DarkMode {
		return TextColorVec(Vec4{0, 80, 70, 1})
	}
	return TextColorVec(Vec4{356, 80, 45, 1})
}

func satsatLightScheme() ColorScheme {
	s := LightColorScheme()
	s.Surfaces.Canvas = SurfaceColors{Background: Vec4{220, 12, 97, 1}, Text: Vec4{220, 14, 15, 1}, Border: Vec4{220, 12, 82, 1}}
	s.Surfaces.Panel = SurfaceColors{Background: Vec4{0, 0, 100, 1}, Text: Vec4{0, 0, 12, 1}, Border: Vec4{220, 12, 87, 1}}
	s.Surfaces.Toolbar = SurfaceColors{Background: satsatNavy, Text: Vec4{0, 0, 100, 1}, Border: satsatNavyDeep}
	s.Buttons.Primary = ButtonStyleWithAccent(s.Buttons.Primary, satsatRed)
	s.Buttons.Destructive = ButtonStyleWithAccent(s.Buttons.Destructive, Vec4{356, 75, 44, 1})
	s.FocusRing = satsatRed
	s.CheckBox = SelectionStyleWithAccent(s.CheckBox, satsatRed)
	s.Segmented = SelectionStyleWithAccent(s.Segmented, satsatRed)
	s.Slider.Track = satsatRed
	s.Table.Header = SurfaceColors{Background: Vec4{223, 22, 92, 1}, Text: satsatNavy, Border: Vec4{223, 18, 84, 1}}
	s.Table.Body = SurfaceColors{Background: Vec4{0, 0, 100, 1}, Text: Vec4{0, 0, 12, 1}, Border: Vec4{220, 12, 88, 1}}
	s.Table.Hovered = Vec4{223, 40, 95, 1}
	s.Table.Sorted = Vec4{223, 35, 92, 1}
	s.ScrollBar = ScrollBarStyle{
		Track:   Vec4{0, 0, 0, 0},
		Normal:  Vec4{223, 20, 70, 0.6},
		Hovered: Vec4{223, 25, 55, 0.8},
		Pressed: Vec4{223, 30, 45, 0.9},
	}
	return s
}

func satsatDarkScheme() ColorScheme {
	s := DarkColorScheme()
	s.Surfaces.Canvas = SurfaceColors{Background: Vec4{220, 18, 12, 1}, Text: Vec4{220, 12, 90, 1}, Border: Vec4{220, 14, 22, 1}}
	s.Surfaces.Panel = SurfaceColors{Background: Vec4{220, 16, 17, 1}, Text: Vec4{220, 12, 90, 1}, Border: Vec4{220, 14, 26, 1}}
	s.Surfaces.Toolbar = SurfaceColors{Background: satsatNavyDeep, Text: Vec4{0, 0, 100, 1}, Border: Vec4{223, 54, 12, 1}}
	s.Buttons.Primary = ButtonStyleWithAccent(s.Buttons.Primary, Vec4{356, 74, 50, 1})
	s.FocusRing = Vec4{356, 80, 62, 1}
	s.CheckBox = SelectionStyleWithAccent(s.CheckBox, Vec4{356, 74, 50, 1})
	s.Segmented = SelectionStyleWithAccent(s.Segmented, Vec4{356, 74, 50, 1})
	s.Slider.Track = Vec4{356, 74, 52, 1}
	s.Table.Header = SurfaceColors{Background: Vec4{223, 30, 18, 1}, Text: Vec4{0, 0, 92, 1}, Border: Vec4{223, 25, 26, 1}}
	s.Table.Hovered = Vec4{223, 30, 22, 1}
	s.Table.Sorted = Vec4{223, 32, 26, 1}
	return s
}
