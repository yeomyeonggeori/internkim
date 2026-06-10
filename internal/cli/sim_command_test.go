package cli

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/localfleet"
)

func TestSimGatePlanPrintsAliasWithoutRunningFleet(t *testing.T) {
	withIsolatedInternkimHome(t)
	requests := captureSimLocalFleet(t, nil)

	errorValue := runSimArguments([]string{"gate", "--plan"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*requests) != 0 {
		t.Fatalf("local fleet requests = %+v", *requests)
	}
}

func TestSimGateRunsPredeployRecipe(t *testing.T) {
	withIsolatedInternkimHome(t)
	requests := captureSimLocalFleet(t, nil)

	errorValue := runSimArguments([]string{"gate"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*requests) != 1 || (*requests)[0].Action != localfleet.ActionRunRecipe || (*requests)[0].Recipe != localfleet.DefaultRecipe {
		t.Fatalf("local fleet requests = %+v", *requests)
	}
}

func TestSimGateUsesLocalFleetInsteadOfLegacySimulationState(t *testing.T) {
	withIsolatedInternkimHome(t)
	commandCalls := captureSimCommandCalls(t)
	captureSimSetup(t, nil)
	requests := captureSimLocalFleet(t, nil)
	saveState(simulationStateDirectoryPath(), "fleet_id", "shared-fleet")
	saveState(physicalStateDirectoryPath(), "fleet_id", "shared-fleet")

	errorValue := runSimArguments([]string{"gate"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*commandCalls) != 0 {
		t.Fatalf("command calls = %+v", *commandCalls)
	}
	if len(*requests) != 1 {
		t.Fatalf("local fleet requests = %+v", *requests)
	}
}

func TestSimCleanupStopsVirtualMachineAndRemovesState(t *testing.T) {
	withIsolatedInternkimHome(t)
	captureSimCommandCalls(t)
	captureSimSetup(t, nil)
	requests := captureSimLocalFleet(t, nil)
	stateDir := simulationStateDirectoryPath()
	saveState(stateDir, "fleet_id", "sim-fleet")

	errorValue := runSimArguments([]string{"cleanup"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*requests) != 2 || (*requests)[0].Action != localfleet.ActionReset || (*requests)[1].Action != localfleet.ActionDown {
		t.Fatalf("local fleet requests = %+v", *requests)
	}
	if _, statError := os.Stat(stateDir); !os.IsNotExist(statError) {
		t.Fatalf("expected simulation state removal, got %v", statError)
	}
}

func captureSimLocalFleet(t *testing.T, errorValue error) *[]localfleet.JobRequest {
	t.Helper()
	previousRunner := runSimLocalFleet
	var requests []localfleet.JobRequest
	runSimLocalFleet = func(request localfleet.JobRequest) error {
		requests = append(requests, request)
		return errorValue
	}
	t.Cleanup(func() {
		runSimLocalFleet = previousRunner
	})
	return &requests
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
