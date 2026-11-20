package box

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

const earlierCompanyID = "00000000-0000-4000-8000-00000000000e"

func holding(current string, directories ...string) func() (companyhost.CompaniesHeld, error) {
	return func() (companyhost.CompaniesHeld, error) {
		return companyhost.CompaniesHeld{Current: current, Directories: directories}, nil
	}
}

type directoryRemovals struct {
	companyIDs []string
}

func (removals *directoryRemovals) watching(daemon Daemon) Daemon {
	daemon.ForgetCompanyDirectory = func(companyID string) error {
		removals.companyIDs = append(removals.companyIDs, companyID)
		return nil
	}
	return daemon
}

type forgetting struct {
	times          int
	installsBefore int
	refusal        error
}

func (recorded *forgetting) watching(daemon Daemon, installed *installs) Daemon {
	daemon.ForgetEveryCompany = func() error {
		recorded.times++
		recorded.installsBefore = len(installed.requests)
		return recorded.refusal
	}
	return daemon
}

func holdTheKeyOf(t *testing.T, places Places, companyID string) {
	t.Helper()
	identity := identityOf(t, places)
	if errorValue := storeCompanyKey(places.StateDirectoryPath, companyID, identity.encryptionKey); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestABoxClaimedByAnotherCompanyIsEmptiedBeforeItInstalls(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	installed := &installs{}
	daemon, places := daemonFor(t, plane, installed)
	holdTheKeyOf(t, places, earlierCompanyID)
	if errorValue := MarkInstalledCompany(places.StateDirectoryPath, earlierCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	forgotten := &forgetting{}
	locks := 0
	daemon.LockAdminPassword = func(context.Context) error {
		locks++
		return nil
	}

	runSteps(t, forgotten.watching(daemon, installed), 1)

	if locks != 1 {
		t.Fatalf("the admin password was locked %d times; the earlier company's administrators must not log in to the next company's box", locks)
	}
	if forgotten.times != 1 || forgotten.installsBefore != 0 {
		t.Fatalf("emptied %d times with %d installs before it; the earlier company goes once, before the new one is installed", forgotten.times, forgotten.installsBefore)
	}
	if len(installed.requests) != 1 || installed.requests[0].Connection.Company.ID != sampleCompanyID {
		t.Fatalf("installed %+v", installed.requests)
	}
	owner, errorValue := companyKeyOwner(places.StateDirectoryPath)
	if errorValue != nil || owner != sampleCompanyID {
		t.Fatalf("the company key belongs to %q (%v); the earlier company's key must not outlive its data", owner, errorValue)
	}
}

func TestABoxItsOwnCompanyClaimsAgainKeepsWhatItHolds(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	installed := &installs{}
	daemon, places := daemonFor(t, plane, installed)
	holdTheKeyOf(t, places, sampleCompanyID)
	forgotten := &forgetting{}

	runSteps(t, forgotten.watching(daemon, installed), 2)

	if forgotten.times != 0 {
		t.Fatalf("emptied %d times; a company connecting its own box again loses nothing", forgotten.times)
	}
	if len(installed.requests) != 1 {
		t.Fatalf("installs = %d", len(installed.requests))
	}
}

func TestABoxThatCannotBeEmptiedInstallsNothingAndKeepsAsking(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	installed := &installs{}
	daemon, places := daemonFor(t, plane, installed)
	holdTheKeyOf(t, places, earlierCompanyID)
	forgotten := &forgetting{refusal: errors.New("user bc_person_one is currently used by process 42")}

	runSteps(t, forgotten.watching(daemon, installed), 2)

	if forgotten.times != 2 || len(installed.requests) != 0 {
		t.Fatalf("emptied %d times, installs = %d; a new company is never installed beside an earlier one's data", forgotten.times, len(installed.requests))
	}
	owner, errorValue := companyKeyOwner(places.StateDirectoryPath)
	if errorValue != nil || owner != earlierCompanyID {
		t.Fatalf("the company key belongs to %q (%v); the record of whose data this is stays until the data is gone", owner, errorValue)
	}
}

func TestAConnectionFileForAnotherCompanyEmptiesTheComputerFirst(t *testing.T) {
	plane := &fakePlane{accessToken: "file.session.token"}
	installed := &installs{}
	daemon, places := daemonFor(t, plane, installed)
	daemon.CompaniesHeld = holding("", earlierCompanyID)
	forgotten := &forgetting{}
	if errorValue := os.MkdirAll(places.StateDirectoryPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(places.StateDirectoryPath, serviceLeaseFileName), []byte("held\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := forgotten.watching(daemon, installed).InstallWithConnectionFile(context.Background(), "connection-file-key", "sk-or-v1-typed", askNobodyForAModelKey); errorValue != nil {
		t.Fatal(errorValue)
	}

	if forgotten.times != 1 || forgotten.installsBefore != 0 || len(installed.requests) != 1 {
		t.Fatalf("emptied %d times with %d installs before it, installs = %d", forgotten.times, forgotten.installsBefore, len(installed.requests))
	}
	if _, errorValue := os.Stat(filepath.Join(places.StateDirectoryPath, serviceLeaseFileName)); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("the earlier company's service lease is still held: %v", errorValue)
	}
}

func TestAConnectionFileForTheCompanyThisComputerServesKeepsItsData(t *testing.T) {
	plane := &fakePlane{accessToken: "file.session.token"}
	installed := &installs{}
	daemon, places := daemonFor(t, plane, installed)
	daemon.CompaniesHeld = holding(sampleCompanyID, sampleCompanyID)
	removals := &directoryRemovals{}
	forgotten := &forgetting{}

	if _, errorValue := removals.watching(forgotten.watching(daemon, installed)).InstallWithConnectionFile(context.Background(), "connection-file-key", "", askNobodyForAModelKey); errorValue != nil {
		t.Fatal(errorValue)
	}

	if forgotten.times != 0 || len(removals.companyIDs) != 0 {
		t.Fatalf("emptied %d times and removed directories %v of the company this computer serves", forgotten.times, removals.companyIDs)
	}
	if len(installed.requests) != 1 || installed.requests[0].StateDirectoryPath != places.CompanyStateDirectoryPath(sampleCompanyID) {
		t.Fatalf("installed %+v; the install runs over the directory the company already has", installed.requests)
	}
}

func TestABoxThatHoldsOnlyAnotherCompanysDirectoryIsEmptiedAndForgetsItsTakeOverNotes(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	installed := &installs{}
	daemon, places := daemonFor(t, plane, installed)
	daemon.CompaniesHeld = holding("", earlierCompanyID)
	daemon.Places.SeveralBoxesPath = filepath.Join(places.StateDirectoryPath, "several-boxes")
	for _, path := range []string{filepath.Join(places.StateDirectoryPath, TakeOverRetryFileName), daemon.Places.SeveralBoxesPath} {
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := os.WriteFile(path, []byte("{}"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	forgotten := &forgetting{}

	runSteps(t, forgotten.watching(daemon, installed), 1)

	if forgotten.times != 1 {
		t.Fatalf("a box holding another company's directory without its key or current link was emptied %d times", forgotten.times)
	}
	if _, errorValue := os.Stat(filepath.Join(places.StateDirectoryPath, TakeOverRetryFileName)); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatal("the earlier company's take-over record outlived its data")
	}
}

func TestAServingBoxKeepsItsCompanysDataAndLosesOnlyAStaleDirectoryOfAnother(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	installed := &installs{}
	daemon, places := daemonFor(t, plane, installed)
	if errorValue := os.MkdirAll(places.StateDirectoryPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := MarkInstalledCompany(places.StateDirectoryPath, sampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	daemon.CompaniesHeld = holding(sampleCompanyID, sampleCompanyID, earlierCompanyID)
	forgotten, removed := &forgetting{}, &directoryRemovals{}

	runSteps(t, removed.watching(forgotten.watching(daemon, installed)), 2)

	if forgotten.times != 0 {
		t.Fatalf("a box serving its company was emptied %d times because another company's directory was found", forgotten.times)
	}
	if !slices.Equal(removed.companyIDs, []string{earlierCompanyID, earlierCompanyID}) {
		t.Fatalf("removed the directories of %v; only the other company's goes, on each step it is found", removed.companyIDs)
	}
}

func TestAServingBoxWhoseCurrentLinkNamesItsCompanyIsNotEmptiedBeforeItIsMarked(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	installed := &installs{}
	daemon, _ := daemonFor(t, plane, installed)
	daemon.CompaniesHeld = holding(sampleCompanyID, sampleCompanyID, earlierCompanyID)
	forgotten, removed := &forgetting{}, &directoryRemovals{}

	runSteps(t, removed.watching(forgotten.watching(daemon, installed)), 1)

	if forgotten.times != 0 || !slices.Equal(removed.companyIDs, []string{earlierCompanyID}) {
		t.Fatalf("emptied %d times and removed %v; a box already restored for its company keeps the shared places", forgotten.times, removed.companyIDs)
	}
}

func TestNothingIsEmptiedWhileATakeOverIsUnderway(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	installed := &installs{}
	daemon, _ := daemonFor(t, plane, installed)
	daemon.CompaniesHeld = holding("", earlierCompanyID)
	daemon.TakeOverIsUnderway = func() bool { return true }
	forgotten, removed := &forgetting{}, &directoryRemovals{}

	runSteps(t, removed.watching(forgotten.watching(daemon, installed)), 1)

	if forgotten.times != 0 || len(removed.companyIDs) != 0 {
		t.Fatalf("emptied %d times and removed %v while a take-over was restoring the company", forgotten.times, removed.companyIDs)
	}
}

func TestAStandbyHoldingItsOwnCompanysDirectoryKeepsIt(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token", role: RoleStandby}
	installed := &installs{}
	daemon, _ := daemonFor(t, plane, installed)
	daemon.CompaniesHeld = holding("", sampleCompanyID)
	forgotten, removed := &forgetting{}, &directoryRemovals{}

	runSteps(t, removed.watching(forgotten.watching(daemon, installed)), 1)

	if forgotten.times != 0 || len(removed.companyIDs) != 0 {
		t.Fatalf("a standby of its own company was emptied %d times and lost %v", forgotten.times, removed.companyIDs)
	}
}
