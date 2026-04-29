package setup

import "testing"

func TestJetsonWiFiIsSatisfiedOverSSH(t *testing.T) {
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardJetsonOrinNano,
	}
	if !StepWifi.IsSatisfied(context) {
		t.Fatalf("expected Jetson Wi-Fi step to be satisfied when setup is already connected over SSH")
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
