package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
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

func TestParseDevFleetRunDefaultsToDisposablePredeploy(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments(nil)
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if !configuration.ServiceOptions.IsEphemeral {
		t.Fatalf("expected disposable service options: %+v", configuration.ServiceOptions)
	}
	if configuration.Request.Action != "runRecipe" || configuration.Request.Recipe != "predeploy-gate" {
		t.Fatalf("request = %+v", configuration.Request)
	}
}

func TestParseDevFleetRunMattermostScenarioUsesDisposableFleet(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--keep",
		"--run-id", "dm-smoke",
		"--admin-port", "19080",
		"--mattermost-port", "19065",
		"--scenario", "mattermost-direct-message-send",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if !configuration.ServiceOptions.IsEphemeral || configuration.ServiceOptions.RunID != "dm-smoke" {
		t.Fatalf("service options = %+v", configuration.ServiceOptions)
	}
	if configuration.ServiceOptions.AdminHostPort != 19080 || configuration.ServiceOptions.MattermostHostPort != 19065 {
		t.Fatalf("ports = %+v", configuration.ServiceOptions)
	}
	if configuration.ServiceOptions.ShouldUseRealModels {
		t.Fatalf("expected test models by default: %+v", configuration.ServiceOptions)
	}
	if configuration.Request.Action != "runScenario" || configuration.Request.Scenario != "mattermost-direct-message-send" {
		t.Fatalf("request = %+v", configuration.Request)
	}
	if !configuration.Request.KeepArtifacts {
		t.Fatalf("expected keep artifacts request: %+v", configuration.Request)
	}
}

func TestParseDevFleetRunCanUseRealModels(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--real",
		"--scenario", "mattermost-direct-message-send",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if !configuration.ServiceOptions.ShouldUseRealModels {
		t.Fatalf("expected real model option: %+v", configuration.ServiceOptions)
	}
}

func TestDevFleetReprovisionPreservesModelRuntime(t *testing.T) {
	environment := devFleetReprovisionEnvironment(nil, "")
	expectedValues := []string{
		"INTERNKIM_TEST_MODEL_TIER=low",
		blueclaw.BlueclawTestMaximumModelTierEnvironment + "=low",
		blueclaw.BlueclawSDKDModeEnvironment + "=authoritative",
	}
	for _, expectedValue := range expectedValues {
		if !slices.Contains(environment, expectedValue) {
			t.Fatalf("expected %q in %#v", expectedValue, environment)
		}
	}
}

func TestLatestLocalFleetConfigurationPathPrefersCanonicalConfiguration(t *testing.T) {
	repositoryRootPath := t.TempDir()
	canonicalPath := filepath.Join(repositoryRootPath, ".local", "local-fleet", "config.json")
	runPath := filepath.Join(repositoryRootPath, ".local", "local-fleet", "runs", "stale", "config.json")
	if errorValue := os.MkdirAll(filepath.Dir(runPath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, path := range []string{canonicalPath, runPath} {
		if errorValue := os.WriteFile(path, []byte("{}"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	configurationPath, errorValue := latestLocalFleetConfigurationPath(repositoryRootPath)
	if errorValue != nil {
		t.Fatalf("expected canonical configuration: %v", errorValue)
	}
	if configurationPath != canonicalPath {
		t.Fatalf("configuration path = %q, want %q", configurationPath, canonicalPath)
	}
}

func TestLatestLocalFleetConfigurationPathFallsBackToLatestRun(t *testing.T) {
	repositoryRootPath := t.TempDir()
	oldRunPath := filepath.Join(repositoryRootPath, ".local", "local-fleet", "runs", "old", "config.json")
	latestRunPath := filepath.Join(repositoryRootPath, ".local", "local-fleet", "runs", "latest", "config.json")
	for _, path := range []string{oldRunPath, latestRunPath} {
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := os.WriteFile(path, []byte("{}"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	oldModificationTime := time.Now().Add(-time.Hour)
	if errorValue := os.Chtimes(oldRunPath, oldModificationTime, oldModificationTime); errorValue != nil {
		t.Fatal(errorValue)
	}

	configurationPath, errorValue := latestLocalFleetConfigurationPath(repositoryRootPath)
	if errorValue != nil {
		t.Fatalf("expected run configuration: %v", errorValue)
	}
	if configurationPath != latestRunPath {
		t.Fatalf("configuration path = %q, want %q", configurationPath, latestRunPath)
	}
}

func TestParseDevFleetRunCanReuseSharedFleet(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--reuse",
		"--scenario", "mattermost-direct-message-send",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if configuration.ServiceOptions.IsEphemeral {
		t.Fatalf("expected reusable service options: %+v", configuration.ServiceOptions)
	}
	if configuration.Request.Action != "runScenario" || configuration.Request.Scenario != "mattermost-direct-message-send" {
		t.Fatalf("request = %+v", configuration.Request)
	}
}

func TestParseDevFleetRunRejectsConflictingFleetModes(t *testing.T) {
	_, errorValue := parseDevFleetRunArguments([]string{
		"--ephemeral",
		"--reuse",
		"--scenario", "mattermost-direct-message-send",
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
		"--scenario", "mattermost-direct-message-send",
	})
	if errorValue == nil {
		t.Fatal("expected run id with reusable fleet to fail")
	}
	if !strings.Contains(errorValue.Error(), "--run-id") {
		t.Fatalf("expected run id guidance, got %q", errorValue.Error())
	}
}

func TestParseDevFleetRunWithoutMattermostRequiresScenario(t *testing.T) {
	_, errorValue := parseDevFleetRunArguments([]string{
		"--without-mattermost",
	})
	if errorValue == nil {
		t.Fatal("expected without-mattermost recipe to fail")
	}
	if !strings.Contains(errorValue.Error(), "--scenario") {
		t.Fatalf("expected scenario guidance, got %q", errorValue.Error())
	}
}

func TestParseDevFleetRunWithoutMattermostScenario(t *testing.T) {
	configuration, errorValue := parseDevFleetRunArguments([]string{
		"--without-mattermost",
		"--scenario", "dm_send_confirm_acceptance",
	})
	if errorValue != nil {
		t.Fatalf("expected parse to pass: %v", errorValue)
	}
	if !configuration.ServiceOptions.IsEphemeral {
		t.Fatalf("expected ephemeral service options: %+v", configuration.ServiceOptions)
	}
	if !configuration.Request.WithoutMattermost || configuration.Request.Scenario != "dm_send_confirm_acceptance" {
		t.Fatalf("request = %+v", configuration.Request)
	}
}

func TestDevVirtualSessionCommandArguments(t *testing.T) {
	arguments := devVirtualSessionCommandArguments(devVirtualSessionArguments{
		ScenarioName:          "slides",
		ArtifactDirectoryPath: "artifacts",
		IsLiveLanguageModel:   true,
		Seed:                  "7",
	})
	expectedArguments := []string{
		"run", "./cmd/blueclaw-lab", "virtual-session",
		"--scenario", "slides",
		"--artifact-dir", "artifacts",
		"--seed", "7",
		"--live-llm",
	}
	if !reflect.DeepEqual(arguments, expectedArguments) {
		t.Fatalf("unexpected command arguments:\n got: %#v\nwant: %#v", arguments, expectedArguments)
	}
}

func TestDevVirtualSessionCommandArgumentsForwardsSDKDProvider(t *testing.T) {
	arguments := devVirtualSessionCommandArguments(devVirtualSessionArguments{
		ScenarioName:             "plain_question_acceptance",
		ArtifactDirectoryPath:    "artifacts",
		LanguageModelEndpoint:    "http://sdkd",
		LanguageModelSocket:      "/tmp/sdkd.sock",
		LanguageModelProvider:    "sdkd",
		LanguageModelAuthKeyPath: "/tmp/sdkd.key",
		IsLiveLanguageModel:      true,
	})

	for _, expectedArgument := range []string{"--llm-provider", "sdkd", "--llm-auth-key-path", "/tmp/sdkd.key"} {
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
