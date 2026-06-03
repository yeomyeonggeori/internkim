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

func TestDevReplayCanTargetTart(t *testing.T) {
	var invocation devVirtualSessionArguments
	previousRunner := runDevTartVirtualSession
	runDevTartVirtualSession = func(arguments devVirtualSessionArguments) error {
		invocation = arguments
		return nil
	}
	t.Cleanup(func() {
		runDevTartVirtualSession = previousRunner
	})

	errorValue := runDevArguments([]string{
		"replay",
		"--target", "tart",
		"--scenario", "slides",
		"--cassette", "cassette.json",
	})
	if errorValue != nil {
		t.Fatalf("expected dev replay to pass: %v", errorValue)
	}
	if invocation.TargetName != "tart" {
		t.Fatalf("expected tart target, got %q", invocation.TargetName)
	}
	if invocation.CassettePath != "cassette.json" {
		t.Fatalf("expected replay cassette, got %q", invocation.CassettePath)
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

func TestTartDevVirtualSessionInvocationUsesNamedWorkspaceSharePath(t *testing.T) {
	invocation, errorValue := tartDevVirtualSessionInvocation(devVirtualSessionArguments{
		ScenarioName: "attachment_material_read",
	})
	if errorValue != nil {
		t.Fatalf("expected invocation: %v", errorValue)
	}
	if invocation.WorkingDirectoryPath != "/mnt/shared/workspace/workspace/.dependency/blueclaw" {
		t.Fatalf("expected named workspace share path, got %q", invocation.WorkingDirectoryPath)
	}
}

func TestTartDevSharedWorkspaceCommandMountsAutomountTag(t *testing.T) {
	command := tartDevSharedWorkspaceCommand("/mnt/shared/workspace/workspace/.dependency/blueclaw", "lab-password")
	for _, expectedFragment := range []string{
		"com.apple.virtio-fs.automount",
		"/mnt/shared/workspace",
		"/mnt/shared/workspace/workspace/.dependency/blueclaw",
		"lab-password",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected shared workspace command to contain %q, got %s", expectedFragment, command)
		}
	}
}
