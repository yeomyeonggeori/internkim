package companyhost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/hostversion"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type RestoreRequest struct {
	ArchivePath string
	Replace     bool
	Now         time.Time
}

type RestoreResult struct {
	Manifest     hostbackup.Manifest
	Connection   Connection
	Files        hostbackup.ExtractionReport
	Unresolved   []hostbackup.Ownership
	SafetyBackup string
	Supervisor   string
}

func Restore(request RestoreRequest, machine Machine, progress io.Writer) (RestoreResult, error) {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	backing, errorValue := backupPlatformOf(platform)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	directoryPath, errorValue := prepareBackupDirectory("")
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	release, errorValue := holdTheBackupLock(directoryPath)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	defer release()
	return restoreWhileLocked(backing, request, directoryPath, machine, progress)
}

func restoreWhileLocked(platform backupPlatform, request RestoreRequest, backupDirectoryPath string, machine Machine, progress io.Writer) (RestoreResult, error) {
	fmt.Fprintln(progress, "1/6 Checking the backup before anything changes…")
	archive, errorValue := checkTheArchive(platform, request, machine)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	result := RestoreResult{Manifest: archive.Manifest, Supervisor: platform.NameOfItsSupervisor()}
	if request.Replace {
		fmt.Fprintln(progress, "2/6 Backing up what this computer holds now, so the restore can be undone…")
		safety, errorValue := backupWhileLocked(platform, BackupRequest{DirectoryPath: backupDirectoryPath, Now: request.Now}, machine, io.Discard)
		if errorValue != nil {
			return RestoreResult{}, fmt.Errorf("the backup of this computer's current state failed, so nothing was restored: %w", errorValue)
		}
		result.SafetyBackup = safety.ArchivePath
		fmt.Fprintf(progress, "  %s\n", safety.ArchivePath)
	} else {
		fmt.Fprintln(progress, "2/6 This computer holds no company, so there is nothing to keep before restoring…")
	}
	fmt.Fprintln(progress, "3/6 Stopping the services…")
	if errorValue := platform.StopTheHost(machine, progress); errorValue != nil {
		return RestoreResult{}, errorValue
	}
	fmt.Fprintln(progress, "4/6 Restoring the files and the people's accounts…")
	extracted, unresolved, errorValue := restoreTheFiles(platform, archive, machine)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	result.Files, result.Unresolved = extracted, unresolved
	connection, directoryPath, errorValue := restoredCompany(archive.Manifest)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	result.Connection = connection
	fmt.Fprintln(progress, "5/6 Restoring the databases…")
	if errorValue := restoreTheDatabases(platform, archive, machine, directoryPath, connection, progress); errorValue != nil {
		return RestoreResult{}, errorValue
	}
	fmt.Fprintln(progress, "6/6 Starting the services and waiting for them to answer…")
	return result, startTheRestoredHost(platform, machine, directoryPath, connection, progress)
}

func checkTheArchive(platform backupPlatform, request RestoreRequest, machine Machine) (hostbackup.Archive, error) {
	archive, errorValue := hostbackup.Open(request.ArchivePath)
	if errorValue != nil {
		return hostbackup.Archive{}, errorValue
	}
	manifest := archive.Manifest
	if errorValue := refuseUnknownMembers(manifest); errorValue != nil {
		return hostbackup.Archive{}, errorValue
	}
	if errorValue := refuseANewerPackage(manifest.PackageVersion, PackageVersion); errorValue != nil {
		return hostbackup.Archive{}, errorValue
	}
	if errorValue := refuseToOverwriteWithoutConsent(manifest.CompanyID, request.Replace); errorValue != nil {
		return hostbackup.Archive{}, errorValue
	}
	if errorValue := refuseAnArchiveInsideWhatItReplaces(request.ArchivePath, platform.Layout(), manifest.FileRoots); errorValue != nil {
		return hostbackup.Archive{}, errorValue
	}
	if errorValue := archive.Verify(); errorValue != nil {
		return hostbackup.Archive{}, fmt.Errorf("%s was refused before anything changed: %w", request.ArchivePath, errorValue)
	}
	if errorValue := refuseADumpTheHostCannotRead(platform, archive, machine); errorValue != nil {
		return hostbackup.Archive{}, errorValue
	}
	return archive, nil
}

func refuseUnknownMembers(manifest hostbackup.Manifest) error {
	if _, hasFiles := manifest.Member(hostbackup.FilesMemberName); !hasFiles {
		return fmt.Errorf("the backup carries no %s, so it holds no company to restore", hostbackup.FilesMemberName)
	}
	for _, member := range manifest.Members {
		if !isRestorableMember(member.Name) {
			return fmt.Errorf("the backup carries %s, which this internkim does not know how to restore", member.Name)
		}
	}
	if !containsRole(manifest.FileRoots, stateRole) {
		return fmt.Errorf("the backup holds no %s files, which is where the company's keys are, so it cannot bring the company back", stateRole)
	}
	roles := map[string]bool{}
	for _, root := range hostFileRoots(blueclaw.LinuxCompanyHostLayout()) {
		roles[root.Role] = true
	}
	for _, role := range manifest.FileRoots {
		if !roles[role] {
			return fmt.Errorf("the backup holds files under %q, which this internkim does not know where to put", role)
		}
	}
	return nil
}

func isRestorableMember(name string) bool {
	if name == hostbackup.FilesMemberName || name == hostbackup.RosterMemberName {
		return true
	}
	database, isDatabase := hostbackup.DatabaseNamedBy(name)
	if !isDatabase {
		return false
	}
	for _, known := range hostDatabases {
		if database == known {
			return true
		}
	}
	return false
}

func refuseANewerPackage(archived string, installed string) error {
	archivedVersion, errorValue := hostversion.Parse(archived)
	if errorValue != nil {
		return nil
	}
	installedVersion, errorValue := hostversion.Parse(installed)
	if errorValue != nil {
		return nil
	}
	if hostversion.Compare(archivedVersion, installedVersion) > 0 {
		return fmt.Errorf(
			"the backup was made by internkim %s and this computer runs %s. Its databases may carry changes this version cannot read, so upgrade internkim first",
			archived, installed)
	}
	return nil
}

func refuseToOverwriteWithoutConsent(archivedCompanyID string, isReplacing bool) error {
	installedID, errorValue := installedCompanyID()
	if errorValue != nil {
		return errorValue
	}
	if installedID == "" {
		if isReplacing {
			return fmt.Errorf("this computer holds no company, so there is nothing to replace. Run the restore without --replace")
		}
		return nil
	}
	if installedID != archivedCompanyID {
		return fmt.Errorf(
			"this computer runs company %s and the backup is of company %s. Restore it onto a computer that holds no company, or onto the one it came from",
			installedID, archivedCompanyID)
	}
	if !isReplacing {
		return fmt.Errorf(
			"this computer already runs this company, and restoring replaces everything it holds now with the backup. Run the restore again with --replace; this computer's current state is backed up first")
	}
	return nil
}

func refuseAnArchiveInsideWhatItReplaces(archivePath string, layout blueclaw.CompanyHostLayout, roles []string) error {
	absolute, errorValue := filepath.Abs(archivePath)
	if errorValue != nil {
		return errorValue
	}
	for _, root := range hostFileRoots(layout) {
		if !containsRole(roles, root.Role) {
			continue
		}
		if strings.HasPrefix(absolute, strings.TrimRight(root.Path, "/")+"/") {
			return fmt.Errorf("%s is inside %s, which the restore replaces. Move it to %s first", archivePath, root.Path, DefaultBackupDirectoryPath())
		}
	}
	return nil
}

func containsRole(roles []string, wanted string) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}

func restoreTheFiles(platform backupPlatform, archive hostbackup.Archive, machine Machine) (hostbackup.ExtractionReport, []hostbackup.Ownership, error) {
	destinations := map[string]string{}
	for _, root := range hostFileRoots(platform.Layout()) {
		if !containsRole(archive.Manifest.FileRoots, root.Role) {
			continue
		}
		if errorValue := emptyTheDirectory(root.Path); errorValue != nil {
			return hostbackup.ExtractionReport{}, nil, errorValue
		}
		destinations[root.Role] = root.Path
	}
	var extracted hostbackup.ExtractionReport
	errorValue := archive.ReadMember(hostbackup.FilesMemberName, func(input io.Reader) error {
		report, errorValue := hostbackup.ExtractFiles(input, destinations)
		extracted = report
		return errorValue
	})
	if errorValue != nil {
		return hostbackup.ExtractionReport{}, nil, errorValue
	}
	if errorValue := recreateThePeople(platform, archive, machine); errorValue != nil {
		return hostbackup.ExtractionReport{}, nil, errorValue
	}
	unresolved, errorValue := ownWhatThePeopleOwn(extracted.Unresolved)
	return extracted, unresolved, errorValue
}

func emptyTheDirectory(directoryPath string) error {
	entries, errorValue := os.ReadDir(directoryPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		if errorValue := os.RemoveAll(filepath.Join(directoryPath, entry.Name())); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func recreateThePeople(platform backupPlatform, archive hostbackup.Archive, machine Machine) error {
	if _, hasRoster := archive.Manifest.Member(hostbackup.RosterMemberName); !hasRoster {
		return nil
	}
	roster, errorValue := os.CreateTemp("", "internkim-restored-roster-*.json")
	if errorValue != nil {
		return errorValue
	}
	defer os.Remove(roster.Name())
	defer roster.Close()
	if errorValue := archive.ReadMember(hostbackup.RosterMemberName, func(input io.Reader) error {
		_, errorValue := io.Copy(roster, input)
		return errorValue
	}); errorValue != nil {
		return errorValue
	}
	layout := platform.Layout()
	arguments := []string{"sync", "--policy", roster.Name(), "--workspace", layout.WorkspacePath}
	var complaint strings.Builder
	if errorValue := machine.Run(layout.POSIXHelperPath(), arguments, nil, &complaint); errorValue != nil {
		return fmt.Errorf("the people's accounts could not be recreated from the backup's roster (%w): %s", errorValue, strings.TrimSpace(complaint.String()))
	}
	return nil
}

func ownWhatThePeopleOwn(unresolved []hostbackup.Ownership) ([]hostbackup.Ownership, error) {
	accounts := hostbackup.NewAccounts()
	remaining := []hostbackup.Ownership{}
	for _, ownership := range unresolved {
		isResolved, errorValue := hostbackup.ApplyOwnership(ownership, accounts)
		if errorValue != nil {
			return nil, errorValue
		}
		if !isResolved {
			remaining = append(remaining, ownership)
		}
	}
	return remaining, nil
}

func restoredCompany(manifest hostbackup.Manifest) (Connection, string, error) {
	directoryPath := DefaultStateDirectoryPath(manifest.CompanyID)
	connection, errorValue := ReadConnection(filepath.Join(directoryPath, connectionFileName))
	if errorValue != nil {
		return Connection{}, "", fmt.Errorf("the backup restored no connection file for company %s: %w", manifest.CompanyID, errorValue)
	}
	if connection.Company.ID != manifest.CompanyID {
		return Connection{}, "", fmt.Errorf("the backup's manifest names company %s and its connection file names %s", manifest.CompanyID, connection.Company.ID)
	}
	return connection, directoryPath, nil
}

func restoreTheDatabases(platform backupPlatform, archive hostbackup.Archive, machine Machine, directoryPath string, connection Connection, progress io.Writer) error {
	if _, errorValue := PrepareStateDirectory(directoryPath, connection); errorValue != nil {
		return errorValue
	}
	if errorValue := pointTheUnitsAtThisCompany(directoryPath); errorValue != nil {
		return errorValue
	}
	password, errorValue := KeepSecret(filepath.Join(directoryPath, secretDirectoryName), databasePasswordFileName)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := platform.StartTheDatabaseAndTheCache(machine); errorValue != nil {
		return errorValue
	}
	if errorValue := platform.RunDatabaseStatements(machine, databaseRemovalStatements(), progress); errorValue != nil {
		return fmt.Errorf("the host's databases could not be emptied for the restore: %w", errorValue)
	}
	settings := companyHostSettings{DirectoryPath: directoryPath, Connection: connection, DatabasePassword: password}
	if errorValue := prepareDatabases(platform, machine, settings, progress); errorValue != nil {
		return errorValue
	}
	for _, database := range hostDatabases {
		if errorValue := restoreOneDatabase(platform, archive, machine, database, progress); errorValue != nil {
			return errorValue
		}
	}
	return rehomeTheMessengerCommunity(platform, machine, connection, progress)
}

func databaseRemovalStatements() string {
	statements := []string{`SET client_min_messages = warning;`}
	for _, database := range hostDatabases {
		statements = append(statements, `DROP DATABASE IF EXISTS `+database+` WITH (FORCE);`)
	}
	return strings.Join(append(statements, ""), "\n")
}

func restoreOneDatabase(platform backupPlatform, archive hostbackup.Archive, machine Machine, database string, progress io.Writer) error {
	member, isCarried := archive.Manifest.Member(hostbackup.DatabaseMemberName(database))
	if !isCarried {
		fmt.Fprintf(progress, "  the backup carries no %s database; it starts empty and its service makes its schema\n", database)
		return nil
	}
	if errorValue := restoreUnderForeignOwners(platform, archive, machine, database, member, progress); errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(progress, "  %s restored (%s)\n", database, DescribeSize(member.Size))
	return nil
}

func startTheRestoredHost(platform backupPlatform, machine Machine, directoryPath string, connection Connection, progress io.Writer) error {
	request := Request{Connection: connection, StateDirectoryPath: directoryPath, PromptForModelKey: noModelKeyPrompt}
	if _, errorValue := prepareCompanyDirectory(platform, machine, directoryPath, connection, request); errorValue != nil {
		return errorValue
	}
	if errorValue := platform.StartTheBox(machine, progress); errorValue != nil {
		return errorValue
	}
	if errorValue := platform.SuperviseTheBundle(machine, progress); errorValue != nil {
		return errorValue
	}
	return waitUntilTheServerAnswers(platform, machine, progress)
}

func noModelKeyPrompt() (string, error) {
	return "", fmt.Errorf("the backup carries no OpenRouter key. Run `internkim install` with --model-key-file after the restore")
}
