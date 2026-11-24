package companyhost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type companyPlaces struct {
	StateRootPath    string
	KeptInStateRoot  string
	StateDirectories []blueclaw.CompanyHostDirectory
	EmptiedPaths     []string
	RemovedPaths     []string
	DatabaseDataPath string
}

func companyPlacesOn(layout blueclaw.CompanyHostLayout) companyPlaces {
	return companyPlaces{
		StateRootPath:    blueclaw.CompanyHostStateRoot,
		KeptInStateRoot:  filepath.Base(blueclaw.CompanyHostBoxStatePath),
		StateDirectories: blueclaw.CompanyHostStateDirectories(),
		EmptiedPaths: []string{
			blueclaw.RelayStateDirectoryPath(blueclaw.CompanyHostRelayStateDirectoryName),
			AdministrationStateRoot,
			layout.WorkspacePath,
			layout.RunPath,
			blueclaw.CompanyHostRosterPath,
			blueclaw.CompanyHostDatabaseDataPath,
			blueclaw.CompanyHostCacheDataPath,
			blueclaw.CompanyHostBackupsPath,
			blueclaw.CompanyHostBrowserStatePath,
			blueclaw.CompanyHostLogPath,
		},
		RemovedPaths:     []string{blueclaw.RelayEnvironmentFilePath, blueclaw.RelayAgentKeyPath},
		DatabaseDataPath: blueclaw.CompanyHostDatabaseDataPath,
	}
}

type CompaniesHeld struct {
	Current     string
	Directories []string
}

func (held CompaniesHeld) OtherThan(companyID string) []string {
	others := []string{}
	for _, heldID := range append([]string{held.Current}, held.Directories...) {
		if heldID != "" && heldID != companyID && !slices.Contains(others, heldID) {
			others = append(others, heldID)
		}
	}
	return others
}

func CompaniesHeldHere() (CompaniesHeld, error) {
	current, errorValue := installedCompanyID()
	if errorValue != nil {
		return CompaniesHeld{}, errorValue
	}
	directories, unrecognized, errorValue := companyDirectoriesIn(blueclaw.CompanyHostCompaniesRoot)
	if errorValue != nil {
		return CompaniesHeld{}, errorValue
	}
	for _, name := range unrecognized {
		if _, isReported := reportedUnrecognizedDirectories.LoadOrStore(name, true); !isReported {
			log.Printf("%s holds %s, which is not named by a company id; it is no company's and is left where it is", blueclaw.CompanyHostCompaniesRoot, name)
		}
	}
	return CompaniesHeld{Current: current, Directories: directories}, nil
}

var reportedUnrecognizedDirectories sync.Map

func companyDirectoriesIn(companiesRootPath string) ([]string, []string, error) {
	entries, errorValue := os.ReadDir(companiesRootPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if errorValue != nil {
		return nil, nil, fmt.Errorf("reading which companies %s holds: %w", companiesRootPath, errorValue)
	}
	companyIDs, unrecognized := []string{}, []string{}
	for _, entry := range entries {
		if entry.IsDir() && isCompanyID(entry.Name()) {
			companyIDs = append(companyIDs, entry.Name())
			continue
		}
		unrecognized = append(unrecognized, entry.Name())
	}
	return companyIDs, unrecognized, nil
}

func isCompanyID(name string) bool {
	parsed, errorValue := uuid.Parse(name)
	return errorValue == nil && parsed.String() == name
}

func ForgetCompanyDirectory(companyID string) error {
	current, errorValue := installedCompanyID()
	if errorValue != nil {
		return errorValue
	}
	return forgetCompanyDirectoryIn(blueclaw.CompanyHostCompaniesRoot, current, companyID)
}

func forgetCompanyDirectoryIn(companiesRootPath, current, companyID string) error {
	if !isCompanyID(companyID) {
		return fmt.Errorf("%q is not a company id, so no directory under %s is removed for it", companyID, companiesRootPath)
	}
	if companyID == current {
		return fmt.Errorf("company %s is the one %s serves, so its directory under %s stays", companyID, blueclaw.CompanyHostCurrentPath, companiesRootPath)
	}
	return os.RemoveAll(filepath.Join(companiesRootPath, companyID))
}

func ForgetEveryCompany(machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	linux, isLinux := platform.(linuxPlatform)
	if !isLinux {
		return fmt.Errorf("a computer is emptied of an earlier company on a Linux host; on %s nothing was removed", platform.Describe())
	}
	return forgetEveryCompanyOn(linux, companyPlacesOn(linux.Layout()), machine, progress)
}

func forgetEveryCompanyOn(platform linuxPlatform, places companyPlaces, machine Machine, progress io.Writer) error {
	held, _, errorValue := companyDirectoriesIn(filepath.Join(places.StateRootPath, filepath.Base(blueclaw.CompanyHostCompaniesRoot)))
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(progress, "removing the data of companies %s: their services, people's accounts, %s, the databases set aside by a take-over and %s apart from %s\n", strings.Join(held, ", "), strings.Join(append(append([]string{}, places.RemovedPaths...), places.EmptiedPaths...), ", "), places.StateRootPath, places.KeptInStateRoot)
	if errorValue := platform.stopEveryCompanyService(machine, progress); errorValue != nil {
		return errorValue
	}
	if errorValue := forgetThePeople(platform.Layout(), machine); errorValue != nil {
		return errorValue
	}
	return places.forget()
}

func (platform linuxPlatform) stopEveryCompanyService(machine Machine, progress io.Writer) error {
	if errorValue := machine.Run("systemctl", append([]string{"disable", "--now"}, platform.companyServiceUnits()...), nil, progress); errorValue != nil {
		return fmt.Errorf("the earlier company's services could not be stopped, so nothing was removed: %w", errorValue)
	}
	dataUnits := []string{}
	for _, unit := range blueclaw.CompanyHostDataServiceUnits(platform.Layout()) {
		dataUnits = append(dataUnits, unit.FileName())
	}
	if errorValue := machine.Run("systemctl", append([]string{"stop"}, dataUnits...), nil, progress); errorValue != nil {
		return fmt.Errorf("the earlier company's database and cache could not be stopped, so nothing was removed: %w", errorValue)
	}
	return nil
}

func forgetThePeople(layout blueclaw.CompanyHostLayout, machine Machine) error {
	var complaint strings.Builder
	if errorValue := machine.Run(layout.POSIXHelperPath(), []string{"forget-projected-identities"}, nil, &complaint); errorValue != nil {
		return fmt.Errorf("the earlier company's accounts could not be removed, so its files were left where they are (%w): %s", errorValue, strings.TrimSpace(complaint.String()))
	}
	return nil
}

func (places companyPlaces) forget() error {
	for _, path := range places.RemovedPaths {
		if errorValue := os.Remove(path); errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
			return errorValue
		}
	}
	for _, directoryPath := range places.EmptiedPaths {
		if errorValue := emptyTheDirectory(directoryPath); errorValue != nil {
			return errorValue
		}
	}
	replacedDatabases, errorValue := filepath.Glob(places.DatabaseDataPath + replacedDataMarker + "*")
	if errorValue != nil {
		return errorValue
	}
	for _, replacedPath := range replacedDatabases {
		if errorValue := os.RemoveAll(replacedPath); errorValue != nil {
			return errorValue
		}
	}
	return places.emptyTheStateRoot()
}

func (places companyPlaces) emptyTheStateRoot() error {
	entries, errorValue := os.ReadDir(places.StateRootPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		if entry.Name() == places.KeptInStateRoot {
			continue
		}
		if errorValue := os.RemoveAll(filepath.Join(places.StateRootPath, entry.Name())); errorValue != nil {
			return errorValue
		}
	}
	return places.makeTheStateDirectoriesAgain()
}

func (places companyPlaces) makeTheStateDirectoriesAgain() error {
	for _, directory := range places.StateDirectories {
		if errorValue := os.MkdirAll(directory.Path, directory.Mode); errorValue != nil {
			return errorValue
		}
		if errorValue := os.Chmod(directory.Path, directory.Mode); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
