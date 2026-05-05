package setup

import "testing"

func TestJetsonWiFiIsSatisfiedWhenRecoveryIsInstalled(t *testing.T) {
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardJetsonOrinNano,
		SSH:       fakeBoardConnection{output: "ready\n"},
	}
	if !StepWifi.IsSatisfied(context) {
		t.Fatalf("expected Jetson Wi-Fi step to be satisfied when recovery is installed")
	}
}

func TestJetsonWiFiIsNotSatisfiedWhenRecoveryIsMissing(t *testing.T) {
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardJetsonOrinNano,
		SSH:       fakeBoardConnection{output: "\n"},
	}
	if StepWifi.IsSatisfied(context) {
		t.Fatalf("expected Jetson Wi-Fi step to run when recovery is missing")
	}
}

func TestLegacyWiFiStillChecksWlanAddress(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     fakeBoardConnection{output: "192.168.0.2\n"},
	}
	if !StepWifi.IsSatisfied(context) {
		t.Fatalf("expected legacy Wi-Fi step to use wlan0 address")
	}
}

type fakeBoardConnection struct {
	output string
}

func (connection fakeBoardConnection) Run(command string) string {
	return connection.output
}

func (connection fakeBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}
