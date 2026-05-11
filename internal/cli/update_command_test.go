package cli

import "testing"

type updateCommandCall struct {
	directoryPath string
	name          string
	arguments     []string
}

type testError string

func (errorValue testError) Error() string {
	return string(errorValue)
}

func TestUpdateArgumentsBuildsAndDeploysDefaultSlice(t *testing.T) {
	calls := captureUpdateCommandCalls(t)
	withUpdateExecutablePath(t, "./internkim")

	errorValue := runUpdateArguments([]string{"--host", "192.0.2.10"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls = %+v", *calls)
	}
	if (*calls)[0].name != "make" || !equalStrings((*calls)[0].arguments, []string{"build"}) {
		t.Fatalf("first call = %+v", (*calls)[0])
	}
	expectedSetupArguments := []string{"setup", "--only", "admin-web,binaries,services", "--force", "--host", "192.0.2.10"}
	if (*calls)[1].name != "./internkim" || !equalStrings((*calls)[1].arguments, expectedSetupArguments) {
		t.Fatalf("second call = %+v", (*calls)[1])
	}
}

func TestUpdatePlanSkipsBuildAndUsesWebSlice(t *testing.T) {
	calls := captureUpdateCommandCalls(t)
	withUpdateExecutablePath(t, "./internkim")

	errorValue := runUpdateArguments([]string{"--web", "--plan"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls = %+v", *calls)
	}
	expectedSetupArguments := []string{"setup", "--only", "admin-web", "--force", "--plan"}
	if (*calls)[0].name != "./internkim" || !equalStrings((*calls)[0].arguments, expectedSetupArguments) {
		t.Fatalf("call = %+v", (*calls)[0])
	}
}

func TestUpdateRejectsConflictingModeFlags(t *testing.T) {
	errorValue := runUpdateArguments([]string{"--web", "--binaries"})

	if errorValue == nil {
		t.Fatal("expected conflicting mode error")
	}
}

func TestUpdateSimFirstRunsGateBeforeDeploy(t *testing.T) {
	calls := captureUpdateCommandCalls(t)
	gateArguments := captureUpdateSimulationGate(t, nil)
	stopCalls := captureUpdateSimulationStop(t, nil)
	withUpdateExecutablePath(t, "./internkim")

	errorValue := runUpdateArguments([]string{"--sim-first", "--host", "192.0.2.10"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !equalStrings(*gateArguments, []string{}) {
		t.Fatalf("gate arguments = %+v", *gateArguments)
	}
	if *stopCalls != 1 {
		t.Fatalf("stop calls = %d", *stopCalls)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls = %+v", *calls)
	}
	expectedSetupArguments := []string{"setup", "--only", "admin-web,binaries,services", "--force", "--host", "192.0.2.10"}
	if (*calls)[0].name != "./internkim" || !equalStrings((*calls)[0].arguments, expectedSetupArguments) {
		t.Fatalf("call = %+v", (*calls)[0])
	}
}

func TestUpdateSimFirstStopsBeforeDeployWhenGateFails(t *testing.T) {
	calls := captureUpdateCommandCalls(t)
	captureUpdateSimulationGate(t, errUpdateSimulationGate)
	stopCalls := captureUpdateSimulationStop(t, nil)

	errorValue := runUpdateArguments([]string{"--sim-first"})

	if errorValue == nil {
		t.Fatal("expected gate error")
	}
	if *stopCalls != 0 {
		t.Fatalf("stop calls = %d", *stopCalls)
	}
	if len(*calls) != 0 {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestUpdateSimFirstPlanRunsPlansOnly(t *testing.T) {
	calls := captureUpdateCommandCalls(t)
	gateArguments := captureUpdateSimulationGate(t, nil)
	stopCalls := captureUpdateSimulationStop(t, nil)
	withUpdateExecutablePath(t, "./internkim")

	errorValue := runUpdateArguments([]string{"--sim-first", "--plan"})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !equalStrings(*gateArguments, []string{"--plan"}) {
		t.Fatalf("gate arguments = %+v", *gateArguments)
	}
	if *stopCalls != 0 {
		t.Fatalf("stop calls = %d", *stopCalls)
	}
	expectedSetupArguments := []string{"setup", "--only", "admin-web,binaries,services", "--force", "--plan"}
	if len(*calls) != 1 || !equalStrings((*calls)[0].arguments, expectedSetupArguments) {
		t.Fatalf("calls = %+v", *calls)
	}
}

func captureUpdateCommandCalls(t *testing.T) *[]updateCommandCall {
	t.Helper()
	previousRunner := runUpdateCommand
	calls := []updateCommandCall{}
	runUpdateCommand = func(directoryPath string, name string, arguments ...string) error {
		calls = append(calls, updateCommandCall{
			directoryPath: directoryPath,
			name:          name,
			arguments:     append([]string(nil), arguments...),
		})
		return nil
	}
	t.Cleanup(func() {
		runUpdateCommand = previousRunner
	})
	return &calls
}

var errUpdateSimulationGate = testError("sim gate failed")

func captureUpdateSimulationGate(t *testing.T, errorValue error) *[]string {
	t.Helper()
	previousGate := runUpdateSimulationGate
	var capturedArguments []string
	runUpdateSimulationGate = func(arguments []string) error {
		capturedArguments = append([]string(nil), arguments...)
		return errorValue
	}
	t.Cleanup(func() {
		runUpdateSimulationGate = previousGate
	})
	return &capturedArguments
}

func captureUpdateSimulationStop(t *testing.T, errorValue error) *int {
	t.Helper()
	previousStop := stopUpdateSimulation
	calls := 0
	stopUpdateSimulation = func() error {
		calls++
		return errorValue
	}
	t.Cleanup(func() {
		stopUpdateSimulation = previousStop
	})
	return &calls
}

func withUpdateExecutablePath(t *testing.T, executablePath string) {
	t.Helper()
	previousResolver := resolveUpdateExecutablePath
	resolveUpdateExecutablePath = func() (string, error) {
		return executablePath, nil
	}
	t.Cleanup(func() {
		resolveUpdateExecutablePath = previousResolver
	})
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
