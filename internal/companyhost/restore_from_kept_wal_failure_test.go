package companyhost

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const ownDatabaseContents = "this box's own database"

func aBoxWithItsOwnDatabase(t *testing.T) string {
	t.Helper()
	dataPath := filepath.Join(t.TempDir(), "postgres")
	if errorValue := os.MkdirAll(dataPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(dataPath, "PG_VERSION"), []byte(ownDatabaseContents), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return dataPath
}

func requireTheOwnDatabaseAlone(t *testing.T, dataPath string) {
	t.Helper()
	held, errorValue := os.ReadFile(filepath.Join(dataPath, "PG_VERSION"))
	if errorValue != nil || string(held) != ownDatabaseContents {
		t.Fatalf("the box's own database is not at %s: %q %v", dataPath, held, errorValue)
	}
	setAside, errorValue := filepath.Glob(dataPath + replacedDataMarker + "*")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(setAside) != 0 {
		t.Fatalf("a failed rebuild left copies set aside: %v", setAside)
	}
	if _, errorValue := os.Stat(dataPath + rebuildingMarkerSuffix); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("a failed rebuild left its marker behind: %v", errorValue)
	}
}

func aRebuildThatSucceeds(dataPath string) error {
	if errorValue := os.MkdirAll(dataPath, 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(filepath.Join(dataPath, "PG_VERSION"), []byte("a rebuilt database"), 0o600)
}

func aRebuildThatFailsHalfway(dataPath string) error {
	if errorValue := aRebuildThatSucceeds(dataPath); errorValue != nil {
		return errorValue
	}
	return errors.New("the base backup could not be read")
}

func aRecoveryThatSucceeds() error { return nil }

func TestAFailedRebuildLeavesTheBoxsOwnDatabaseWhereItWas(t *testing.T) {
	dataPath := aBoxWithItsOwnDatabase(t)

	_, errorValue := rebuildKeepingThisBoxDatabase(dataPath, time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC), aRebuildThatFailsHalfway, aRecoveryThatSucceeds, io.Discard)

	if errorValue == nil {
		t.Fatal("a rebuild that failed was reported as done")
	}
	requireTheOwnDatabaseAlone(t, dataPath)
}

func TestAFailedReplayLeavesTheBoxsOwnDatabaseWhereItWas(t *testing.T) {
	dataPath := aBoxWithItsOwnDatabase(t)

	_, errorValue := rebuildKeepingThisBoxDatabase(dataPath, time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC), aRebuildThatSucceeds, func() error { return errors.New("the kept WAL could not be replayed") }, io.Discard)

	if errorValue == nil {
		t.Fatal("a replay that failed was reported as done")
	}
	requireTheOwnDatabaseAlone(t, dataPath)
}

func TestASecondAttemptAfterAFailedOneStillFindsTheBoxsOwnDatabase(t *testing.T) {
	dataPath := aBoxWithItsOwnDatabase(t)
	first := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	rebuildKeepingThisBoxDatabase(dataPath, first, aRebuildThatFailsHalfway, aRecoveryThatSucceeds, io.Discard)

	rebuildKeepingThisBoxDatabase(dataPath, first.Add(time.Minute), aRebuildThatFailsHalfway, aRecoveryThatSucceeds, io.Discard)

	requireTheOwnDatabaseAlone(t, dataPath)
}

func TestARebuildCutOffMidwayIsDiscardedAndTheDatabaseSetAsideBeforeItIsKept(t *testing.T) {
	dataPath := aBoxWithItsOwnDatabase(t)
	setAsidePath := dataPath + replacedDataMarker + "20261118T090000Z"
	if errorValue := os.Rename(dataPath, setAsidePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := aRebuildThatSucceeds(dataPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(dataPath+rebuildingMarkerSuffix, nil, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	replacedPath, errorValue := setThisBoxDatabaseAside(dataPath, time.Date(2026, 11, 18, 9, 5, 0, 0, time.UTC))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if replacedPath != setAsidePath {
		t.Fatalf("the box's own database is said to be at %s, and it was set aside at %s", replacedPath, setAsidePath)
	}
	held, errorValue := os.ReadFile(filepath.Join(setAsidePath, "PG_VERSION"))
	if errorValue != nil || string(held) != ownDatabaseContents {
		t.Fatalf("the copy set aside is no longer the box's own database: %q %v", held, errorValue)
	}
	if _, errorValue := os.Stat(dataPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("the half-made rebuild is still at %s", dataPath)
	}
}
