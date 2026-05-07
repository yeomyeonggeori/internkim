package setup

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
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

func TestJetsonBinariesRequireLlamaServer(t *testing.T) {
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardJetsonOrinNano,
		SSH:       binaryPresenceBoardConnection{},
	}

	if StepBinaries.IsSatisfied(context) {
		t.Fatal("expected Jetson binaries to require llama-server")
	}
}

type binaryPresenceBoardConnection struct{}

func (connection binaryPresenceBoardConnection) Run(command string) string {
	if strings.Contains(command, locallm.BinaryPath()) {
		return "n"
	}
	if strings.Contains(command, "test -e ") {
		return "y"
	}
	return ""
}

func (connection binaryPresenceBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}
