package cli

import (
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
		"--record-cassette", "cassette.json",
		"--seed", "42",
		"--temperature", "0.2",
	})
	if errorValue != nil {
		t.Fatalf("expected dev simulate to pass: %v", errorValue)
	}
	if invocation.ScenarioName != "site_artifact_acceptance" {
		t.Fatalf("expected scenario to be forwarded, got %q", invocation.ScenarioName)
	}
	if invocation.RecordCassettePath != "cassette.json" {
		t.Fatalf("expected cassette path to be forwarded, got %q", invocation.RecordCassettePath)
	}
	if invocation.Seed != "42" || invocation.Temperature != "0.2" {
		t.Fatalf("expected generation options to be forwarded, got seed=%q temperature=%q", invocation.Seed, invocation.Temperature)
	}
}

func TestDevReplayCanTargetContainer(t *testing.T) {
	var invocation devVirtualSessionArguments
	previousRunner := runDevContainerVirtualSession
	runDevContainerVirtualSession = func(arguments devVirtualSessionArguments) error {
		invocation = arguments
		return nil
	}
	t.Cleanup(func() {
		runDevContainerVirtualSession = previousRunner
	})

	errorValue := runDevArguments([]string{
		"replay",
		"--target", "container",
		"--scenario", "slides",
		"--cassette", "cassette.json",
	})
	if errorValue != nil {
		t.Fatalf("expected dev replay to pass: %v", errorValue)
	}
	if invocation.TargetName != "container" {
		t.Fatalf("expected container target, got %q", invocation.TargetName)
	}
	if invocation.CassettePath != "cassette.json" {
		t.Fatalf("expected replay cassette, got %q", invocation.CassettePath)
	}
}

func TestDevReplayRejectsRemovedTartTarget(t *testing.T) {
	errorValue := runDevArguments([]string{
		"replay",
		"--target", "tart",
		"--scenario", "slides",
	})
	if errorValue == nil {
		t.Fatal("expected tart target to be rejected")
	}
	if !strings.Contains(errorValue.Error(), "without-mattermost") {
		t.Fatalf("expected guidance toward fleet Linux target, got %q", errorValue.Error())
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
		CassettePath:          "cassette.json",
		IsLiveLanguageModel:   true,
		Seed:                  "7",
	})
	expectedArguments := []string{
		"run", "./cmd/blueclaw-lab", "virtual-session",
		"--scenario", "slides",
		"--artifact-dir", "artifacts",
		"--cassette", "cassette.json",
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

func TestContainerDevVirtualSessionInvocationUsesBindMountedWorkspacePath(t *testing.T) {
	invocation, errorValue := containerDevVirtualSessionInvocation(devVirtualSessionArguments{
		ScenarioName: "attachment_material_read",
	})
	if errorValue != nil {
		t.Fatalf("expected invocation: %v", errorValue)
	}
	if invocation.WorkingDirectoryPath != "/mnt/shared/workspace/.dependency/blueclaw" {
		t.Fatalf("expected bind-mounted workspace path, got %q", invocation.WorkingDirectoryPath)
	}
}

func TestContainerDevSharedWorkspaceCommandChecksBindMountedDirectory(t *testing.T) {
	command := containerDevSharedWorkspaceCommand("/mnt/shared/workspace/.dependency/blueclaw")
	if !strings.Contains(command, "/mnt/shared/workspace/.dependency/blueclaw") {
		t.Fatalf("expected shared workspace command to reference the bind-mounted path, got %s", command)
	}
	if strings.Contains(command, "virtiofs") || strings.Contains(command, "mount") {
		t.Fatalf("expected shared workspace command to avoid mount logic, got %s", command)
	}
}
