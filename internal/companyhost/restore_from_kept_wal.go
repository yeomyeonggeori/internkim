package companyhost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

var ErrNoKeptWAL = errors.New("the company keeps no base backup and WAL to rebuild its database from")

const (
	replacedDataTimeLayout = "20060102T150405Z"
	replacedDataMarker     = ".before-take-over-"
	rebuildingMarkerSuffix = ".rebuilding"
)

func rebuildTheDatabaseFromKeptWAL(platform backupPlatform, machine Machine, rebuild func(dataDirectoryPath string) error, now time.Time, progress io.Writer) (bool, error) {
	if !platform.Layout().RunsTheDataServiceScript() {
		return false, fmt.Errorf("the database is rebuilt from kept WAL only where the data-service script runs it, and on %s it does not", platform.Describe())
	}
	dataPath := blueclaw.CompanyHostDatabaseDataPath
	if errorValue := machine.Run("systemctl", []string{"stop", blueclaw.CompanyHostDatabaseServiceName + ".service"}, nil, progress); errorValue != nil {
		return false, fmt.Errorf("the database could not be stopped to rebuild it: %w", errorValue)
	}
	replay := func() error {
		owner := blueclaw.CompanyHostDatabaseUser + ":" + blueclaw.CompanyHostDatabaseUser
		if errorValue := machine.Run("chown", []string{"-R", owner, dataPath}, nil, progress); errorValue != nil {
			return fmt.Errorf("giving the rebuilt database to %s: %w", blueclaw.CompanyHostDatabaseUser, errorValue)
		}
		fmt.Fprintln(progress, "  replaying the kept WAL; a day of it can take some minutes")
		recovery := []string{"-u", blueclaw.CompanyHostDatabaseUser, "--", platform.Layout().DataServicePath(), "postgres-recover"}
		if errorValue := machine.Run("runuser", recovery, nil, progress); errorValue != nil {
			return fmt.Errorf("replaying the kept WAL into the rebuilt database: %w", errorValue)
		}
		return nil
	}
	return rebuildKeepingThisBoxDatabase(dataPath, now, rebuild, replay, progress)
}

func rebuildKeepingThisBoxDatabase(dataPath string, now time.Time, rebuild func(dataDirectoryPath string) error, replay func() error, progress io.Writer) (bool, error) {
	replacedPath, errorValue := setThisBoxDatabaseAside(dataPath, now)
	if errorValue != nil {
		return false, errorValue
	}
	if errorValue := os.WriteFile(dataPath+rebuildingMarkerSuffix, nil, 0o600); errorValue != nil {
		return false, errors.Join(fmt.Errorf("marking %s as being rebuilt: %w", dataPath, errorValue), putTheDatabaseBackAfter(dataPath, replacedPath))
	}
	errorValue = rebuild(dataPath)
	if errors.Is(errorValue, ErrNoKeptWAL) {
		fmt.Fprintln(progress, "  the company keeps no base backup and WAL yet")
		return false, putTheDatabaseBackAfter(dataPath, replacedPath)
	}
	if errorValue == nil {
		errorValue = replay()
	}
	if errorValue != nil {
		return false, errors.Join(fmt.Errorf("rebuilding the database from the company's kept WAL: %w", errorValue), putTheDatabaseBackAfter(dataPath, replacedPath))
	}
	if errorValue := os.Remove(dataPath + rebuildingMarkerSuffix); errorValue != nil {
		return false, fmt.Errorf("the database was rebuilt, and its marker at %s could not be removed: %w", dataPath+rebuildingMarkerSuffix, errorValue)
	}
	fmt.Fprintf(progress, "  %s\n", whereThisBoxDatabaseIsKept(replacedPath))
	return true, nil
}

func putTheDatabaseBackAfter(dataPath, replacedPath string) error {
	if errorValue := putTheReplacedDataBack(dataPath, replacedPath); errorValue != nil {
		return fmt.Errorf("putting this box's own database back at %s: %w", dataPath, errorValue)
	}
	errorValue := os.Remove(dataPath + rebuildingMarkerSuffix)
	if errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
		return fmt.Errorf("this box's own database is back at %s, and the marker of the rebuild could not be removed: %w", dataPath, errorValue)
	}
	return nil
}

func whereThisBoxDatabaseIsKept(replacedPath string) string {
	if replacedPath == "" {
		return "this box had no database of its own to keep"
	}
	return "this box's own database is kept at " + replacedPath
}

func setThisBoxDatabaseAside(dataPath string, now time.Time) (string, error) {
	if errorValue := discardARebuildThatWasCutOff(dataPath); errorValue != nil {
		return "", errorValue
	}
	if _, errorValue := os.Stat(dataPath); errors.Is(errorValue, fs.ErrNotExist) {
		return whereAnEarlierTakeOverSetTheDatabaseAside(dataPath)
	}
	if errorValue := forgetEarlierReplacedData(dataPath); errorValue != nil {
		return "", errorValue
	}
	replacedPath := dataPath + replacedDataMarker + now.UTC().Format(replacedDataTimeLayout)
	if errorValue := os.Rename(dataPath, replacedPath); errorValue != nil {
		return "", fmt.Errorf("moving this box's own database aside to %s: %w", replacedPath, errorValue)
	}
	return replacedPath, nil
}

func whereAnEarlierTakeOverSetTheDatabaseAside(dataPath string) (string, error) {
	earlier, errorValue := filepath.Glob(dataPath + replacedDataMarker + "*")
	if errorValue != nil {
		return "", errorValue
	}
	if len(earlier) == 0 {
		return "", nil
	}
	return slices.Max(earlier), nil
}

func putTheReplacedDataBack(dataPath, replacedPath string) error {
	if errorValue := os.RemoveAll(dataPath); errorValue != nil {
		return errorValue
	}
	if replacedPath == "" {
		return nil
	}
	return os.Rename(replacedPath, dataPath)
}

func forgetEarlierReplacedData(dataPath string) error {
	earlier, errorValue := filepath.Glob(dataPath + replacedDataMarker + "*")
	if errorValue != nil {
		return errorValue
	}
	for _, path := range earlier {
		if errorValue := os.RemoveAll(path); errorValue != nil {
			return fmt.Errorf("removing the database an earlier take-over set aside at %s: %w", path, errorValue)
		}
	}
	return nil
}

func discardARebuildThatWasCutOff(dataPath string) error {
	if _, errorValue := os.Stat(dataPath + rebuildingMarkerSuffix); errors.Is(errorValue, fs.ErrNotExist) {
		return nil
	}
	if errorValue := os.RemoveAll(dataPath); errorValue != nil {
		return fmt.Errorf("removing the rebuild an earlier take-over left unfinished at %s: %w", dataPath, errorValue)
	}
	return os.Remove(dataPath + rebuildingMarkerSuffix)
}
