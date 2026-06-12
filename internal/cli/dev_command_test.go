package cli

import (
	"reflect"
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
		"--scenario", "site_prototype_acceptance",
		"--record-cassette", "cassette.json",
		"--seed", "42",
		"--temperature", "0.2",
	})
	if errorValue != nil {
		t.Fatalf("expected dev simulate to pass: %v", errorValue)
	}
	if invocation.ScenarioName != "site_prototype_acceptance" {
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
	if !strings.Contains(errorValue.Error(), "--target container") {
		t.Fatalf("expected guidance toward container target, got %q", errorValue.Error())
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
