package companyhost

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestARestoreThatKeepsThisBoxPutsItsOwnIdentityBackOverTheArchivedOne(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "lib", "internkim")
	boxStatePath := filepath.Join(stateRoot, "box")
	writeTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "this box")

	errorValue := keepingThisBoxThrough(boxStatePath, func() error {
		if _, statError := os.Stat(boxStatePath); !os.IsNotExist(statError) {
			t.Fatal("the box's own state was still in the directory being restored")
		}
		writeTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "the archived box")
		return nil
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requireTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "this box")
}

func TestAFailedRestoreStillPutsThisBoxBack(t *testing.T) {
	boxStatePath := filepath.Join(t.TempDir(), "lib", "internkim", "box")
	writeTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "this box")
	failure := errors.New("the archive ended early")

	errorValue := keepingThisBoxThrough(boxStatePath, func() error { return failure })

	if !errors.Is(errorValue, failure) {
		t.Fatalf("the restore's failure was lost: %v", errorValue)
	}
	requireTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "this box")
}

func TestABoxSetAsideByAnInterruptedRestoreIsTheOneKept(t *testing.T) {
	root := t.TempDir()
	boxStatePath := filepath.Join(root, "lib", "internkim", "box")
	writeTestFile(t, filepath.Join(root, "lib", ".internkim-box-during-restore", boxIdentityFileName), "this box")
	writeTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "half restored")

	if errorValue := keepingThisBoxThrough(boxStatePath, func() error { return nil }); errorValue != nil {
		t.Fatal(errorValue)
	}

	requireTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "this box")
}

func TestAComputerWithoutABoxIdentityTakesTheArchivedOne(t *testing.T) {
	boxStatePath := filepath.Join(t.TempDir(), "lib", "internkim", "box")

	errorValue := keepingThisBoxThrough(boxStatePath, func() error {
		writeTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "the archived box")
		return nil
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requireTestFile(t, filepath.Join(boxStatePath, boxIdentityFileName), "the archived box")
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(contents), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func requireTestFile(t *testing.T, path, expected string) {
	t.Helper()
	contents, errorValue := os.ReadFile(path)
	if errorValue != nil || string(contents) != expected {
		t.Fatalf("%s holds %q, %v; expected %q", path, contents, errorValue, expected)
	}
}
