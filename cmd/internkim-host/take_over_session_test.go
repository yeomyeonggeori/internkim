package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

func TestATakeOverStartsTheCompanyWithThisBoxSessionAndNotTheOneTheBackupCarried(t *testing.T) {
	directory := t.TempDir()
	hostKeyPath := filepath.Join(directory, "agent-key")
	relayKeyPath := filepath.Join(directory, "relay-agent-key")
	for _, path := range []string{hostKeyPath, relayKeyPath} {
		if errorValue := os.WriteFile(path, []byte("the-session-the-backup-carried\n"), 0o640); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	sessionPath := filepath.Join(directory, "backups", takeOverSessionFileName)
	if errorValue := keepTheTakeOverSession(sessionPath, "this-box-own-session"); errorValue != nil {
		t.Fatal(errorValue)
	}
	places := box.Places{CredentialPaths: []string{hostKeyPath, relayKeyPath}, StagedCredentialPaths: []string{filepath.Join(directory, "not-staged-yet")}}
	marked := ""

	errorValue := startingWithTheTakeOverSession(places, sessionPath, func(companyID string) error {
		marked = companyID
		return nil
	})(exampleCompanyID)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marked != exampleCompanyID {
		t.Fatalf("the box was marked for %q before its services started", marked)
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

func TestATakeOverThatCannotMarkTheBoxLeavesTheCredentialsAlone(t *testing.T) {
	directory := t.TempDir()
	hostKeyPath := filepath.Join(directory, "agent-key")
	if errorValue := os.WriteFile(hostKeyPath, []byte("the-session-the-backup-carried\n"), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	sessionPath := filepath.Join(directory, takeOverSessionFileName)
	if errorValue := keepTheTakeOverSession(sessionPath, "this-box-own-session"); errorValue != nil {
		t.Fatal(errorValue)
	}
	refused := errors.New("the marker could not be written")

	errorValue := startingWithTheTakeOverSession(box.Places{CredentialPaths: []string{hostKeyPath}}, sessionPath, func(string) error {
		return refused
	})(exampleCompanyID)

	if !errors.Is(errorValue, refused) {
		t.Fatalf("answered %v", errorValue)
	}
	document, errorValue := os.ReadFile(hostKeyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.TrimSpace(string(document)) != "the-session-the-backup-carried" {
		t.Fatalf("a take-over that stopped before starting the services rewrote the session to %q", strings.TrimSpace(string(document)))
	}
}
