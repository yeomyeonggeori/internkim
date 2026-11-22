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
	ArchivePath    string
	Log            KeptLog
	Replace        bool
	KeepsThisBox   bool
	EarlierFinding SafetyFinding
	FindingKept    func(SafetyFinding) error
	RebuildFromWAL func(dataDirectoryPath string) error
	ReplayFiles    FileJournalReplay
	BeforeStarting func(companyID string) error
	OnceServing    func(companyID string) error
	Now            time.Time
}

type KeptLog struct {
	CompanyID       string
	BaseBackupName  string
	TakenAt         time.Time
	PackageVersion  string
	PostgreSQLMajor int
}

type restoreSource struct {
	Manifest hostbackup.Manifest
	archive  *hostbackup.Archive
}

type RestoreResult struct {
	Manifest     hostbackup.Manifest
	IsFromTheLog bool
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
	source, errorValue := checkTheSource(platform, request, machine)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	result := RestoreResult{Manifest: source.Manifest, IsFromTheLog: source.archive == nil, Supervisor: platform.NameOfItsSupervisor()}
	safetyBackup, errorValue := keepWhatThisComputerHolds(platform, request, backupDirectoryPath, machine, progress)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	result.SafetyBackup = safetyBackup
	fmt.Fprintln(progress, "3/6 Stopping the services…")
	if errorValue := platform.StopTheHost(machine, progress); errorValue != nil {
		return RestoreResult{}, errorValue
	}
	fmt.Fprintln(progress, "4/6 Restoring the files and the people's accounts…")
	extracted, unresolved, errorValue := restoreTheFiles(platform, source, machine, request, progress)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	result.Files, result.Unresolved = extracted, unresolved
	connection, directoryPath, errorValue := restoredCompany(source.Manifest)
	if errorValue != nil {
		return RestoreResult{}, errorValue
	}
	result.Connection = connection
	fmt.Fprintln(progress, "5/6 Restoring the databases…")
	if errorValue := restoreTheDatabases(platform, source, machine, directoryPath, connection, request, progress); errorValue != nil {
		return RestoreResult{}, errorValue
	}
	fmt.Fprintln(progress, "6/6 Starting the services and waiting for them to answer…")
	return result, startTheRestoredHost(platform, machine, directoryPath, connection, request.BeforeStarting, request.OnceServing, progress)
}

func checkTheSource(platform backupPlatform, request RestoreRequest, machine Machine) (restoreSource, error) {
	if request.ArchivePath == "" {
		return checkTheLog(platform, request, machine)
	}
	archive, errorValue := checkTheArchive(platform, request, machine)
	if errorValue != nil {
		return restoreSource{}, errorValue
	}
	return restoreSource{Manifest: archive.Manifest, archive: &archive}, nil
}

func checkTheLog(platform backupPlatform, request RestoreRequest, machine Machine) (restoreSource, error) {
	kept := request.Log
	if kept.CompanyID == "" || request.RebuildFromWAL == nil || request.ReplayFiles == nil {
		return restoreSource{}, errors.New("a restore names the archive it restores, or restores a company from its log by rebuilding the database from its WAL and replaying its file journal")
	}
	if errorValue := refuseANewerPackage(kept.PackageVersion, PackageVersion); errorValue != nil {
		return restoreSource{}, errorValue
	}
	if errorValue := refuseAnotherMajor(platform, kept, machine); errorValue != nil {
		return restoreSource{}, errorValue
	}
	if errorValue := refuseToOverwriteWithoutConsent(kept.CompanyID, request.Replace); errorValue != nil {
		return restoreSource{}, errorValue
	}
	return restoreSource{Manifest: hostbackup.Manifest{
		CreatedAt:       kept.TakenAt,
		PackageVersion:  kept.PackageVersion,
		CompanyID:       kept.CompanyID,
		PostgreSQLMajor: kept.PostgreSQLMajor,
		FileRoots:       rolesOf(hostFileRoots(platform.Layout())),
	}}, nil
}

func refuseAnotherMajor(platform backupPlatform, kept KeptLog, machine Machine) error {
	if kept.PostgreSQLMajor == 0 {
		return fmt.Errorf("the base backup %s does not say which PostgreSQL major wrote it, and its WAL replays only on that major, so nothing was restored", kept.BaseBackupName)
	}
	hostMajor, errorValue := platform.DatabaseMajor(machine)
	if errorValue != nil {
		return errorValue
	}
	if hostMajor != kept.PostgreSQLMajor {
		return fmt.Errorf(
			"the company's database was kept by PostgreSQL %d and this computer runs PostgreSQL %d. WAL replays only on the major that wrote it, so install PostgreSQL %d here first",
			kept.PostgreSQLMajor, hostMajor, kept.PostgreSQLMajor)
	}
	return nil
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
	if name == hostbackup.FilesMemberName {
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

func restoreTheFiles(platform backupPlatform, source restoreSource, machine Machine, request RestoreRequest, progress io.Writer) (hostbackup.ExtractionReport, []hostbackup.Ownership, error) {
	var extracted hostbackup.ExtractionReport
	extract := func() error {
		destinations := destinationsOf(platform, source.Manifest.FileRoots)
		if errorValue := extractTheArchivedFiles(source, destinations, &extracted); errorValue != nil {
			return errorValue
		}
		return replayTheFileJournalOnto(source, request.ReplayFiles, destinations, &extracted, progress)
	}
	errorValue := extractKeepingThisBoxIf(request.KeepsThisBox, extract)
	if errorValue != nil {
		return hostbackup.ExtractionReport{}, nil, errorValue
	}
	if errorValue := recreateThePeople(platform, machine, progress); errorValue != nil {
		return hostbackup.ExtractionReport{}, nil, errorValue
	}
	unresolved, errorValue := ownWhatThePeopleOwn(extracted.Unresolved)
	return extracted, unresolved, errorValue
}

func extractTheArchivedFiles(source restoreSource, destinations map[string]string, extracted *hostbackup.ExtractionReport) error {
	if source.archive == nil {
		return nil
	}
	if errorValue := emptyTheDestinations(destinations); errorValue != nil {
		return errorValue
	}
	return source.archive.ReadMember(hostbackup.FilesMemberName, func(input io.Reader) error {
		report, errorValue := hostbackup.ExtractFiles(input, destinations)
		*extracted = report
		return errorValue
	})
}

func extractKeepingThisBoxIf(keepsThisBox bool, extract func() error) error {
	if !keepsThisBox {
		return extract()
	}
	return keepingThisBoxThrough(blueclaw.CompanyHostBoxStatePath, extract)
}

func destinationsOf(platform backupPlatform, roles []string) map[string]string {
	destinations := map[string]string{}
	for _, root := range hostFileRoots(platform.Layout()) {
		if containsRole(roles, root.Role) {
			destinations[root.Role] = root.Path
		}
	}
	return destinations
}

func emptyTheDestinations(destinations map[string]string) error {
	for _, rootPath := range destinations {
		if errorValue := emptyTheDirectory(rootPath); errorValue != nil {
			return errorValue
		}
	}
	return nil
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

func recreateThePeople(platform backupPlatform, machine Machine, progress io.Writer) error {
	layout := platform.Layout()
	rosterPath := layout.PolicyDocumentPath()
	_, errorValue := os.Stat(rosterPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		fmt.Fprintf(progress, "  the restored files hold no roster at %s, so no person's account is recreated until admind delivers the roster\n", rosterPath)
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	arguments := []string{"sync", "--policy", rosterPath, "--workspace", layout.WorkspacePath}
	var complaint strings.Builder
	if errorValue := machine.Run(layout.POSIXHelperPath(), arguments, nil, &complaint); errorValue != nil {
		return fmt.Errorf("the people's accounts could not be recreated from the restored roster at %s (%w): %s", rosterPath, errorValue, strings.TrimSpace(complaint.String()))
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

func restoreTheDatabases(platform backupPlatform, source restoreSource, machine Machine, directoryPath string, connection Connection, request RestoreRequest, progress io.Writer) error {
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
	settings := companyHostSettings{DirectoryPath: directoryPath, Connection: connection, DatabasePassword: password}
	if request.RebuildFromWAL != nil {
		isRebuilt, errorValue := rebuildTheDatabaseFromKeptWAL(platform, machine, request.RebuildFromWAL, request.Now, progress)
		if errorValue != nil {
			return errorValue
		}
		if isRebuilt {
			if errorValue := prepareDatabases(platform, machine, settings, progress); errorValue != nil {
				return errorValue
			}
			return rehomeTheMessengerCommunity(platform, machine, connection, progress)
		}
	}
	if source.archive == nil {
		return ErrNoKeptWAL
	}
	fmt.Fprintln(progress, "  the databases come from the backup's dumps")
	if errorValue := platform.StartTheDatabaseAndTheCache(machine); errorValue != nil {
		return errorValue
	}
	if errorValue := platform.RunDatabaseStatements(machine, databaseRemovalStatements(), progress); errorValue != nil {
		return fmt.Errorf("the host's databases could not be emptied for the restore: %w", errorValue)
	}
	if errorValue := prepareDatabases(platform, machine, settings, progress); errorValue != nil {
		return errorValue
	}
	for _, database := range hostDatabases {
		if errorValue := restoreOneDatabase(platform, *source.archive, machine, database, progress); errorValue != nil {
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

func startTheRestoredHost(platform backupPlatform, machine Machine, directoryPath string, connection Connection, beforeStarting func(companyID string) error, onceServing func(companyID string) error, progress io.Writer) error {
	request := Request{Connection: connection, StateDirectoryPath: directoryPath, PromptForModelKey: noModelKeyPrompt}
	if _, errorValue := prepareCompanyDirectory(platform, machine, directoryPath, connection, request); errorValue != nil {
		return errorValue
	}
	if beforeStarting != nil {
		if errorValue := beforeStarting(connection.Company.ID); errorValue != nil {
			return errorValue
		}
	}
	return startTheServicesThenConfirm(platform, machine, connection.Company.ID, onceServing, progress)
}

func startTheServicesThenConfirm(platform backupPlatform, machine Machine, companyID string, onceServing func(companyID string) error, progress io.Writer) error {
	if errorValue := platform.StartTheBox(machine, progress); errorValue != nil {
		return errorValue
	}
	if errorValue := platform.SuperviseTheBundle(machine, progress); errorValue != nil {
		return errorValue
	}
	if errorValue := waitUntilTheServerAnswers(platform, machine, progress); errorValue != nil {
		return errorValue
	}
	if onceServing == nil {
		return nil
	}
	return onceServing(companyID)
}

func noModelKeyPrompt() (string, error) {
	return "", nil
}
