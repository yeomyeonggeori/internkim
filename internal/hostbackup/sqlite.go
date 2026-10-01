package hostbackup

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Snapshotter interface {
	Snapshot(databasePath string) (string, error)
}

type SQLiteSnapshots struct {
	ScratchDirectoryPath string
}

func (snapshots SQLiteSnapshots) Snapshot(databasePath string) (string, error) {
	scratch, errorValue := os.CreateTemp(snapshots.ScratchDirectoryPath, filepath.Base(databasePath)+".*.snapshot")
	if errorValue != nil {
		return "", errorValue
	}
	snapshotPath := scratch.Name()
	scratch.Close()
	os.Remove(snapshotPath)
	if errorValue := vacuumInto(databasePath, snapshotPath); errorValue != nil {
		os.Remove(snapshotPath)
		return "", errorValue
	}
	return snapshotPath, nil
}

func vacuumInto(databasePath string, snapshotPath string) error {
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(30000)")
	database, errorValue := sql.Open("sqlite", "file:"+databasePath+"?"+query.Encode())
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	if _, errorValue := database.Exec("VACUUM INTO ?", snapshotPath); errorValue != nil {
		return fmt.Errorf("VACUUM INTO: %w", errorValue)
	}
	return nil
}
