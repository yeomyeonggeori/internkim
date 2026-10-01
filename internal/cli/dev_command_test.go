package cli

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
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
		"--scenario", "site_artifact_acceptance",
		"--seed", "42",
		"--temperature", "0.2",
	})
	if errorValue != nil {
		t.Fatalf("expected dev simulate to pass: %v", errorValue)
	}
	if invocation.ScenarioName != "site_artifact_acceptance" {
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

func TestDevPlaneHoldsTheLocalPlaneLock(t *testing.T) {
	command := holdingTheLocalPlane(
		"/repository",
		"/repository/internkim",
		[]string{"dev", "plane", "-t", "leaves on the messenger"},
	)
	expected := []string{
		"/repository/tools/with-local-plane",
		"/repository/internkim",
		"dev",
		"plane",
		"-t",
		"leaves on the messenger",
	}
	if !slices.Equal(command.Args, expected) {
		t.Fatalf("dev plane runs %v, expected %v", command.Args, expected)
	}
}
