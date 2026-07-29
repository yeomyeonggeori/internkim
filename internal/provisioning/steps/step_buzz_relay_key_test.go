package setup

import (
	"strings"
	"testing"
)

type buzzRelayKeyConnection struct{ deviceKey string }

func (connection buzzRelayKeyConnection) Run(command string) string {
	if strings.Contains(command, buzzRelayKeyDevicePath) {
		return connection.deviceKey
	}
	return ""
}

func (connection buzzRelayKeyConnection) SCP(localPath, remotePath string) error {
	return nil
}

func TestBuzzRelayKeyPreservesExistingDeviceKey(t *testing.T) {
	context := &Context{Backend: BackendSSH, SSH: buzzRelayKeyConnection{deviceKey: "BUZZ_RELAY_PRIVATE_KEY=abc123\n"}}
	if !StepBuzzRelayKey.IsSatisfied(context) {
		t.Fatal("an existing relay key must be preserved, never regenerated")
	}
}

func TestBuzzRelayKeyGeneratesWhenAbsent(t *testing.T) {
	context := &Context{Backend: BackendSSH, SSH: buzzRelayKeyConnection{deviceKey: ""}}
	if StepBuzzRelayKey.IsSatisfied(context) {
		t.Fatal("a missing relay key must require generation")
	}
}

func TestRandomRelayKeyHexIsThirtyTwoBytes(t *testing.T) {
	key, errorValue := randomRelayKeyHex()
	if errorValue != nil {
		t.Fatalf("key generation failed: %v", errorValue)
	}
	if len(key) != 64 {
		t.Fatalf("expected 64 hex chars (32 bytes), got %d", len(key))
	}
}
