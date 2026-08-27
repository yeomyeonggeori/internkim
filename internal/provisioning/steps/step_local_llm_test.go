package setup

import "testing"

func TestSimulationLocalLLMIsSatisfiedWithoutSSH(t *testing.T) {
	context := &Context{BoardType: BoardSimulation}

	if !localLLMIsSatisfied(context) {
		t.Fatal("expected simulation local LLM to be treated as satisfied")
	}
}

func TestLocalLLMStepInstallsTheRuntime(t *testing.T) {
	installed := false
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardJetsonOrinNano,
		SSH:       localLLMBoardConnection{},
	}
	context.Callbacks.InstallLocalLLMRuntimeSSH = func(*Context) error {
		installed = true
		return nil
	}

	_ = StepLocalLLM.Run(context)

	if !installed {
		t.Fatal("expected the local-llm step to install the local model runtime")
	}
}

func TestSimulationLocalLLMStepLeavesTheRuntimeAlone(t *testing.T) {
	installed := false
	context := &Context{
		Backend:   BackendSSH,
		BoardType: BoardSimulation,
		SSH:       localLLMBoardConnection{},
	}
	context.Callbacks.InstallLocalLLMRuntimeSSH = func(*Context) error {
		installed = true
		return nil
	}

	_ = StepLocalLLM.Run(context)

	if installed {
		t.Fatal("expected simulation to leave the local model runtime alone")
	}
}

type localLLMBoardConnection struct{}

func (connection localLLMBoardConnection) Run(command string) string {
	return ""
}

func (connection localLLMBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}
