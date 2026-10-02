package companyhost

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func exampleBackup(t *testing.T, manifest hostbackup.Manifest, members map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "internkim-backup-20261001T000000Z.tar")
	writer, errorValue := hostbackup.Create(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, name := range []string{hostbackup.DatabaseMemberName(blueclaw.BlueclawDatabaseName), hostbackup.FilesMemberName} {
		if errorValue := writer.AddMember(name, func(output io.Writer) error {
			_, errorValue := io.WriteString(output, members[name])
			return errorValue
		}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := writer.Finish(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	return archivePath
}

func restorableManifest() hostbackup.Manifest {
	return hostbackup.Manifest{
		CreatedAt:       time.Now(),
		CompanyID:       "00000000-0000-4000-8000-000000000001",
		PostgreSQLMajor: 17,
		FileRoots:       []string{stateRole, workspaceRole},
	}
}

func machineRunningPostgreSQL(major string) *recordedMachine {
	return &recordedMachine{answers: map[string]string{"runuser": major + "\n"}}
}

func refusalOf(t *testing.T, archivePath string, machine *recordedMachine) error {
	t.Helper()
	_, errorValue := restoreWhileLocked(linuxPlatform{}, RestoreRequest{ArchivePath: archivePath, Now: time.Now()}, t.TempDir(), machine, io.Discard)
	if errorValue == nil {
		t.Fatal("the restore went ahead")
	}
	for _, run := range machine.runs {
		if run[0] != "runuser" {
			t.Errorf("the restore ran %v before it refused", run)
		}
	}
	return errorValue
}

func TestATamperedMemberIsRefusedBeforeAnythingChanges(t *testing.T) {
	archivePath := exampleBackup(t, restorableManifest(), map[string]string{
		hostbackup.DatabaseMemberName(blueclaw.BlueclawDatabaseName): "the agent's ledger",
		hostbackup.FilesMemberName:                                   "files",
	})
	document, _ := os.ReadFile(archivePath)
	document[bytes.Index(document, []byte("ledger"))] ^= 0xFF
	if errorValue := os.WriteFile(archivePath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	errorValue := refusalOf(t, archivePath, machineRunningPostgreSQL("17"))
	if !strings.Contains(errorValue.Error(), "refused before anything changed") || !strings.Contains(errorValue.Error(), "blueclaw.dump") {
		t.Errorf("the refusal does not name the changed member: %v", errorValue)
	}
}

func TestABackupFromANewerFormatIsRefusedBeforeAnythingChanges(t *testing.T) {
	var held bytes.Buffer
	writer := tar.NewWriter(&held)
	document, _ := json.Marshal(map[string]any{"format": hostbackup.FormatName, "formatVersion": hostbackup.FormatVersion + 1})
	writer.WriteHeader(&tar.Header{Name: hostbackup.ManifestName, Mode: 0o600, Size: int64(len(document)), Typeflag: tar.TypeReg})
	writer.Write(document)
	writer.Close()
	archivePath := filepath.Join(t.TempDir(), "newer.tar")
	os.WriteFile(archivePath, held.Bytes(), 0o600)

	machine := machineRunningPostgreSQL("17")
	errorValue := refusalOf(t, archivePath, machine)
	if !strings.Contains(errorValue.Error(), "format 2") || len(machine.runs) != 0 {
		t.Errorf("the refusal was %v after running %v", errorValue, machine.runs)
	}
}

func TestABackupFromANewerPostgreSQLIsRefused(t *testing.T) {
	archivePath := exampleBackup(t, restorableManifest(), map[string]string{hostbackup.FilesMemberName: "files"})
	errorValue := refusalOf(t, archivePath, machineRunningPostgreSQL("16"))
	if !strings.Contains(errorValue.Error(), "PostgreSQL 17") || !strings.Contains(errorValue.Error(), "PostgreSQL 16") {
		t.Errorf("the refusal does not name both majors: %v", errorValue)
	}
}

func TestABackupWithoutTheCompanysKeysIsRefused(t *testing.T) {
	manifest := restorableManifest()
	manifest.FileRoots = []string{workspaceRole}
	archivePath := exampleBackup(t, manifest, map[string]string{hostbackup.FilesMemberName: "files"})
	if errorValue := refusalOf(t, archivePath, machineRunningPostgreSQL("17")); !strings.Contains(errorValue.Error(), "keys") {
		t.Errorf("the refusal was %v", errorValue)
	}
}

func TestABackupMadeByANewerPackageIsRefused(t *testing.T) {
	if errorValue := refuseANewerPackage("2026.10.02.000000", "2026.10.01.235959"); errorValue == nil {
		t.Error("a backup from a newer package was accepted")
	}
	if errorValue := refuseANewerPackage("2026.10.01.000000", "2026.10.01.000000"); errorValue != nil {
		t.Errorf("a backup from the same package was refused: %v", errorValue)
	}
	if errorValue := refuseANewerPackage("2026.09.30.120000", "2026.10.01.000000"); errorValue != nil {
		t.Errorf("a backup from an older package was refused: %v", errorValue)
	}
	if errorValue := refuseANewerPackage("2026.10.02.000000", ""); errorValue != nil {
		t.Errorf("a development build with no version refused a backup: %v", errorValue)
	}
}

func TestPruningKeepsTheNewestBackupsAndNothingElseIsTouched(t *testing.T) {
	directory := t.TempDir()
	stamps := []string{"20260925T030000Z", "20260926T030000Z", "20260927T030000Z", "20260928T030000Z", "20260929T030000Z"}
	for _, stamp := range stamps {
		os.WriteFile(filepath.Join(directory, backupArchivePrefix+stamp+backupArchiveSuffix), []byte("x"), 0o600)
	}
	keptByHand := []string{"before-the-upgrade.tar", ".internkim-backup-20260930T030000Z.tar.partial", backupLockFileName}
	for _, name := range keptByHand {
		os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o600)
	}
	pruned, errorValue := pruneBackups(directory, 3)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(pruned) != 2 || !strings.HasSuffix(pruned[0], stamps[0]+backupArchiveSuffix) || !strings.HasSuffix(pruned[1], stamps[1]+backupArchiveSuffix) {
		t.Errorf("pruned %v, not the two oldest", pruned)
	}
	remaining, _ := backupArchivesIn(directory)
	if len(remaining) != 3 || remaining[0] != backupArchivePrefix+stamps[2]+backupArchiveSuffix {
		t.Errorf("kept %v", remaining)
	}
	for _, name := range keptByHand {
		if _, errorValue := os.Stat(filepath.Join(directory, name)); errorValue != nil {
			t.Errorf("pruning removed %s, which it did not write", name)
		}
	}
}

func TestKeepingZeroPrunesNothing(t *testing.T) {
	directory := t.TempDir()
	os.WriteFile(filepath.Join(directory, backupArchivePrefix+"20260925T030000Z"+backupArchiveSuffix), []byte("x"), 0o600)
	if pruned, _ := pruneBackups(directory, 0); len(pruned) != 0 {
		t.Errorf("keep 0 pruned %v", pruned)
	}
}

func TestEveryRootABackupTakesIsOneARestoreCanPlace(t *testing.T) {
	roles := map[string]bool{}
	for _, root := range hostFileRoots(blueclaw.LinuxCompanyHostLayout()) {
		if roles[root.Role] {
			t.Errorf("two roots are called %s", root.Role)
		}
		roles[root.Role] = true
		if strings.HasPrefix(blueclaw.CompanyHostBackupsPath+"/", strings.TrimRight(root.Path, "/")+"/") {
			t.Errorf("%s holds the backups directory, so every backup would carry the ones before it", root.Path)
		}
	}
	if !roles[stateRole] {
		t.Error("no root holds the company's keys")
	}
}

func TestABackupClearsWhatAnInterruptedOneLeftAndNothingElse(t *testing.T) {
	directory := t.TempDir()
	finished := backupArchivePrefix + "20260929T030000Z" + backupArchiveSuffix
	interrupted := "." + backupArchivePrefix + "20260930T030000Z" + backupArchiveSuffix + backupPartialExtension
	for _, name := range []string{finished, interrupted, "before-the-upgrade.tar"} {
		os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o600)
	}
	os.MkdirAll(filepath.Join(directory, backupScratchPattern+"123", "nested"), 0o700)
	if errorValue := removeWhatAnInterruptedBackupLeft(directory); errorValue != nil {
		t.Fatal(errorValue)
	}
	entries, _ := os.ReadDir(directory)
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if strings.Join(names, " ") != "before-the-upgrade.tar "+finished {
		t.Errorf("left %v", names)
	}
}

func TestEveryCommandRunAsAnotherAccountStartsFromANeutralDirectory(t *testing.T) {
	machine := &recordedMachine{
		answers: map[string]string{"runuser": "15\n"},
		printed: map[string]string{"runuser": "15|t\n"},
	}
	platform := linuxPlatform{}
	if _, errorValue := platform.DatabaseMajor(machine); errorValue != nil {
		t.Fatalf("read the major: %v", errorValue)
	}
	platform.DumpDatabase(machine, blueclaw.BlueclawDatabaseName, io.Discard)
	platform.RestoreDatabase(machine, blueclaw.BlueclawDatabaseName, strings.NewReader(""))
	platform.OwnersInDump(machine, strings.NewReader(""))
	if errorValue := prepareDatabases(platform, machine, companyHostSettings{DatabasePassword: "password"}, io.Discard); errorValue != nil {
		t.Fatalf("prepare the databases: %v", errorValue)
	}
	asAnotherAccount := 0
	for _, run := range machine.runs {
		if run[0] != "runuser" {
			continue
		}
		asAnotherAccount++
		separator := slices.Index(run, "--")
		if separator < 0 || !slices.Equal(run[separator+1:separator+3], []string{"env", "--chdir=/"}) {
			t.Errorf("%v runs from whatever directory the person ran internkim in", run)
		}
	}
	if asAnotherAccount < 6 {
		t.Fatalf("only %d commands ran as another account: %v", asAnotherAccount, machine.runs)
	}
}
