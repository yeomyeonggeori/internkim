package admind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func stagedDirectoryForTest(t *testing.T, path string, age time.Duration) string {
	t.Helper()
	if errorValue := os.MkdirAll(path, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(path, "payload.tar.gz"), []byte("bytes"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	when := time.Now().Add(-age)
	if errorValue := os.Chtimes(path, when, when); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func exists(path string) bool {
	_, errorValue := os.Stat(path)
	return errorValue == nil
}

func TestAnUpdateClearsWhatItStagedHoweverItEnds(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{StateDirectory: stateDirectory})
	staged := stagedDirectoryForTest(t, service.releaseStagingPath("job-1"), 0)

	service.forgetReleaseStaging("job-1")

	if exists(staged) {
		t.Fatal("an update that has been installed keeps none of what it downloaded")
	}
}

func TestAnUpdateNobodyFinishedIsClearedOnTheNextStart(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{StateDirectory: stateDirectory})
	abandoned := stagedDirectoryForTest(t, service.releaseStagingPath("job-old"), 48*time.Hour)
	uploading := stagedDirectoryForTest(
		t,
		filepath.Join(stateDirectory, "blueclaw-updates", "uploads", "upload-old"),
		48*time.Hour,
	)

	service.sweepUpdateLeftovers()

	if exists(abandoned) || exists(uploading) {
		t.Fatal("a device that already filled up recovers by starting")
	}
}

func TestAnUpdateStillRunningIsLeftAlone(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{StateDirectory: stateDirectory})
	running := stagedDirectoryForTest(t, service.releaseStagingPath("job-now"), time.Minute)

	service.sweepUpdateLeftovers()

	if !exists(running) {
		t.Fatal("an update in flight owns its directory")
	}
}

func TestOnlyTheNewestPayloadsAreKept(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{StateDirectory: stateDirectory})
	artifacts := filepath.Join(stateDirectory, "blueclaw-updates", "artifacts")
	oldest := stagedDirectoryForTest(t, filepath.Join(artifacts, "rev-1"), 72*time.Hour)
	middle := stagedDirectoryForTest(t, filepath.Join(artifacts, "rev-2"), 48*time.Hour)
	newest := stagedDirectoryForTest(t, filepath.Join(artifacts, "rev-3"), 24*time.Hour)

	service.sweepUpdateLeftovers()

	if exists(oldest) {
		t.Fatal("a payload older than what a rollback would reach is not kept")
	}
	if !exists(middle) || !exists(newest) {
		t.Fatal("the payload running and the one before it are what a rollback needs")
	}
}
