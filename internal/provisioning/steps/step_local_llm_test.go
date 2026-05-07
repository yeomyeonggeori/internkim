package setup

import "testing"

func TestSimulationLocalLLMIsSatisfiedWithoutSSH(t *testing.T) {
	context := &Context{BoardType: BoardSimulation}

	if !localLLMIsSatisfied(context) {
		t.Fatal("expected simulation local LLM to be treated as satisfied")
	}
}
