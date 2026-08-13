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

func TestWorkspaceSyncStartsBlueclawWhateverHappens(t *testing.T) {
	testCases := []struct {
		name       string
		failOnPart string
	}{
		{"a sync that works", ""},
		{"a sync that fails", "sync-workspace"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &recordingCommandRunner{failOnPart: testCase.failOnPart}

			errorValue := syncBlueclawWorkspaceImageOn(runner)

			if testCase.failOnPart != "" && errorValue == nil {
				t.Fatal("a failing sync must be reported")
			}
			if !hasCommandContaining(runner.commands, "sync-workspace") {
				t.Fatalf("the sync never ran: %+v", runner.commands)
			}
			startCommand := blueclawStartAfterPayloadSyncCommand()
			if !hasCommandContaining(runner.commands, startCommand) {
				t.Fatalf("blueclaw was stopped for the sync and never started again: %+v", runner.commands)
			}
		})
	}
}

func TestWorkspaceSyncReportsAStartThatFails(t *testing.T) {
	runner := &recordingCommandRunner{failOnPart: blueclawStartAfterPayloadSyncCommand()}

	errorValue := syncBlueclawWorkspaceImageOn(runner)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "start blueclaw after workspace sync") {
		t.Fatalf("a blueclaw that does not come back must be reported, got %v", errorValue)
	}
}
