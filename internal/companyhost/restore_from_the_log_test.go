package companyhost

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func aKeptLog(major int, release string) RestoreRequest {
	return RestoreRequest{
		Log: KeptLog{
			CompanyID:       "00000000-0000-4000-8000-000000000001",
			BaseBackupName:  "internkim-base-20261122T030000Z.json",
			TakenAt:         time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC),
			PackageVersion:  release,
			PostgreSQLMajor: major,
		},
		RebuildFromWAL: func(string) error { return nil },
		ReplayFiles:    func(FileJournalApplier) (int, error) { return 0, nil },
		Now:            time.Now(),
	}
}

func refusalOfTheLog(t *testing.T, request RestoreRequest, machine *recordedMachine) error {
	t.Helper()
	_, errorValue := restoreWhileLocked(linuxPlatform{}, request, t.TempDir(), machine, io.Discard)
	if errorValue == nil {
		t.Fatal("the take-over from the log went ahead")
	}
	for _, run := range machine.runs {
		if run[0] != "runuser" {
			t.Errorf("the take-over from the log ran %v before it refused", run)
		}
	}
	return errorValue
}

func TestATakeOverFromTheLogOnAnotherPostgreSQLMajorIsRefusedBeforeAnythingChanges(t *testing.T) {
	errorValue := refusalOfTheLog(t, aKeptLog(17, ""), machineRunningPostgreSQL("16"))

	for _, wanted := range []string{"PostgreSQL 17", "PostgreSQL 16", "install PostgreSQL 17"} {
		if !strings.Contains(errorValue.Error(), wanted) {
			t.Errorf("the refusal lacks %q: %v", wanted, errorValue)
		}
	}
}

func TestATakeOverFromALogANewerReleaseKeptIsRefusedBeforeAnythingChanges(t *testing.T) {
	installed := PackageVersion
	PackageVersion = "1:0.0.1"
	t.Cleanup(func() { PackageVersion = installed })

	errorValue := refusalOfTheLog(t, aKeptLog(17, "1:0.0.2"), machineRunningPostgreSQL("17"))

	if !strings.Contains(errorValue.Error(), "made by internkim 1:0.0.2") {
		t.Errorf("the refusal does not name the release that kept the log: %v", errorValue)
	}
}

func TestATakeOverFromABaseBackupThatNamesNoMajorIsRefusedBeforeAnythingChanges(t *testing.T) {
	machine := machineRunningPostgreSQL("17")

	errorValue := refusalOfTheLog(t, aKeptLog(0, ""), machine)

	if !strings.Contains(errorValue.Error(), "internkim-base-20261122T030000Z.json") || len(machine.runs) != 0 {
		t.Errorf("the refusal was %v after running %v", errorValue, machine.runs)
	}
}

func TestARestoreNamesAnArchiveOrTheMeansToRestoreFromTheLog(t *testing.T) {
	request := aKeptLog(17, "")
	request.ReplayFiles = nil

	errorValue := refusalOfTheLog(t, request, machineRunningPostgreSQL("17"))

	if !strings.Contains(errorValue.Error(), "replaying its file journal") {
		t.Errorf("a restore with no archive and no file journal to replay was refused with %v", errorValue)
	}
}

func TestTheRosterIsInARootTheFileJournalCarries(t *testing.T) {
	layout := blueclaw.LinuxCompanyHostLayout()
	rosterPath := layout.PolicyDocumentPath()
	for _, root := range JournaledFileRoots(layout, ".file-journal-barrier") {
		relative, errorValue := filepath.Rel(root.Path, rosterPath)
		if errorValue != nil || strings.HasPrefix(relative, "..") {
			continue
		}
		if root.Excludes(filepath.ToSlash(relative)) {
			t.Fatalf("%s is in the %s root and the file journal excludes it", rosterPath, root.Role)
		}
		return
	}
	t.Fatalf("no root the file journal carries holds %s, so a take-over from the log recreates nobody's account", rosterPath)
}
