//go:build !darwin && !windows

package browser

// NewPlatformInputSynthesizer has no Linux implementation yet: Wayland (the
// default session type on current distributions) has no equivalent to the
// X11 XTest extension go-vgo/robotgo relies on, so OS-level input synthesis
// on Linux needs its own investigation (X11/XTest for Xorg sessions,
// uinput/ydotool for Wayland) rather than reusing the macOS or Windows
// approach. Callers get UnsupportedInputSynthesizer until that lands.
func NewPlatformInputSynthesizer() (OSInputSynthesizer, error) {
	return UnsupportedInputSynthesizer{}, nil
}
