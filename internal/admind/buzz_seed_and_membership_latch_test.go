package admind

import (
	"os"
	"path/filepath"
	"testing"
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
