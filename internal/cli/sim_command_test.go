package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimGatePlanDoesNotBuildOrStartVirtualMachine(t *testing.T) {
	withIsolatedInternkimHome(t)
	commandCalls := captureSimCommandCalls(t)
	setupArguments := captureSimSetup(t, nil)
	captureSimLabTarget(t, nil)

	errorValue := runSimArguments([]string{"gate", "--plan"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*commandCalls) != 0 {
		t.Fatalf("command calls = %+v", *commandCalls)
	}
	if !equalStrings(*setupArguments, []string{"--plan", "--force", "--verify-browser"}) {
		t.Fatalf("setup arguments = %+v", *setupArguments)
	}
}

func TestSimGateBuildsBeforeFullVerification(t *testing.T) {
	withIsolatedInternkimHome(t)
	commandCalls := captureSimCommandCalls(t)
	setupArguments := captureSimSetup(t, nil)
	captureSimLabTarget(t, nil)

	errorValue := runSimArguments([]string{"gate"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*commandCalls) != 1 || (*commandCalls)[0].name != "make" || !equalStrings((*commandCalls)[0].arguments, []string{"build"}) {
		t.Fatalf("command calls = %+v", *commandCalls)
	}
	if !equalStrings(*setupArguments, []string{"--force", "--verify-browser"}) {
		t.Fatalf("setup arguments = %+v", *setupArguments)
	}
}

func TestSimGateRejectsSharedPhysicalState(t *testing.T) {
	withIsolatedInternkimHome(t)
	commandCalls := captureSimCommandCalls(t)
	captureSimSetup(t, nil)
	captureSimLabTarget(t, nil)
	saveState(simulationStateDirectoryPath(), "fleet_id", "shared-fleet")
	saveState(physicalStateDirectoryPath(), "fleet_id", "shared-fleet")

	errorValue := runSimArguments([]string{"gate"})

	if errorValue == nil {
		t.Fatal("expected isolation error")
	}
	if !strings.Contains(errorValue.Error(), "shares fleet_id") {
		t.Fatalf("error = %v", errorValue)
	}
	if len(*commandCalls) != 0 {
		t.Fatalf("command calls = %+v", *commandCalls)
	}
}

func TestSimCleanupStopsVirtualMachineAndRemovesState(t *testing.T) {
	withIsolatedInternkimHome(t)
	captureSimCommandCalls(t)
	captureSimSetup(t, nil)
	labArguments := captureSimLabTarget(t, nil)
	stateDir := simulationStateDirectoryPath()
	saveState(stateDir, "fleet_id", "sim-fleet")

	errorValue := runSimArguments([]string{"cleanup"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !equalStrings(*labArguments, []string{"vm-down"}) {
		t.Fatalf("lab arguments = %+v", *labArguments)
	}
	if _, statError := os.Stat(stateDir); !os.IsNotExist(statError) {
		t.Fatalf("expected simulation state removal, got %v", statError)
	}
}

func TestSimulationHostTargetResolvesTartHostAndCredentials(t *testing.T) {
	withIsolatedInternkimHome(t)
	previousResolver := resolveSimulationVirtualMachineIPAddress
	resolveSimulationVirtualMachineIPAddress = func() string {
		return "192.0.2.44"
	}
	t.Cleanup(func() {
		resolveSimulationVirtualMachineIPAddress = previousResolver
	})
	repositoryRootPath := t.TempDir()
	labDirectoryPath := filepath.Join(repositoryRootPath, "lab")
	if errorValue := os.MkdirAll(labDirectoryPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	configurationPath := filepath.Join(labDirectoryPath, "config.example.json")
	configurationDocument := `{"vm":{"sshUsername":"simadmin","sshPassword":"simpass"}}`
	if errorValue := os.WriteFile(configurationPath, []byte(configurationDocument), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	target := simulationHostTarget(repositoryRootPath, resolveCommandTarget([]string{"--sim"}))

	if target.host != "192.0.2.44" || target.sshUser != "simadmin" || target.sshPassword != "simpass" {
		t.Fatalf("target = %+v", target)
	}
}

func captureSimCommandCalls(t *testing.T) *[]updateCommandCall {
	t.Helper()
	previousRunner := runSimCommand
	calls := []updateCommandCall{}
	runSimCommand = func(directoryPath string, name string, arguments ...string) error {
		calls = append(calls, updateCommandCall{
			directoryPath: directoryPath,
			name:          name,
			arguments:     append([]string(nil), arguments...),
		})
		return nil
	}
	t.Cleanup(func() {
		runSimCommand = previousRunner
	})
	return &calls
}

func captureSimSetup(t *testing.T, errorValue error) *[]string {
	t.Helper()
	previousSetup := runSimSetup
	var capturedArguments []string
	runSimSetup = func(arguments []string) error {
		capturedArguments = append([]string(nil), arguments...)
		return errorValue
	}
	t.Cleanup(func() {
		runSimSetup = previousSetup
	})
	return &capturedArguments
}

func captureSimLabTarget(t *testing.T, errorValue error) *[]string {
	t.Helper()
	previousLabTarget := runSimLabTarget
	var capturedArguments []string
	runSimLabTarget = func(arguments []string, boardType string) error {
		capturedArguments = append([]string(nil), arguments...)
		if boardType != commandTargetBoardSimulation {
			t.Fatalf("board type = %s", boardType)
		}
		return errorValue
	}
	t.Cleanup(func() {
		runSimLabTarget = previousLabTarget
	})
	return &capturedArguments
}

func withIsolatedInternkimHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}
