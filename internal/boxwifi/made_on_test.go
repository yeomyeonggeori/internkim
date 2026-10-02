package boxwifi

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTheDayABoxWasMadeIsReadFromItsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "internkim-made-on")
	if errorValue := os.WriteFile(path, []byte("2026-10-02\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	madeOn, errorValue := ReadMadeOn(path)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !madeOn.Equal(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("made on = %v, want 2026-10-02", madeOn)
	}
}

func TestABoxWithoutAMadeOnFileHasNoDate(t *testing.T) {
	madeOn, errorValue := ReadMadeOn(filepath.Join(t.TempDir(), "absent"))

	if errorValue != nil || !madeOn.IsZero() {
		t.Fatalf("made on = %v, error = %v, want no date and no error", madeOn, errorValue)
	}
}

func TestAMadeOnFileInAnotherShapeIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "internkim-made-on")
	if errorValue := os.WriteFile(path, []byte("261002"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := ReadMadeOn(path); errorValue == nil {
		t.Fatal("expected a date that is not YYYY-MM-DD to be refused")
	}
}
