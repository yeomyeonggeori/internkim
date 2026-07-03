package browser

// scaleToAbsoluteAxis maps a screen coordinate into the 0-65535 normalized
// range the Windows SendInput MOUSEEVENTF_ABSOLUTE mode requires, given the
// origin and extent of the target virtual screen axis (GetSystemMetrics
// SM_XVIRTUALSCREEN/SM_CXVIRTUALSCREEN or the Y equivalents). extent must be
// at least 1; callers on multi-monitor mixed-DPI setups must first make sure
// value, origin, and extent are all expressed in the same pixel space (see
// extension_input_windows.go for why that is not automatic).
func scaleToAbsoluteAxis(value int, origin int, extent int) uint16 {
	if extent <= 1 {
		return 0
	}
	relativeValue := value - origin
	if relativeValue <= 0 {
		return 0
	}
	if relativeValue >= extent {
		return 65535
	}
	return uint16(relativeValue * 65535 / (extent - 1))
}
