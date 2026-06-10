package main

import "testing"

func TestNormalizeWindowStateBumpsUndersizedGeometry(t *testing.T) {
	got := normalizeWindowState(DesktopWindowState{Width: 800, Height: 500, X: 10, Y: 20})
	if got.Width != DefaultWindowWidth {
		t.Fatalf("width = %d, want %d", got.Width, DefaultWindowWidth)
	}
	if got.Height != DefaultWindowHeight {
		t.Fatalf("height = %d, want %d", got.Height, DefaultWindowHeight)
	}
	if got.X != 10 || got.Y != 20 {
		t.Fatalf("position changed: (%d,%d)", got.X, got.Y)
	}
}

func TestFitWindowSizeCapsToScreen(t *testing.T) {
	width, height := fitWindowSize(1920, 1080, 1920, 1080)
	maxW := 1651
	maxH := 928
	if width > maxW || height > maxH {
		t.Fatalf("fitWindowSize = %dx%d, want <= %dx%d", width, height, maxW, maxH)
	}
	if width < MinWindowWidth || height < MinWindowHeight {
		t.Fatalf("fitWindowSize below minimum: %dx%d", width, height)
	}
}

func TestNormalizeWindowStateKeepsComfortableGeometry(t *testing.T) {
	in := DesktopWindowState{Width: 1400, Height: 900, X: 0, Y: 0}
	got := normalizeWindowState(in)
	if got != in {
		t.Fatalf("normalize changed comfortable geometry: %+v -> %+v", in, got)
	}
}
