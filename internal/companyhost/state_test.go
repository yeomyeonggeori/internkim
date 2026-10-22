package companyhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASecretLeftEmptyByAnInterruptedInstallIsDrawnAgain(t *testing.T) {
	secretDirectoryPath := t.TempDir()
	path := filepath.Join(secretDirectoryPath, databasePasswordFileName)
	if errorValue := os.WriteFile(path, nil, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	value, errorValue := KeepSecret(secretDirectoryPath, databasePasswordFileName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	saved, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !secretPattern.MatchString(value) || strings.TrimSpace(string(saved)) != value {
		t.Fatalf("drew %q and saved %q", value, saved)
	}
}

func TestASavedSecretThatIsNotOneIsRefusedRatherThanReplaced(t *testing.T) {
	secretDirectoryPath := t.TempDir()
	path := filepath.Join(secretDirectoryPath, databasePasswordFileName)
	if errorValue := os.WriteFile(path, []byte("not-a-secret\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := KeepSecret(secretDirectoryPath, databasePasswordFileName); errorValue == nil {
		t.Fatal("a saved secret that is not one was replaced")
	}
}
