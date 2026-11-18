package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

func thisBoxOwnSession() (string, error) {
	return "this-box-own-session", nil
}

func TestATakeOverStartsTheCompanyWithThisBoxSessionAndNotTheOneTheBackupCarried(t *testing.T) {
	directory := t.TempDir()
	hostKeyPath := filepath.Join(directory, "agent-key")
	relayKeyPath := filepath.Join(directory, "relay-agent-key")
	for _, path := range []string{hostKeyPath, relayKeyPath} {
		if errorValue := os.WriteFile(path, []byte("the-session-the-backup-carried\n"), 0o640); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	places := box.Places{CredentialPaths: []string{hostKeyPath, relayKeyPath}, StagedCredentialPaths: []string{filepath.Join(directory, "not-staged-yet")}}

	errorValue := startingWithTheTakeOverSession(places, thisBoxOwnSession)(exampleCompanyID)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, path := range []string{hostKeyPath, relayKeyPath} {
		document, errorValue := os.ReadFile(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.TrimSpace(string(document)) != "this-box-own-session" {
			t.Fatalf("%s holds %q when the services start, a session that stops working an hour after the backup", path, strings.TrimSpace(string(document)))
		}
	}
}
