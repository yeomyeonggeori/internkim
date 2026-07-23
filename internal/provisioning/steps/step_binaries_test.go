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

func TestBinariesRequireLLMD(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     binaryPresenceWithoutLLMDBoardConnection{},
	}

	if StepBinaries.IsSatisfied(context) {
		t.Fatal("expected missing LLMD binary to require installation")
	}
}

type binaryPresenceBoardConnection struct{}

type binaryPresenceWithoutLLMDBoardConnection struct{}

func (connection binaryPresenceWithoutLLMDBoardConnection) Run(command string) string {
	if strings.Contains(command, "'/usr/local/bin/blueclaw-llmd'") {
		return ""
	}
	if strings.Contains(command, "test -e ") {
		return "y"
	}
	return ""
}

func (connection binaryPresenceWithoutLLMDBoardConnection) SCP(localPath, remotePath string) error {
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
