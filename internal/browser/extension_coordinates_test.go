package browser

import "testing"

func TestElementCenterScreenPointAddsChromeOffsetAndWindowPosition(t *testing.T) {
	rect := ExtensionElementRect{X: 100, Y: 40, Width: 80, Height: 20}
	viewport := ExtensionViewportGeometry{
		ScreenX:          200,
		ScreenY:          50,
		DevicePixelRatio: 2,
		OuterWidth:       1200,
		InnerWidth:       1184,
		OuterHeight:      900,
		InnerHeight:      812,
	}

	point := elementCenterScreenPoint(rect, viewport)

	expectedX := 200 + (1200 - 1184) + (100 + 80/2)
	expectedY := 50 + (900 - 812) + (40 + 20/2)
	if point.X != expectedX || point.Y != expectedY {
		t.Fatalf("unexpected screen point: got %+v, expected {%d %d}", point, expectedX, expectedY)
	}
}

func TestElementCenterScreenPointRoundsFractionalCoordinates(t *testing.T) {
	rect := ExtensionElementRect{X: 10.4, Y: 10.6, Width: 1, Height: 1}
	viewport := ExtensionViewportGeometry{ScreenX: 0, ScreenY: 0, OuterWidth: 100, InnerWidth: 100, OuterHeight: 100, InnerHeight: 100}

	point := elementCenterScreenPoint(rect, viewport)

	if point.X != 11 || point.Y != 11 {
		t.Fatalf("expected rounded coordinates {11 11}, got %+v", point)
	}
}

func TestElementCenterScreenPointWithZeroChromeOffset(t *testing.T) {
	rect := ExtensionElementRect{X: 0, Y: 0, Width: 200, Height: 40}
	viewport := ExtensionViewportGeometry{ScreenX: 500, ScreenY: 300, OuterWidth: 800, InnerWidth: 800, OuterHeight: 600, InnerHeight: 600}

	point := elementCenterScreenPoint(rect, viewport)

	if point.X != 600 || point.Y != 320 {
		t.Fatalf("unexpected screen point with zero chrome offset: %+v", point)
	}
}
