package cli

import (
	"errors"
	"strings"
	"testing"
)

type recordingCommandRunner struct {
	commands   []string
	failOnPart string
}

func (runner *recordingCommandRunner) runResult(command string) (string, error) {
	runner.commands = append(runner.commands, command)
	if runner.failOnPart != "" && strings.Contains(command, runner.failOnPart) {
		return "refused", errors.New("command failed")
	}
	return "", nil
}

func hasCommandContaining(commands []string, part string) bool {
	for _, command := range commands {
		if strings.Contains(command, part) {
			return true
		}
	}
	return false
}

func TestDeliveringThePayloadNeverStopsTheGuest(t *testing.T) {
	runner := &recordingCommandRunner{}

	if errorValue := refreshBlueclawDeliveryOn(runner); errorValue != nil {
		t.Fatalf("expected the refresh to run, got %v", errorValue)
	}
	if !hasCommandContaining(runner.commands, "rsync -a --delete") {
		t.Fatalf("the share was never refreshed: %+v", runner.commands)
	}
	for _, forbidden := range []string{"systemctl stop", "sync-workspace"} {
		if hasCommandContaining(runner.commands, forbidden) {
			t.Fatalf("the share is a directory on the host, so %q has no reason to run: %+v", forbidden, runner.commands)
		}
	}
}

func TestADeliveryThatFailsIsReported(t *testing.T) {
	runner := &recordingCommandRunner{failOnPart: "rsync"}

	errorValue := refreshBlueclawDeliveryOn(runner)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "refused") {
		t.Fatalf("a share left half written must be reported, got %v", errorValue)
	}
}

type stoppingCommandRunner struct {
	commands      []string
	serviceStatus string
}

func (runner *stoppingCommandRunner) run(command string) string {
	runner.commands = append(runner.commands, command)
	if strings.Contains(command, "is-active") {
		return runner.serviceStatus
	}
	return ""
}

func (runner *stoppingCommandRunner) runResult(command string) (string, error) {
	runner.commands = append(runner.commands, command)
	return "", nil
}

func TestAnInterruptedRuntimeInstallStartsTheGuestItStopped(t *testing.T) {
	runner := &stoppingCommandRunner{serviceStatus: "inactive"}
	startBlueclawIfTheInstallLeftItStopped(runner, true)

	if !hasCommandContaining(runner.commands, "systemctl start blueclaw") {
		t.Fatalf("a 4GB transfer over a flaky uplink fails often, and it must not leave the guest down: %+v", runner.commands)
	}
}

func TestARuntimeInstallDoesNotStartAGuestThatWasAlreadyDown(t *testing.T) {
	runner := &stoppingCommandRunner{serviceStatus: "inactive"}
	startBlueclawIfTheInstallLeftItStopped(runner, false)

	if hasCommandContaining(runner.commands, "systemctl start blueclaw") {
		t.Fatalf("a guest the operator had stopped must stay stopped: %+v", runner.commands)
	}
}

func TestARuntimeInstallLeavesARunningGuestAlone(t *testing.T) {
	runner := &stoppingCommandRunner{serviceStatus: "active"}
	startBlueclawIfTheInstallLeftItStopped(runner, true)

	if hasCommandContaining(runner.commands, "systemctl start blueclaw") {
		t.Fatalf("the success path already started it: %+v", runner.commands)
	}
}

func TestEveryRuntimeArtifactIsAskedAboutBeforeItIsSent(t *testing.T) {
	presenceCommand := blueclawRuntimeFilePresenceCommand()

	for _, artifact := range requiredBlueclawRuntimeInstallArtifacts() {
		if !strings.Contains(presenceCommand, artifact.name+":"+artifact.remotePath) {
			t.Fatalf("an artifact the check never asks about is reported missing and re-sent on every run: %s", artifact.name)
		}
	}
}
