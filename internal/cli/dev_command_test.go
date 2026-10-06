package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/localfleet"
)

func TestDevSimulateBuildsLocalVirtualSessionCommand(t *testing.T) {
	var invocation devVirtualSessionArguments
	previousRunner := runDevLocalVirtualSession
	runDevLocalVirtualSession = func(arguments devVirtualSessionArguments) error {
		invocation = arguments
		return nil
	}
	t.Cleanup(func() {
		runDevLocalVirtualSession = previousRunner
	})

	errorValue := runDevArguments([]string{
		"simulate",
		"--scenario", "capability_question_acceptance",
		"--seed", "42",
		"--temperature", "0.2",
	})
	if errorValue != nil {
		t.Fatalf("expected dev simulate to pass: %v", errorValue)
	}
	if invocation.ScenarioName != "capability_question_acceptance" {
		t.Fatalf("expected scenario to be forwarded, got %q", invocation.ScenarioName)
	}
	if invocation.Seed != "42" || invocation.Temperature != "0.2" {
		t.Fatalf("expected generation options to be forwarded, got seed=%q temperature=%q", invocation.Seed, invocation.Temperature)
	}
}

func TestDevReplaySubcommandIsRemoved(t *testing.T) {
	errorValue := runDevArguments([]string{"replay"})
	if errorValue == nil {
		t.Fatal("expected replay subcommand to be rejected")
	}
	if !strings.Contains(errorValue.Error(), "unknown dev subcommand") {
		t.Fatalf("expected unknown subcommand error, got %q", errorValue.Error())
	}
}

func TestDevPlaneCarriesItsTestArgumentsToTheCompanyPlane(t *testing.T) {
	for _, arguments := range [][]string{{"-t", "the agent's directory"}, {"--", "-t", "the agent's directory"}} {
		options := devPlaneServiceOptions(arguments)
		if !options.IsEphemeral || !reflect.DeepEqual(options.ScenarioArguments, []string{"-t", "the agent's directory"}) {
			t.Fatalf("dev plane %q became %+v", arguments, options)
		}
	}
}

func TestDevFleetRunTakesNoTestArguments(t *testing.T) {
	if _, errorValue := parseDevFleetRunArguments([]string{"--scenario", "workspace-ownership", "--", "-t", "example"}); errorValue == nil {
		t.Fatal("dev fleet run accepted arguments no scenario reads")
	}
}

func TestParseDevFleetRunBuzzScenarioUsesDisposableFleet(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--keep",
		"--run-id", "dm-smoke",
		"--admin-port", "19080",
		"--scenario", "workspace-ownership",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if !configuration.ServiceOptions.IsEphemeral || configuration.ServiceOptions.RunID != "dm-smoke" {
		t.Fatalf("service options = %+v", configuration.ServiceOptions)
	}
	if configuration.ServiceOptions.AdminHostPort != 19080 {
		t.Fatalf("ports = %+v", configuration.ServiceOptions)
	}
	if configuration.ServiceOptions.ShouldUseRealModels {
		t.Fatalf("expected test models by default: %+v", configuration.ServiceOptions)
	}
	if configuration.Request.Action != "runScenario" || configuration.Request.Scenario != "workspace-ownership" {
		t.Fatalf("request = %+v", configuration.Request)
	}
	if !configuration.Request.KeepArtifacts {
		t.Fatalf("expected keep artifacts request: %+v", configuration.Request)
	}
}

func TestParseDevFleetRunCanUseRealModels(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--real",
		"--scenario", "workspace-ownership",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if !configuration.ServiceOptions.ShouldUseRealModels {
		t.Fatalf("expected real model option: %+v", configuration.ServiceOptions)
	}
}

func TestParseDevFleetRunCanReuseSharedFleet(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--reuse",
		"--scenario", "workspace-ownership",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if configuration.ServiceOptions.IsEphemeral {
		t.Fatalf("expected reusable service options: %+v", configuration.ServiceOptions)
	}
	if configuration.Request.Action != "runScenario" || configuration.Request.Scenario != "workspace-ownership" {
		t.Fatalf("request = %+v", configuration.Request)
	}
}

func TestParseDevFleetRunHelpListsScenariosFromTheRegistry(t *testing.T) {
	readEnd, writeEnd, errorValue := os.Pipe()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	previousStderr := os.Stderr
	os.Stderr = writeEnd
	_, parseError := parseDevFleetRunArguments([]string{"--help"})
	os.Stderr = previousStderr
	writeEnd.Close()
	if parseError == nil {
		t.Fatal("expected --help to report flag.ErrHelp")
	}
	helpOutput, errorValue := io.ReadAll(readEnd)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, scenarioName := range localfleet.ScenarioNames() {
		if !strings.Contains(string(helpOutput), scenarioName) {
			t.Fatalf("expected --help output to list scenario %q, got:\n%s", scenarioName, helpOutput)
		}
	}
}

func TestParseDevFleetRunRejectsConflictingFleetModes(t *testing.T) {
	_, errorValue := parseDevFleetRunArguments([]string{
		"--ephemeral",
		"--reuse",
		"--scenario", "workspace-ownership",
	})
	if errorValue == nil {
		t.Fatal("expected conflicting fleet modes to fail")
	}
	if !strings.Contains(errorValue.Error(), "--reuse") {
		t.Fatalf("expected reuse guidance, got %q", errorValue.Error())
	}
}

func TestParseDevFleetRunRejectsRunIDWithReusableFleet(t *testing.T) {
	_, errorValue := parseDevFleetRunArguments([]string{
		"--reuse",
		"--run-id", "debug",
		"--scenario", "workspace-ownership",
	})
	if errorValue == nil {
		t.Fatal("expected run id with reusable fleet to fail")
	}
	if !strings.Contains(errorValue.Error(), "--run-id") {
		t.Fatalf("expected run id guidance, got %q", errorValue.Error())
	}
}

func TestParseDevFleetRunVirtualSessionScenario(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--virtual-session",
		"--scenario", "dm_send_confirm_acceptance",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if !configuration.ServiceOptions.IsEphemeral {
		t.Fatalf("expected ephemeral service options: %+v", configuration.ServiceOptions)
	}
	if !configuration.Request.VirtualSession || configuration.Request.Scenario != "dm_send_confirm_acceptance" {
		t.Fatalf("request = %+v", configuration.Request)
	}
}

func TestDevVirtualSessionCommandArguments(t *testing.T) {
	arguments := devVirtualSessionCommandArguments(devVirtualSessionArguments{
		ScenarioName:          "slides",
		ScenarioFilePath:      "scenarios/slides.json",
		ArtifactDirectoryPath: "artifacts",
		MaximumModelTier:      "low",
		HasStrictAssertions:   true,
		IsLiveLanguageModel:   true,
		Seed:                  "7",
	})
	expectedArguments := []string{
		"run", "./cmd/blueclaw-lab", "virtual-session",
		"--scenario", "slides",
		"--scenario-file", "scenarios/slides.json",
		"--artifact-dir", "artifacts",
		"--maximum-model-tier", "low",
		"--strict-assertions",
		"--seed", "7",
		"--live-llm",
	}
	if !reflect.DeepEqual(arguments, expectedArguments) {
		t.Fatalf("unexpected command arguments:\n got: %#v\nwant: %#v", arguments, expectedArguments)
	}
}

func TestResolveDevPathUsesRepositoryRoot(t *testing.T) {
	repositoryRootPath := t.TempDir()
	if resolvedPath := resolveDevPath(repositoryRootPath, ".artifacts/blueclaw-dev"); resolvedPath != filepath.Join(repositoryRootPath, ".artifacts", "blueclaw-dev") {
		t.Fatalf("unexpected resolved artifact path %q", resolvedPath)
	}
	absolutePath := filepath.Join(t.TempDir(), "scenario.json")
	if resolvedPath := resolveDevPath(repositoryRootPath, absolutePath); resolvedPath != absolutePath {
		t.Fatalf("expected absolute path to remain unchanged, got %q", resolvedPath)
	}
	if resolvedPath := resolveDevPath(repositoryRootPath, " "); resolvedPath != "" {
		t.Fatalf("expected empty path to remain empty, got %q", resolvedPath)
	}
}

func TestParseDevVirtualSessionArgumentsForwardsStrictScenarioFile(t *testing.T) {
	arguments, errorValue := parseDevVirtualSessionArguments([]string{
		"--scenario-file", "scenarios/task-lifecycle.json",
		"--maximum-model-tier", "low",
		"--strict-assertions",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if arguments.ScenarioFilePath != "scenarios/task-lifecycle.json" ||
		arguments.MaximumModelTier != "low" ||
		!arguments.HasStrictAssertions {
		t.Fatalf("unexpected arguments: %+v", arguments)
	}
}

func TestDevVirtualSessionCommandArgumentsForwardsTheChosenProvider(t *testing.T) {
	arguments := devVirtualSessionCommandArguments(devVirtualSessionArguments{
		ScenarioName:          "plain_question_acceptance",
		ArtifactDirectoryPath: "artifacts",
		LanguageModelEndpoint: "http://capability",
		LanguageModelSocket:   "/tmp/capability.sock",
		LanguageModelProvider: "capability",
		IsLiveLanguageModel:   true,
	})

	for _, expectedArgument := range []string{"--llm-provider", "capability", "--llm-unix-socket", "/tmp/capability.sock"} {
		if !slices.Contains(arguments, expectedArgument) {
			t.Fatalf("expected %q in %#v", expectedArgument, arguments)
		}
	}
}

func TestDevVirtualSessionScriptedRunOmitsLiveGenerationFlags(t *testing.T) {
	arguments := devVirtualSessionCommandArguments(devVirtualSessionArguments{
		ScenarioName:          "failure_explanation_acceptance",
		ArtifactDirectoryPath: "artifacts",
		Seed:                  "7",
		Temperature:           "0.2",
	})
	for _, argument := range arguments {
		if argument == "--seed" || argument == "--temperature" || argument == "--live-llm" {
			t.Fatalf("scripted run must omit live generation flags, got %#v", arguments)
		}
	}
}

func TestDevFleetRunHoldsTheLocalPlaneLock(t *testing.T) {
	command := holdingTheLocalPlane(
		"/repository",
		"/repository/internkim",
		[]string{"dev", "fleet", "run", "--scenario", "workspace-ownership"},
	)

	expected := []string{
		"/repository/tools/with-local-plane",
		"/repository/internkim",
		"dev",
		"fleet",
		"run",
		"--scenario",
		"workspace-ownership",
	}
	if !slices.Equal(command.Args, expected) {
		t.Fatalf("dev fleet run runs %v, expected %v", command.Args, expected)
	}
}

func TestDevFleetRunAnswersWithoutTouchingTheLocalPlane(t *testing.T) {
	repositoryRoot := t.TempDir()
	touchedPath := filepath.Join(repositoryRoot, "plane-touched")
	toolsPath := filepath.Join(repositoryRoot, "tools")
	if errorValue := os.MkdirAll(toolsPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	script := "#!/bin/sh\ntouch " + touchedPath + "\n"
	if errorValue := os.WriteFile(filepath.Join(toolsPath, "with-local-plane"), []byte(script), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Chdir(repositoryRoot)
	t.Setenv("LOCAL_PLANE_LOCK_HOLDER", "")

	for name, arguments := range map[string][]string{
		"help":            {"--help"},
		"missing":         {},
		"unknown flag":    {"--no-such-flag"},
		"extra arguments": {"--scenario", "workspace-ownership", "stray"},
	} {
		t.Run(name, func(t *testing.T) {
			readEnd, writeEnd, errorValue := os.Pipe()
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			previousStderr := os.Stderr
			os.Stderr = writeEnd
			runDevFleetArguments(append([]string{"run"}, arguments...))
			os.Stderr = previousStderr
			writeEnd.Close()
			io.Copy(io.Discard, readEnd)
			if _, statError := os.Stat(touchedPath); statError == nil {
				t.Fatalf("dev fleet run %v started the local plane before answering", arguments)
			}
		})
	}
}
