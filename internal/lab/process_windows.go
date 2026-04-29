//go:build windows

package lab

import "os/exec"

func detachCommand(command *exec.Cmd) {
	_ = command
}
