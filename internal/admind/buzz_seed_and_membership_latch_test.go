package admind

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuzzKeySeedWrittenAfterTheFirstReadIsPickedUp(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), "buzz-key-seed")
	service := &Service{Configuration: Configuration{BuzzKeySeedPath: seedPath}}
	if seed := service.buzzKeySeed(); seed != "" {
		t.Fatalf("seed before the file exists = %q, want empty", seed)
	}
	if errorValue := os.WriteFile(seedPath, []byte("late-seed\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if seed := service.buzzKeySeed(); seed != "late-seed" {
		t.Fatalf("seed after the file appears = %q, want late-seed", seed)
	}
}

func TestBuzzKeySeedIsReadOnceItHasBeenRead(t *testing.T) {
	seedPath := writeTestFile(t, "first-seed")
	service := &Service{Configuration: Configuration{BuzzKeySeedPath: seedPath}}
	service.buzzKeySeed()
	if errorValue := os.Remove(seedPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	if seed := service.buzzKeySeed(); seed != "first-seed" {
		t.Fatalf("seed after the file is gone = %q, want first-seed", seed)
	}
}

func TestMembershipPassesWaitForARelayTheyCannotYetWriteTo(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), "buzz-key-seed")
	service := &Service{Configuration: Configuration{
		BuzzKeySeedPath: seedPath,
		BuzzRelayURL:    "ws://127.0.0.1:3000",
		BuzzDatabaseURL: "postgres://localhost/buzz",
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	passes := make(chan struct{}, 1)
	go service.repeatMembershipPasses(ctx, time.Millisecond, func(context.Context) time.Duration {
		passes <- struct{}{}
		return time.Hour
	})
	select {
	case <-passes:
		t.Fatal("a pass ran before the relay could be written to")
	case <-time.After(20 * time.Millisecond):
	}
	if errorValue := os.WriteFile(seedPath, []byte("late-seed"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	select {
	case <-passes:
	case <-time.After(5 * time.Second):
		t.Fatal("no pass ran after the relay became writable")
	}
}
