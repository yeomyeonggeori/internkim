package browser

import (
	"context"
	"errors"
)

type MouseButton string

const (
	MouseButtonLeft   MouseButton = "left"
	MouseButtonRight  MouseButton = "right"
	MouseButtonMiddle MouseButton = "middle"
)

// OSInputSynthesizer performs real OS-level input events (not CDP synthetic
// events, which the isTrusted=false flag exposes to page scripts). Screen
// coordinates are in the same CSS pixel space elementCenterScreenPoint
// produces; an implementation is responsible for converting to physical
// pixels using the platform's own DPI model. macOS and Windows
// implementations are added separately (see
// internal/browser/input_darwin.go, input_windows.go); this package only
// depends on the interface.
type OSInputSynthesizer interface {
	MoveMouse(ctx context.Context, screenX int, screenY int) error
	Click(ctx context.Context, screenX int, screenY int, button MouseButton) error
	TypeText(ctx context.Context, text string) error
	PressKey(ctx context.Context, key string) error
}

// UnsupportedInputSynthesizer is the default OSInputSynthesizer until a
// platform-specific implementation is injected. It exists so
// ExtensionInputRuntime always has a non-nil synthesizer to call.
type UnsupportedInputSynthesizer struct{}

func (UnsupportedInputSynthesizer) MoveMouse(context.Context, int, int) error {
	return errors.New("OS input synthesis is not supported on this platform build")
}

func (UnsupportedInputSynthesizer) Click(context.Context, int, int, MouseButton) error {
	return errors.New("OS input synthesis is not supported on this platform build")
}

func (UnsupportedInputSynthesizer) TypeText(context.Context, string) error {
	return errors.New("OS input synthesis is not supported on this platform build")
}

func (UnsupportedInputSynthesizer) PressKey(context.Context, string) error {
	return errors.New("OS input synthesis is not supported on this platform build")
}
