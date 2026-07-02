//go:build windows

package browser

import (
	"context"
	"errors"
	"sync"
	"unicode/utf16"
	"unsafe"
)

type windowsInputSynthesizer struct{}

var declareDpiAwarenessOnce sync.Once

// NewPlatformInputSynthesizer returns the Windows OSInputSynthesizer, built
// on the user32 SendInput syscall. It also marks this process per-monitor
// DPI aware so GetSystemMetrics reports the virtual screen in the same
// physical-pixel space Chrome (itself per-monitor DPI aware) uses for
// window.screenX/screenY: without this, a DPI-unaware process would see a
// scaled-down virtual screen and misplace clicks on any monitor that is not
// at 100% scaling, which is the mixed-DPI multi-monitor risk called out in
// the design doc.
func NewPlatformInputSynthesizer() (OSInputSynthesizer, error) {
	declareDpiAwarenessOnce.Do(declarePerMonitorDpiAwareness)
	return windowsInputSynthesizer{}, nil
}

func (windowsInputSynthesizer) MoveMouse(ctx context.Context, screenX int, screenY int) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	return sendAbsoluteMouseInput(screenX, screenY, mouseEventFlagMove)
}

func (windowsInputSynthesizer) Click(ctx context.Context, screenX int, screenY int, button MouseButton) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	downFlag, upFlag, errorValue := mouseEventFlagsForButton(button)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := sendAbsoluteMouseInput(screenX, screenY, mouseEventFlagMove|downFlag); errorValue != nil {
		return errorValue
	}
	return sendAbsoluteMouseInput(screenX, screenY, mouseEventFlagMove|upFlag)
}

func (windowsInputSynthesizer) TypeText(ctx context.Context, text string) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	for _, utf16CodeUnit := range utf16.Encode([]rune(text)) {
		if errorValue := sendUnicodeKeyboardInput(utf16CodeUnit); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (windowsInputSynthesizer) PressKey(ctx context.Context, key string) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	virtualKeyCode, found := windowsVirtualKeyCodes[key]
	if !found {
		return errors.New("unsupported key name: " + key)
	}
	if errorValue := sendVirtualKeyInput(virtualKeyCode, keyEventFlagNone); errorValue != nil {
		return errorValue
	}
	return sendVirtualKeyInput(virtualKeyCode, keyEventFlagKeyUp)
}

func mouseEventFlagsForButton(button MouseButton) (uint32, uint32, error) {
	switch button {
	case MouseButtonLeft, "":
		return mouseEventFlagLeftDown, mouseEventFlagLeftUp, nil
	case MouseButtonRight:
		return mouseEventFlagRightDown, mouseEventFlagRightUp, nil
	case MouseButtonMiddle:
		return mouseEventFlagMiddleDown, mouseEventFlagMiddleUp, nil
	default:
		return 0, 0, errors.New("unsupported mouse button: " + string(button))
	}
}

func sendAbsoluteMouseInput(screenX int, screenY int, flags uint32) error {
	virtualScreenLeft := int(getSystemMetric(systemMetricVirtualScreenLeft))
	virtualScreenTop := int(getSystemMetric(systemMetricVirtualScreenTop))
	virtualScreenWidth := int(getSystemMetric(systemMetricVirtualScreenWidth))
	virtualScreenHeight := int(getSystemMetric(systemMetricVirtualScreenHeight))
	input := newMouseInput(
		int32(scaleToAbsoluteAxis(screenX, virtualScreenLeft, virtualScreenWidth)),
		int32(scaleToAbsoluteAxis(screenY, virtualScreenTop, virtualScreenHeight)),
		flags|mouseEventFlagAbsolute|mouseEventFlagVirtualDesk,
	)
	return sendSingleInput(unsafe.Pointer(&input), unsafe.Sizeof(input))
}

func sendUnicodeKeyboardInput(utf16CodeUnit uint16) error {
	downInput := newUnicodeKeyboardInput(utf16CodeUnit, keyEventFlagUnicode)
	if errorValue := sendSingleInput(unsafe.Pointer(&downInput), unsafe.Sizeof(downInput)); errorValue != nil {
		return errorValue
	}
	upInput := newUnicodeKeyboardInput(utf16CodeUnit, keyEventFlagUnicode|keyEventFlagKeyUp)
	return sendSingleInput(unsafe.Pointer(&upInput), unsafe.Sizeof(upInput))
}

func sendVirtualKeyInput(virtualKeyCode uint16, extraFlags uint32) error {
	input := newVirtualKeyboardInput(virtualKeyCode, extraFlags)
	return sendSingleInput(unsafe.Pointer(&input), unsafe.Sizeof(input))
}

func sendSingleInput(inputPointer unsafe.Pointer, inputSize uintptr) error {
	sentCount, _, lastError := procSendInput.Call(1, uintptr(inputPointer), inputSize)
	if sentCount == 0 {
		return errors.New("SendInput failed: " + lastError.Error())
	}
	return nil
}

func getSystemMetric(metricIndex int32) int32 {
	value, _, _ := procGetSystemMetrics.Call(uintptr(metricIndex))
	return int32(value)
}

func declarePerMonitorDpiAwareness() {
	_, _, _ = procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
}

// windowsVirtualKeyCodes maps the key names ExtensionInputRuntime.Press
// forwards (JavaScript KeyboardEvent.key values, per extension_input.go) to
// Windows virtual-key codes. TypeText covers printable Unicode text directly
// through KEYEVENTF_UNICODE, so this table only needs to cover
// non-printable/control keys.
var windowsVirtualKeyCodes = map[string]uint16{
	"Enter":      0x0D,
	"Return":     0x0D,
	"Tab":        0x09,
	"Escape":     0x1B,
	"Backspace":  0x08,
	"Delete":     0x2E,
	"Space":      0x20,
	"ArrowLeft":  0x25,
	"ArrowUp":    0x26,
	"ArrowRight": 0x27,
	"ArrowDown":  0x28,
	"Home":       0x24,
	"End":        0x23,
	"PageUp":     0x21,
	"PageDown":   0x22,
}
