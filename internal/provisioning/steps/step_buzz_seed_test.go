package setup

import (
	"strings"
	"testing"
)

type buzzSeedConnection struct{ deviceSeed string }

func (connection buzzSeedConnection) Run(command string) string {
	if strings.Contains(command, buzzKeySeedDevicePath) {
		return connection.deviceSeed
	}
	return ""
}

func (connection buzzSeedConnection) SCP(localPath, remotePath string) error {
	return nil
}

func TestBuzzSeedPreservesExistingDeviceSeed(t *testing.T) {
	context := &Context{Backend: BackendSSH, SSH: buzzSeedConnection{deviceSeed: "6f0dd8e2f83e\n"}}
	if !StepBuzzSeed.IsSatisfied(context) {
		t.Fatal("an existing device seed must be preserved, never overwritten")
	}
}

func TestBuzzSeedRequiresInstallWhenAbsent(t *testing.T) {
	context := &Context{Backend: BackendSSH, SSH: buzzSeedConnection{deviceSeed: ""}}
	if StepBuzzSeed.IsSatisfied(context) {
		t.Fatal("a missing device seed must require installation")
	}
}
