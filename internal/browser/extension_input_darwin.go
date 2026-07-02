//go:build darwin

package browser

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>
*/
import "C"

import (
	"context"
	"errors"
	"unicode/utf16"
	"unsafe"
)

type darwinInputSynthesizer struct{}

var (
	zeroCGEventSource C.CGEventSourceRef
	zeroCGEvent       C.CGEventRef
)

// NewPlatformInputSynthesizer returns the macOS OSInputSynthesizer, built on
// Quartz Event Services (CGEventPost). Quartz's screen coordinate space is
// point-based (not raw framebuffer pixels), which is the same space
// window.screenX/screenY report in Chrome, so no explicit DPI conversion
// happens here: this identity mapping is an assumption inherited from
// elementCenterScreenPoint and has not been verified against a live Retina
// multi-display session.
func NewPlatformInputSynthesizer() (OSInputSynthesizer, error) {
	return darwinInputSynthesizer{}, nil
}

func (darwinInputSynthesizer) MoveMouse(ctx context.Context, screenX int, screenY int) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	return postMouseEvent(C.kCGEventMouseMoved, screenPointFrom(screenX, screenY), C.kCGMouseButtonLeft)
}

func (darwinInputSynthesizer) Click(ctx context.Context, screenX int, screenY int, button MouseButton) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	downType, upType, cgButton, errorValue := mouseEventTypesForButton(button)
	if errorValue != nil {
		return errorValue
	}
	point := screenPointFrom(screenX, screenY)
	if errorValue := postMouseEvent(downType, point, cgButton); errorValue != nil {
		return errorValue
	}
	return postMouseEvent(upType, point, cgButton)
}

func (darwinInputSynthesizer) TypeText(ctx context.Context, text string) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	utf16Characters := utf16.Encode([]rune(text))
	if len(utf16Characters) == 0 {
		return nil
	}
	if errorValue := postUnicodeKeyboardEvent(utf16Characters, true); errorValue != nil {
		return errorValue
	}
	return postUnicodeKeyboardEvent(utf16Characters, false)
}

func (darwinInputSynthesizer) PressKey(ctx context.Context, key string) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	virtualKeyCode, found := darwinVirtualKeyCodes[key]
	if !found {
		return errors.New("unsupported key name: " + key)
	}
	if errorValue := postVirtualKeyEvent(virtualKeyCode, true); errorValue != nil {
		return errorValue
	}
	return postVirtualKeyEvent(virtualKeyCode, false)
}

func screenPointFrom(screenX int, screenY int) C.CGPoint {
	return C.CGPoint{x: C.double(screenX), y: C.double(screenY)}
}

func mouseEventTypesForButton(button MouseButton) (C.CGEventType, C.CGEventType, C.CGMouseButton, error) {
	switch button {
	case MouseButtonLeft, "":
		return C.kCGEventLeftMouseDown, C.kCGEventLeftMouseUp, C.kCGMouseButtonLeft, nil
	case MouseButtonRight:
		return C.kCGEventRightMouseDown, C.kCGEventRightMouseUp, C.kCGMouseButtonRight, nil
	case MouseButtonMiddle:
		return C.kCGEventOtherMouseDown, C.kCGEventOtherMouseUp, C.kCGMouseButtonCenter, nil
	default:
		return 0, 0, 0, errors.New("unsupported mouse button: " + string(button))
	}
}

func postMouseEvent(eventType C.CGEventType, point C.CGPoint, button C.CGMouseButton) error {
	event := C.CGEventCreateMouseEvent(zeroCGEventSource, eventType, point, button)
	if event == zeroCGEvent {
		return errors.New("failed to create macOS mouse event")
	}
	defer C.CFRelease(C.CFTypeRef(event))
	C.CGEventSetIntegerValueField(event, C.kCGMouseEventClickState, 1)
	C.CGEventPost(C.kCGHIDEventTap, event)
	return nil
}

func postUnicodeKeyboardEvent(utf16Characters []uint16, keyDown bool) error {
	event := C.CGEventCreateKeyboardEvent(zeroCGEventSource, 0, C.bool(keyDown))
	if event == zeroCGEvent {
		return errors.New("failed to create macOS keyboard event")
	}
	defer C.CFRelease(C.CFTypeRef(event))
	C.CGEventKeyboardSetUnicodeString(event, C.UniCharCount(len(utf16Characters)), (*C.UniChar)(unsafe.Pointer(&utf16Characters[0])))
	C.CGEventPost(C.kCGHIDEventTap, event)
	return nil
}

func postVirtualKeyEvent(virtualKeyCode C.CGKeyCode, keyDown bool) error {
	event := C.CGEventCreateKeyboardEvent(zeroCGEventSource, virtualKeyCode, C.bool(keyDown))
	if event == zeroCGEvent {
		return errors.New("failed to create macOS keyboard event")
	}
	defer C.CFRelease(C.CFTypeRef(event))
	C.CGEventPost(C.kCGHIDEventTap, event)
	return nil
}

// darwinVirtualKeyCodes maps the key names ExtensionInputRuntime.Press
// forwards (JavaScript KeyboardEvent.key values, per extension_input.go) to
// the fixed physical virtual keycodes of the ANSI USB keyboard layout Apple
// documents in Events.h. TypeText covers printable Unicode text directly
// through CGEventKeyboardSetUnicodeString, so this table only needs to cover
// non-printable/control keys.
var darwinVirtualKeyCodes = map[string]C.CGKeyCode{
	"Enter":      0x24,
	"Return":     0x24,
	"Tab":        0x30,
	"Escape":     0x35,
	"Backspace":  0x33,
	"Delete":     0x75,
	"Space":      0x31,
	"ArrowLeft":  0x7B,
	"ArrowRight": 0x7C,
	"ArrowDown":  0x7D,
	"ArrowUp":    0x7E,
	"Home":       0x73,
	"End":        0x77,
	"PageUp":     0x74,
	"PageDown":   0x79,
}
