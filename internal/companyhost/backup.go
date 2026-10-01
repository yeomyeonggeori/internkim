package companyhost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

var PackageVersion = ""

const (
	backupArchivePrefix    = "internkim-backup-"
	backupArchiveSuffix    = ".tar"
	backupStampLayout      = "20060102T150405Z"
	backupDirectoryMode    = 0o700
	backupLockFileName     = ".lock"
	backupScratchPattern   = ".scratch-"
	backupPartialExtension = ".partial"
)

var backupArchiveNamePattern = regexp.MustCompile(`^` + regexp.QuoteMeta(backupArchivePrefix) + `\d{8}T\d{6}Z` + regexp.QuoteMeta(backupArchiveSuffix) + `$`)

var hostDatabases = []string{blueclaw.BlueclawDatabaseName, blueclaw.BuzzRelayDatabaseName}

type BackupRequest struct {
	DirectoryPath string
	Keep          int
	Now           time.Time
}

type BackupResult struct {
	ArchivePath string
	Manifest    hostbackup.Manifest
	Files       hostbackup.FilesReport
	Pruned      []string
}

func DefaultBackupDirectoryPath() string {
	return blueclaw.CompanyHostBackupsPath
}

func Backup(request BackupRequest, machine Machine, progress io.Writer) (BackupResult, error) {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	backing, errorValue := backupPlatformOf(platform)
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	directoryPath, errorValue := prepareBackupDirectory(request.DirectoryPath)
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	release, errorValue := holdTheBackupLock(directoryPath)
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	defer release()
	request.DirectoryPath = directoryPath
	return backupWhileLocked(backing, request, machine, progress)
}

func backupWhileLocked(platform backupPlatform, request BackupRequest, machine Machine, progress io.Writer) (BackupResult, error) {
	companyID, errorValue := installedCompanyID()
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	if companyID == "" {
		return BackupResult{}, fmt.Errorf("this computer holds no company, so there is nothing to back up")
	}
	if errorValue := removeWhatAnInterruptedBackupLeft(request.DirectoryPath); errorValue != nil {
		return BackupResult{}, errorValue
	}
	archivePath := filepath.Join(request.DirectoryPath, backupArchivePrefix+request.Now.UTC().Format(backupStampLayout)+backupArchiveSuffix)
	result, errorValue := writeTheArchive(platform, request, companyID, archivePath, machine, progress)
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	result.Pruned, errorValue = pruneBackups(request.DirectoryPath, request.Keep)
	if errorValue != nil {
		return BackupResult{}, fmt.Errorf("%s was written, and the older backups could not be pruned: %w", archivePath, errorValue)
	}
	return result, nil
}

func writeTheArchive(platform backupPlatform, request BackupRequest, companyID string, archivePath string, machine Machine, progress io.Writer) (BackupResult, error) {
	major, errorValue := platform.DatabaseMajor(machine)
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	partialPath := filepath.Join(request.DirectoryPath, "."+filepath.Base(archivePath)+backupPartialExtension)
	writer, errorValue := hostbackup.Create(partialPath)
	if errorValue != nil {
		return BackupResult{}, errorValue
	}
	roots := hostFileRoots(platform.Layout())
	files, errorValue := writeBackupMembers(platform, writer, roots, request.DirectoryPath, machine, progress)
	if errorValue != nil {
		writer.Abandon()
		return BackupResult{}, errorValue
	}
	manifest, errorValue := writer.Finish(hostbackup.Manifest{
		CreatedAt:       request.Now.UTC(),
		PackageVersion:  PackageVersion,
		CompanyID:       companyID,
		PostgreSQLMajor: major,
		FileRoots:       rootRoles(roots),
	})
	if errorValue != nil {
		writer.Abandon()
		return BackupResult{}, errorValue
	}
	if errorValue := os.Rename(partialPath, archivePath); errorValue != nil {
		return BackupResult{}, errorValue
	}
	return BackupResult{ArchivePath: archivePath, Manifest: manifest, Files: files}, nil
}

func removeWhatAnInterruptedBackupLeft(directoryPath string) error {
	entries, errorValue := os.ReadDir(directoryPath)
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		name := entry.Name()
		isPartialArchive := strings.HasPrefix(name, "."+backupArchivePrefix) && strings.HasSuffix(name, backupPartialExtension)
		if !isPartialArchive && !strings.HasPrefix(name, backupScratchPattern) {
			continue
		}
		if errorValue := os.RemoveAll(filepath.Join(directoryPath, name)); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func writeBackupMembers(platform backupPlatform, writer *hostbackup.Writer, roots []hostbackup.FileRoot, directoryPath string, machine Machine, progress io.Writer) (hostbackup.FilesReport, error) {
	for _, database := range hostDatabases {
		fmt.Fprintf(progress, "Dumping the %s database…\n", database)
		if errorValue := writer.AddMember(hostbackup.DatabaseMemberName(database), func(output io.Writer) error {
			return platform.DumpDatabase(machine, database, output)
		}); errorValue != nil {
			return hostbackup.FilesReport{}, errorValue
		}
	}
	if errorValue := addTheRoster(writer, platform.Layout().PolicyDocumentPath(), progress); errorValue != nil {
		return hostbackup.FilesReport{}, errorValue
	}
	fmt.Fprintln(progress, "Archiving the company's files…")
	return addTheFiles(writer, roots, directoryPath)
}

func addTheRoster(writer *hostbackup.Writer, rosterPath string, progress io.Writer) error {
	roster, errorValue := os.Open(rosterPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		fmt.Fprintf(progress, "No roster at %s, so a restore will recreate no person's account until the agent syncs the roster again.\n", rosterPath)
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	defer roster.Close()
	return writer.AddMember(hostbackup.RosterMemberName, func(output io.Writer) error {
		_, errorValue := io.Copy(output, roster)
		return errorValue
	})
}

func addTheFiles(writer *hostbackup.Writer, roots []hostbackup.FileRoot, directoryPath string) (hostbackup.FilesReport, error) {
	scratchPath, errorValue := os.MkdirTemp(directoryPath, backupScratchPattern)
	if errorValue != nil {
		return hostbackup.FilesReport{}, errorValue
	}
	defer os.RemoveAll(scratchPath)
	var report hostbackup.FilesReport
	errorValue = writer.AddMember(hostbackup.FilesMemberName, func(output io.Writer) error {
		written, errorValue := hostbackup.WriteFiles(output, presentRoots(roots), hostbackup.SQLiteSnapshots{ScratchDirectoryPath: scratchPath})
		report = written
		return errorValue
	})
	return report, errorValue
}

func presentRoots(roots []hostbackup.FileRoot) []hostbackup.FileRoot {
	present := []hostbackup.FileRoot{}
	for _, root := range roots {
		if _, errorValue := os.Lstat(root.Path); errorValue == nil {
			present = append(present, root)
		}
	}
	return present
}

func rootRoles(roots []hostbackup.FileRoot) []string {
	roles := []string{}
	for _, root := range presentRoots(roots) {
		roles = append(roles, root.Role)
	}
	return roles
}

func installedCompanyID() (string, error) {
	companyDirectory, errorValue := filepath.EvalSymlinks(blueclaw.CompanyHostCurrentPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return "", nil
	}
	if errorValue != nil {
		return "", errorValue
	}
	if filepath.Dir(companyDirectory) != blueclaw.CompanyHostCompaniesRoot {
		return "", fmt.Errorf(
			"this company's directory is %s, outside %s, and a backup holds only what is under the host's own directories",
			companyDirectory, blueclaw.CompanyHostCompaniesRoot)
	}
	connection, errorValue := ReadConnection(filepath.Join(companyDirectory, connectionFileName))
	if errorValue != nil {
		return "", errorValue
	}
	return connection.Company.ID, nil
}

func prepareBackupDirectory(requested string) (string, error) {
	directoryPath := requested
	if directoryPath == "" {
		directoryPath = DefaultBackupDirectoryPath()
	}
	directoryPath, errorValue := filepath.Abs(directoryPath)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.MkdirAll(directoryPath, backupDirectoryMode); errorValue != nil {
		return "", errorValue
	}
	return directoryPath, os.Chmod(directoryPath, backupDirectoryMode)
}

func holdTheBackupLock(directoryPath string) (func(), error) {
	lock, errorValue := os.OpenFile(filepath.Join(directoryPath, backupLockFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); errorValue != nil {
		lock.Close()
		return nil, fmt.Errorf("another backup or restore is running on this computer; it holds %s", lock.Name())
	}
	return func() { lock.Close() }, nil
}

func pruneBackups(directoryPath string, keep int) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}
	archives, errorValue := backupArchivesIn(directoryPath)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(archives) <= keep {
		return nil, nil
	}
	pruned := []string{}
	for _, name := range archives[:len(archives)-keep] {
		path := filepath.Join(directoryPath, name)
		if errorValue := os.Remove(path); errorValue != nil {
			return pruned, errorValue
		}
		pruned = append(pruned, path)
	}
	return pruned, nil
}

func backupArchivesIn(directoryPath string) ([]string, error) {
	entries, errorValue := os.ReadDir(directoryPath)
	if errorValue != nil {
		return nil, errorValue
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() && backupArchiveNamePattern.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func DescribeSize(bytes int64) string {
	units := []string{"bytes", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unit := 0
	for size >= 1000 && unit < len(units)-1 {
		size /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", bytes, units[0])
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", size), ".0") + " " + units[unit]
}

const AdministrationStateRoot = "/root/.internkim"

const (
	stateRole          = "state"
	relayRole          = "relay"
	configurationRole  = "configuration"
	administrationRole = "administration"
	workspaceRole      = "workspace"
)

func hostFileRoots(layout blueclaw.CompanyHostLayout) []hostbackup.FileRoot {
	return []hostbackup.FileRoot{
		{Role: stateRole, Path: blueclaw.CompanyHostStateRoot, Excluded: []string{"media/.vgwlocks", "media/*/.sgwtmp"}},
		{Role: relayRole, Path: blueclaw.RelayStateDirectoryPath(blueclaw.CompanyHostRelayStateDirectoryName)},
		{Role: configurationRole, Path: blueclaw.CompanyHostConfigurationRoot},
		{Role: administrationRole, Path: AdministrationStateRoot},
		{Role: workspaceRole, Path: layout.WorkspacePath, Excluded: []string{"shared/cache", "private/people/*/tmp"}},
	}
}
