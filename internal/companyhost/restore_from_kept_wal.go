package companyhost

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

var ErrNoKeptWAL = errors.New("the company keeps no base backup and WAL to rebuild its database from")

const replacedDataTimeLayout = "20060102T150405Z"

func rebuildTheDatabaseFromKeptWAL(platform backupPlatform, machine Machine, rebuild func(dataDirectoryPath string) error, now time.Time, progress io.Writer) (bool, error) {
	if !platform.Layout().OwnsItsDataServices() {
		return false, fmt.Errorf("the database is rebuilt from kept WAL only where the package runs it, and on %s it does not", platform.Describe())
	}
	dataPath := blueclaw.CompanyHostDatabaseDataPath
	replacedPath := dataPath + ".before-take-over-" + now.UTC().Format(replacedDataTimeLayout)
	if errorValue := machine.Run("systemctl", []string{"stop", blueclaw.CompanyHostDatabaseServiceName + ".service"}, nil, progress); errorValue != nil {
		return false, fmt.Errorf("the database could not be stopped to rebuild it: %w", errorValue)
	}
	if errorValue := os.Rename(dataPath, replacedPath); errorValue != nil {
		return false, fmt.Errorf("moving this box's own database aside to %s: %w", replacedPath, errorValue)
	}
	errorValue := rebuild(dataPath)
	if errors.Is(errorValue, ErrNoKeptWAL) {
		fmt.Fprintln(progress, "  the company keeps no WAL yet, so the databases come from the snapshot's dumps")
		if errorValue := putTheReplacedDataBack(dataPath, replacedPath); errorValue != nil {
			return false, errorValue
		}
		return false, nil
	}
	if errorValue != nil {
		return false, fmt.Errorf("rebuilding the database from the company's kept WAL (this box's own database is kept at %s): %w", replacedPath, errorValue)
	}
	owner := blueclaw.CompanyHostDatabaseUser + ":" + blueclaw.CompanyHostDatabaseUser
	if errorValue := machine.Run("chown", []string{"-R", owner, dataPath}, nil, progress); errorValue != nil {
		return false, errorValue
	}
	fmt.Fprintf(progress, "  this box's own database is kept at %s\n", replacedPath)
	return true, nil
}

func putTheReplacedDataBack(dataPath, replacedPath string) error {
	if errorValue := os.RemoveAll(dataPath); errorValue != nil {
		return errorValue
	}
	return os.Rename(replacedPath, dataPath)
}
