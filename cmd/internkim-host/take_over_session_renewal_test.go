package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

type sessionsAsked struct {
	mutex sync.Mutex
	asked int
}

func (sessions *sessionsAsked) ask(context.Context) (box.Session, error) {
	sessions.mutex.Lock()
	defer sessions.mutex.Unlock()
	sessions.asked++
	return box.Session{
		Role:     box.RoleService,
		Session:  box.HostSession{AccessToken: fmt.Sprintf("session-%d", sessions.asked), ExpiresAt: time.Now().Add(time.Hour).Unix()},
		TakeOver: box.TakeOverPolicy{IsAutomatic: true, LapseSeconds: 90},
	}, nil
}

func heldSession(t *testing.T, path string) string {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.TrimSpace(string(document))
}

func TestATakeOverThatOutlivesItsFirstSessionStartsTheCompanyWithOneAskedForAtTheEnd(t *testing.T) {
	directory := t.TempDir()
	hostKeyPath := filepath.Join(directory, "agent-key")
	if errorValue := os.WriteFile(hostKeyPath, []byte("the-session-the-backup-carried\n"), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	sessionPath := filepath.Join(directory, "backups", takeOverSessionFileName)
	if errorValue := keepTheTakeOverSession(sessionPath, "the-session-that-ran-out-while-restoring"); errorValue != nil {
		t.Fatal(errorValue)
	}
	sessions := &sessionsAsked{}
	session := takeOverSession{Path: sessionPath, Ask: sessions.ask}

	errorValue := startingWithTheTakeOverSession(box.Places{CredentialPaths: []string{hostKeyPath}}, session.fresh)(exampleCompanyID)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if held := heldSession(t, hostKeyPath); held != "session-1" {
		t.Fatalf("the services start with %q, a session the central plane refuses once the restore has run for an hour", held)
	}
	if held := heldSession(t, sessionPath); held != "session-1" {
		t.Fatalf("the take-over still reads the company's store with %q", held)
	}
}

func TestATakeOverReplacesTheSessionItWasHandedWhileItRestores(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), takeOverSessionFileName)
	if errorValue := keepTheTakeOverSession(sessionPath, "the-session-the-box-handed-over"); errorValue != nil {
		t.Fatal(errorValue)
	}
	sessions := &sessionsAsked{}
	renewing, stop := context.WithCancel(context.Background())
	defer stop()

	go takeOverSession{Path: sessionPath, Ask: sessions.ask}.keepFresh(renewing)

	deadline := time.Now().Add(5 * time.Second)
	for heldSession(t, sessionPath) != "session-1" {
		if time.Now().After(deadline) {
			t.Fatalf("the take-over kept reading the store with %q", heldSession(t, sessionPath))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestATakeOverChecksInWithinTheLeaseSoTheCompanyIsNotHandedToAnotherStandbyWhileItRestores(t *testing.T) {
	sessions := &sessionsAsked{}
	renewed, errorValue := sessions.ask(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	wait := untilTheNextRenewal(renewed)

	if wait > 45*time.Second/3 {
		t.Fatalf("a take-over whose session lasts an hour checks in again after %s; the record lets another standby take over once the box goes unseen for 90s", wait)
	}
}

func TestATakeOverRefusedAFreshSessionLeavesTheCredentialsAlone(t *testing.T) {
	hostKeyPath := filepath.Join(t.TempDir(), "agent-key")
	if errorValue := os.WriteFile(hostKeyPath, []byte("the-session-the-backup-carried\n"), 0o640); errorValue != nil {
		t.Fatal(errorValue)
	}
	refused := errors.New("no company holds this box any longer")

	errorValue := startingWithTheTakeOverSession(box.Places{CredentialPaths: []string{hostKeyPath}}, func() (string, error) {
		return "", refused
	})(exampleCompanyID)

	if !errors.Is(errorValue, refused) {
		t.Fatalf("answered %v", errorValue)
	}
	if held := heldSession(t, hostKeyPath); held != "the-session-the-backup-carried" {
		t.Fatalf("a refused session rewrote the credential to %q", held)
	}
}
