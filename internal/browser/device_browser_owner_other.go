//go:build !linux

package browser

import "syscall"

type deviceBrowserOwner struct{}

func deviceBrowserOwnerOf(string) (*deviceBrowserOwner, error) {
	return nil, nil
}

func (owner *deviceBrowserOwner) own(string) error {
	return nil
}

func deviceBrowserProcessAttributes(*deviceBrowserOwner) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
