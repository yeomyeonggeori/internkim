//go:build !darwin

package machost

import "errors"

var errorSealingNeedsDarwin = errors.New("the delivery share is sealed with macOS file flags and this host has none")

func SealDeliveryDirectory(layout Layout) error {
	_ = layout
	return errorSealingNeedsDarwin
}

func UnsealDeliveryDirectory(layout Layout) error {
	_ = layout
	return nil
}
