package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func requestsIn(t *testing.T, stateDirectoryPath string) int {
	t.Helper()
	entries, errorValue := os.ReadDir(stateDirectoryPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return len(entries)
}

func TestAskingForAResetLeavesTheRequestForTheBoxItRestarts(t *testing.T) {
	stateDirectoryPath := t.TempDir()
	restarts := 0

	errorValue := askTheBoxToReset(stateDirectoryPath, func() error {
		restarts++
		if requestsIn(t, stateDirectoryPath) != 1 {
			t.Fatal("the box was restarted before the reset request was written")
		}
		return nil
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if restarts != 1 || requestsIn(t, stateDirectoryPath) != 1 {
		t.Fatalf("restarts = %d, requests left = %d, want one restart and the request left for the box", restarts, requestsIn(t, stateDirectoryPath))
	}
}

func TestABoxThatCannotRestartIsLeftNoResetToFindLater(t *testing.T) {
	stateDirectoryPath := t.TempDir()

	errorValue := askTheBoxToReset(stateDirectoryPath, func() error {
		return errors.New("exit status 1: Failed to restart internkim-box.service: Unit internkim-box.service is masked.")
	})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "masked") {
		t.Fatalf("asking answered %v, want the reason the box did not restart", errorValue)
	}
	if requestsIn(t, stateDirectoryPath) != 0 {
		t.Fatal("a reset request stayed behind for whenever the box next starts")
	}
}
