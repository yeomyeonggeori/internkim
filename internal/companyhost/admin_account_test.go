package companyhost

import (
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const sampleAdministratorGroupID = "991"

func directoryWith(accounts map[string][]string) AccountDirectory {
	return AccountDirectory{
		LookupAccount: func(name string) (*user.User, error) {
			if _, isPresent := accounts[name]; !isPresent {
				return nil, user.UnknownUserError(name)
			}
			return &user.User{Username: name}, nil
		},
		LookupGroup: func(name string) (*user.Group, error) {
			if name != blueclaw.CompanyHostAdministratorGroup {
				return nil, user.UnknownGroupError(name)
			}
			return &user.Group{Name: name, Gid: sampleAdministratorGroupID}, nil
		},
		GroupIDsOf: func(account *user.User) ([]string, error) {
			return accounts[account.Username], nil
		},
	}
}

func TestOnlyAnAdminInTheAdministratorGroupIsTheCompanysAdminAccount(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		accounts map[string][]string
		want     bool
	}{
		{name: "a Kim mini's admin", accounts: map[string][]string{AdminAccountName: {"1000", sampleAdministratorGroupID}}, want: true},
		{name: "an admin someone made outside the group", accounts: map[string][]string{AdminAccountName: {"1000", "27"}}, want: false},
		{name: "a computer with no admin", accounts: map[string][]string{"operator": {"1000", sampleAdministratorGroupID}}, want: false},
	} {
		if got := directoryWith(testCase.accounts).HasAdminAccount(); got != testCase.want {
			t.Errorf("%s: HasAdminAccount = %t, want %t", testCase.name, got, testCase.want)
		}
	}
}

func TestAComputerWithoutTheAdministratorGroupHasNoAdminAccount(t *testing.T) {
	directory := directoryWith(map[string][]string{AdminAccountName: {"1000"}})
	directory.LookupGroup = func(name string) (*user.Group, error) { return nil, user.UnknownGroupError(name) }
	if directory.HasAdminAccount() {
		t.Fatal("an admin on a computer without the package's administrator group was taken as the company's")
	}
}

func TestTheKimMiniImageMakesTheAdminAccountThisDetects(t *testing.T) {
	assemble, errorValue := os.ReadFile(filepath.Join("..", "..", "tools", "kim-mini-image", "assemble"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	madeInTheGroup := regexp.MustCompile(`useradd [^\n]*--groups "[^"]*\$administratorGroup[^"]*" ` + regexp.QuoteMeta(AdminAccountName) + `\n`)
	if !madeInTheGroup.Match(assemble) {
		t.Fatalf("tools/kim-mini-image/assemble no longer makes %s in the package's administrator group, which is how a box finds its admin account", AdminAccountName)
	}
}
