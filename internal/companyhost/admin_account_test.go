package companyhost

import (
	"context"
	"os/user"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type accountsOnAComputer struct {
	userIDs map[string]string
	calls   [][]string
	inputs  []string
}

func (computer *accountsOnAComputer) run(ctx context.Context, input string, name string, arguments ...string) error {
	computer.calls = append(computer.calls, append([]string{name}, arguments...))
	computer.inputs = append(computer.inputs, input)
	if name == "useradd" {
		computer.userIDs[arguments[len(arguments)-1]] = "1000"
	}
	return nil
}

func (computer *accountsOnAComputer) lookupUserID(name string) (string, error) {
	userID, isPresent := computer.userIDs[name]
	if !isPresent {
		return "", user.UnknownUserError(name)
	}
	return userID, nil
}

func adminAccountOn(t *testing.T, userIDs map[string]string) (AdminAccount, *accountsOnAComputer) {
	t.Helper()
	computer := &accountsOnAComputer{userIDs: userIDs}
	return AdminAccount{StateDirectoryPath: t.TempDir(), Run: computer.run, LookupUserID: computer.lookupUserID}, computer
}

func TestTheAdminAccountIsCreatedLockedWithNoRoot(t *testing.T) {
	account, computer := adminAccountOn(t, map[string]string{})
	isInternKims, errorValue := account.Ensure(context.Background())
	if errorValue != nil || !isInternKims {
		t.Fatalf("Ensure = %t, %v; want the account InternKim just made", isInternKims, errorValue)
	}
	wantedCalls := [][]string{
		{"useradd", "--create-home", "--shell", "/bin/bash", "--groups", "systemd-journal," + blueclaw.CompanyHostAdministratorGroup, AdminAccountName},
		{"passwd", "--lock", AdminAccountName},
	}
	if !slices.EqualFunc(computer.calls, wantedCalls, slices.Equal) {
		t.Fatalf("calls = %v, want %v", computer.calls, wantedCalls)
	}
}

func TestTheAdminAccountInternKimMadeIsKeptAcrossStarts(t *testing.T) {
	account, computer := adminAccountOn(t, map[string]string{})
	if _, errorValue := account.Ensure(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	computer.calls = nil
	isInternKims, errorValue := account.Ensure(context.Background())
	if errorValue != nil || !isInternKims || len(computer.calls) != 0 {
		t.Fatalf("a second start: Ensure = %t, %v, calls = %v", isInternKims, errorValue, computer.calls)
	}
}

func TestAnAdminAccountSomeoneElseMadeIsNeverTouched(t *testing.T) {
	account, computer := adminAccountOn(t, map[string]string{AdminAccountName: "1001"})
	isInternKims, errorValue := account.Ensure(context.Background())
	if errorValue != nil || isInternKims || len(computer.calls) != 0 {
		t.Fatalf("Ensure = %t, %v, calls = %v; want the account left alone", isInternKims, errorValue, computer.calls)
	}
}

func TestAnAdminAccountMadeAgainBySomeoneElseIsNotInternKims(t *testing.T) {
	account, computer := adminAccountOn(t, map[string]string{})
	if _, errorValue := account.Ensure(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	computer.userIDs[AdminAccountName] = "1002"
	if isInternKims, errorValue := account.Ensure(context.Background()); errorValue != nil || isInternKims {
		t.Fatalf("Ensure = %t, %v; want an admin with another user ID left alone", isInternKims, errorValue)
	}
}

func TestAdoptingTakesTheAdminAccountAKimMiniImageMade(t *testing.T) {
	account, computer := adminAccountOn(t, map[string]string{AdminAccountName: "1000"})
	if errorValue := account.Adopt(); errorValue != nil {
		t.Fatal(errorValue)
	}
	isInternKims, errorValue := account.Ensure(context.Background())
	if errorValue != nil || !isInternKims || len(computer.calls) != 0 {
		t.Fatalf("Ensure after adopting = %t, %v, calls = %v", isInternKims, errorValue, computer.calls)
	}
}

func TestAdoptingWithNoAdminAccountLeavesItToBeCreated(t *testing.T) {
	account, computer := adminAccountOn(t, map[string]string{})
	if errorValue := account.Adopt(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if isInternKims, errorValue := account.Ensure(context.Background()); errorValue != nil || !isInternKims || len(computer.calls) == 0 {
		t.Fatalf("Ensure = %t, %v, calls = %v; want the account created", isInternKims, errorValue, computer.calls)
	}
}

func TestTheAdminPasswordReachesChpasswdOnStandardInput(t *testing.T) {
	account, computer := adminAccountOn(t, map[string]string{})
	if errorValue := account.SetPassword(context.Background(), "a sample password"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(computer.calls[0], []string{"chpasswd"}) || computer.inputs[0] != AdminAccountName+":a sample password\n" {
		t.Fatalf("calls = %v, input = %q", computer.calls, computer.inputs)
	}
	if strings.Contains(strings.Join(computer.calls[0], " "), "sample") {
		t.Fatal("the password is on a command line, which every account can read")
	}
}
