//go:build windows

package browser

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                            = windows.NewLazySystemDLL("user32.dll")
	procSendInput                     = user32.NewProc("SendInput")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
)

const (
	systemMetricVirtualScreenLeft   = 76 // SM_XVIRTUALSCREEN
	systemMetricVirtualScreenTop    = 77 // SM_YVIRTUALSCREEN
	systemMetricVirtualScreenWidth  = 78 // SM_CXVIRTUALSCREEN
	systemMetricVirtualScreenHeight = 79 // SM_CYVIRTUALSCREEN
)

// dpiAwarenessContextPerMonitorAwareV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
// (winuser.h defines it as the pointer-sized value -4).
const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(0) - 3

const (
	inputTypeMouse    = 0 // INPUT_MOUSE
	inputTypeKeyboard = 1 // INPUT_KEYBOARD
)

const (
	mouseEventFlagMove        = 0x0001 // MOUSEEVENTF_MOVE
	mouseEventFlagAbsolute    = 0x8000 // MOUSEEVENTF_ABSOLUTE
	mouseEventFlagVirtualDesk = 0x4000 // MOUSEEVENTF_VIRTUALDESK
	mouseEventFlagLeftDown    = 0x0002 // MOUSEEVENTF_LEFTDOWN
	mouseEventFlagLeftUp      = 0x0004 // MOUSEEVENTF_LEFTUP
	mouseEventFlagRightDown   = 0x0008 // MOUSEEVENTF_RIGHTDOWN
	mouseEventFlagRightUp     = 0x0010 // MOUSEEVENTF_RIGHTUP
	mouseEventFlagMiddleDown  = 0x0020 // MOUSEEVENTF_MIDDLEDOWN
	mouseEventFlagMiddleUp    = 0x0040 // MOUSEEVENTF_MIDDLEUP
)

const (
	keyEventFlagNone    = 0x0000
	keyEventFlagKeyUp   = 0x0002 // KEYEVENTF_KEYUP
	keyEventFlagUnicode = 0x0004 // KEYEVENTF_UNICODE
)

// mouseInput and keyboardInput each mirror the Win32 INPUT struct (a tagged
// union of MOUSEINPUT/KEYBDINPUT/HARDWAREINPUT) rather than embedding the
// union directly, because Go has no union type. SendInput validates cbSize
// against the real sizeof(INPUT) (40 bytes on amd64/arm64: a 4-byte type tag,
// 4 bytes of padding to 8-byte-align the union, then the union sized to its
// largest member, MOUSEINPUT), so both structs below carry explicit trailing
// padding to reach that same 40-byte size even though KEYBDINPUT's own
// fields only use the first 24 of the union's 32 bytes. The compile-time
// size assertions below catch a layout mistake without needing to run on
// Windows.
type mouseInput struct {
	inputType   uint32
	_           uint32
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type keyboardInput struct {
	inputType   uint32
	_           uint32
	virtualKey  uint16
	scanCode    uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
	_           uint64
}

func newMouseInput(absoluteX int32, absoluteY int32, flags uint32) mouseInput {
	return mouseInput{
		inputType: inputTypeMouse,
		dx:        absoluteX,
		dy:        absoluteY,
		dwFlags:   flags,
	}
}

func newUnicodeKeyboardInput(utf16CodeUnit uint16, flags uint32) keyboardInput {
	return keyboardInput{
		inputType: inputTypeKeyboard,
		scanCode:  utf16CodeUnit,
		dwFlags:   flags,
	}
}

func newVirtualKeyboardInput(virtualKeyCode uint16, extraFlags uint32) keyboardInput {
	return keyboardInput{
		inputType:  inputTypeKeyboard,
		virtualKey: virtualKeyCode,
		dwFlags:    extraFlags,
	}
}

const win32InputStructSize = 40

type mouseInputSizeFloorCheck [unsafe.Sizeof(mouseInput{}) - win32InputStructSize]struct{}
type mouseInputSizeCeilingCheck [win32InputStructSize - unsafe.Sizeof(mouseInput{})]struct{}
type keyboardInputSizeFloorCheck [unsafe.Sizeof(keyboardInput{}) - win32InputStructSize]struct{}
type keyboardInputSizeCeilingCheck [win32InputStructSize - unsafe.Sizeof(keyboardInput{})]struct{}
