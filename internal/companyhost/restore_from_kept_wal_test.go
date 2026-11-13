package companyhost

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTheDatabaseIsSetAsideAndAnEarlierOneForgotten(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "postgres")
	earlierPath := dataPath + replacedDataMarker + "20260101T000000Z"
	for _, path := range []string{dataPath, earlierPath} {
		if errorValue := os.MkdirAll(path, 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	replacedPath, errorValue := setThisBoxDatabaseAside(dataPath, time.Date(2026, 11, 13, 8, 23, 14, 0, time.UTC))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if replacedPath != dataPath+replacedDataMarker+"20261113T082314Z" {
		t.Fatalf("the database was set aside at %s", replacedPath)
	}
	if _, errorValue := os.Stat(replacedPath); errorValue != nil {
		t.Fatalf("nothing is at %s: %v", replacedPath, errorValue)
	}
	if _, errorValue := os.Stat(earlierPath); !os.IsNotExist(errorValue) {
		t.Fatalf("the database an earlier take-over set aside is still at %s", earlierPath)
	}
}

func TestATakeOverThatStoppedAfterSettingTheDatabaseAsideCanBeTriedAgain(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "postgres")
	earlierPath := dataPath + replacedDataMarker + "20261113T082314Z"
	if errorValue := os.MkdirAll(earlierPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}

	replacedPath, errorValue := setThisBoxDatabaseAside(dataPath, time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))

	if errorValue != nil {
		t.Fatalf("a second take-over could not start where the first one stopped: %v", errorValue)
	}
	if replacedPath != earlierPath {
		t.Fatalf("the box's own database is said to be at %s, and the earlier take-over left it at %s", replacedPath, earlierPath)
	}
	if _, errorValue := os.Stat(earlierPath); errorValue != nil {
		t.Fatalf("the only copy of this box's own database is gone: %v", errorValue)
	}
}
