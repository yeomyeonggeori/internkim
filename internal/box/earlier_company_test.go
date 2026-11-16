package box

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const earlierCompanyID = "00000000-0000-4000-8000-00000000000e"

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
	asked := ""
	daemon.HoldsAnotherCompany = func(companyID string) (bool, error) {
		asked = companyID
		return true, nil
	}
	forgotten := &forgetting{}
	if errorValue := os.MkdirAll(places.StateDirectoryPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(places.StateDirectoryPath, serviceLeaseFileName), []byte("held\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := forgotten.watching(daemon, installed).InstallWithConnectionFile(context.Background(), "connection-file-key", "sk-or-v1-typed"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if asked != sampleCompanyID || forgotten.times != 1 || forgotten.installsBefore != 0 || len(installed.requests) != 1 {
		t.Fatalf("asked about %q, emptied %d times with %d installs before it, installs = %d", asked, forgotten.times, forgotten.installsBefore, len(installed.requests))
	}
	if _, errorValue := os.Stat(filepath.Join(places.StateDirectoryPath, serviceLeaseFileName)); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("the earlier company's service lease is still held: %v", errorValue)
	}
}
