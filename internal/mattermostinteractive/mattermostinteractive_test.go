package mattermostinteractive

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestActionBuilderAddsSharedEndpointAndToken(t *testing.T) {
	builder := ActionBuilder{
		URL:   ActionURL(" https://flow.example.com/ "),
		Token: "shared-token",
	}

	button := builder.Button("attendanceClockIn", "출근", "", "primary", Context{})
	if button.Integration.URL != "https://flow.example.com/_internkim/mattermost/actions" {
		t.Fatalf("button URL = %q", button.Integration.URL)
	}
	if button.Integration.Context.Action != "attendanceClockIn" || button.Integration.Context.Token != "shared-token" {
		t.Fatalf("button context = %+v", button.Integration.Context)
	}

	selection := builder.Select("askChoice", "선택", Context{Action: "ask.choice"}, []Option{{Text: "A", Value: "a"}})
	if selection.Integration.Context.Action != "ask.choice" || selection.Integration.Context.Token != "shared-token" {
		t.Fatalf("selection context = %+v", selection.Integration.Context)
	}
}

func TestLoadOrCreateTokenConvergesAcrossConcurrentCallers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "mattermost-interactive-token")
	tokens := make(chan string, 64)
	errors := make(chan error, 64)
	waitGroup := sync.WaitGroup{}
	for index := 0; index < 64; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			token, errorValue := LoadOrCreateToken(path, 32)
			tokens <- token
			errors <- errorValue
		}()
	}
	waitGroup.Wait()
	close(tokens)
	close(errors)

	for errorValue := range errors {
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	storedToken := readToken(path)
	if storedToken == "" {
		t.Fatal("expected a persisted token")
	}
	for token := range tokens {
		if token != storedToken {
			t.Fatalf("concurrent caller returned %q, stored token is %q", token, storedToken)
		}
	}
	fileInformation, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if fileInformation.Mode().Perm() != 0o600 {
		t.Fatalf("token permissions = %o", fileInformation.Mode().Perm())
	}
}
