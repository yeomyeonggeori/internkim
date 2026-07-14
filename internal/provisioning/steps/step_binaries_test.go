package setup

import (
	"strings"
	"testing"
)

func TestSimulationBinariesSatisfiedWithoutLlamaServer(t *testing.T) {
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardSimulation,
		SSH:       binaryPresenceBoardConnection{},
	}

	if !StepBinaries.IsSatisfied(context) {
		t.Fatal("expected simulation binaries to ignore missing llama-server")
	}
}

func TestJetsonBinariesSatisfiedWithoutLlamaServer(t *testing.T) {
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardJetsonOrinNano,
		SSH:       binaryPresenceBoardConnection{},
	}

	if !StepBinaries.IsSatisfied(context) {
		t.Fatal("expected Jetson binaries to leave llama-server to local-llm")
	}
}

func TestBinariesRequireSDKD(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     binaryPresenceWithoutSDKDBoardConnection{},
	}

	if StepBinaries.IsSatisfied(context) {
		t.Fatal("expected missing SDKD binary to require installation")
	}
}

type binaryPresenceBoardConnection struct{}

type binaryPresenceWithoutSDKDBoardConnection struct{}

func (connection binaryPresenceWithoutSDKDBoardConnection) Run(command string) string {
	if strings.Contains(command, "'/usr/local/bin/blueclaw-sdkd'") {
		return ""
	}
	if strings.Contains(command, "test -e ") {
		return "y"
	}
	return ""
}

func (connection binaryPresenceWithoutSDKDBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

func (connection binaryPresenceBoardConnection) Run(command string) string {
	if strings.Contains(command, "test -e ") {
		return "y"
	}
	return ""
}

func (connection binaryPresenceBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}
