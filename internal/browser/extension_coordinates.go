package browser

import "math"

type ScreenPoint struct {
	X int
	Y int
}

// elementCenterScreenPoint converts a viewport-relative element rect into an
// OS screen coordinate by adding the browser chrome offset (tab strip,
// address bar, window borders) and the window's screen position. Values stay
// in the CSS pixel space the browser itself reports (window.screenX and
// friends); a platform OSInputSynthesizer is responsible for any further
// conversion to physical pixels using viewport.DevicePixelRatio.
func elementCenterScreenPoint(rect ExtensionElementRect, viewport ExtensionViewportGeometry) ScreenPoint {
	chromeOffsetWidth := viewport.OuterWidth - viewport.InnerWidth
	chromeOffsetHeight := viewport.OuterHeight - viewport.InnerHeight
	viewportCenterX := rect.X + rect.Width/2
	viewportCenterY := rect.Y + rect.Height/2
	screenX := viewport.ScreenX + chromeOffsetWidth + viewportCenterX
	screenY := viewport.ScreenY + chromeOffsetHeight + viewportCenterY
	return ScreenPoint{
		X: int(math.Round(screenX)),
		Y: int(math.Round(screenY)),
	}
}
