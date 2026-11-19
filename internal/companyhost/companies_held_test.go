package companyhost

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	joiningCompanyID = "00000000-0000-4000-8000-0000000000c1"
	earlierCompanyID = "00000000-0000-4000-8000-0000000000e1"
)

func aCompaniesRootHolding(t *testing.T, names ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "companies")
	for _, name := range names {
		if errorValue := os.MkdirAll(filepath.Join(root, name, "secrets"), 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return root
}

func TestADirectoryOfAnotherCompanyIsAnotherCompanyWithNoCurrentLinkLeft(t *testing.T) {
	directories, _, errorValue := companyDirectoriesIn(aCompaniesRootHolding(t, earlierCompanyID))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	held := CompaniesHeld{Directories: directories}
	if !slices.Equal(held.OtherThan(joiningCompanyID), []string{earlierCompanyID}) {
		t.Fatalf("a box holding %v holds %v other than %s", directories, held.OtherThan(joiningCompanyID), joiningCompanyID)
	}
}

func TestOnlyTheJoiningCompanysDirectoryIsNoOtherCompany(t *testing.T) {
	directories, _, errorValue := companyDirectoriesIn(aCompaniesRootHolding(t, joiningCompanyID))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if others := (CompaniesHeld{Current: joiningCompanyID, Directories: directories}).OtherThan(joiningCompanyID); len(others) != 0 {
		t.Fatalf("a box holding only %s holds %v", joiningCompanyID, others)
	}
}

func TestADirectoryNotNamedByACompanyIdIsNoCompany(t *testing.T) {
	names := []string{"lost+found", "tmp.x81", earlierCompanyID + ".copy", "00000000000040008000000000000e1"}
	directories, unrecognized, errorValue := companyDirectoriesIn(aCompaniesRootHolding(t, names...))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(directories) != 0 || len(unrecognized) != len(names) {
		t.Fatalf("companies %v and unrecognized %v; a name that is not a company id in its written form is no company", directories, unrecognized)
	}
}

func TestABoxWithNoCompaniesDirectoryHoldsNoCompany(t *testing.T) {
	directories, unrecognized, errorValue := companyDirectoriesIn(filepath.Join(t.TempDir(), "companies"))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(directories) != 0 || len(unrecognized) != 0 {
		t.Fatalf("a box with no companies directory holds %v and %v", directories, unrecognized)
	}
}

func TestForgettingAnotherCompanysDirectoryLeavesTheServedCompanyAndEverythingElse(t *testing.T) {
	root := aCompaniesRootHolding(t, joiningCompanyID, earlierCompanyID, "lost+found")

	if errorValue := forgetCompanyDirectoryIn(root, joiningCompanyID, earlierCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := os.Stat(filepath.Join(root, earlierCompanyID)); !errors.Is(errorValue, fs.ErrNotExist) {
		t.Fatalf("the other company's directory is still there: %v", errorValue)
	}
	for _, kept := range []string{joiningCompanyID, "lost+found"} {
		if _, errorValue := os.Stat(filepath.Join(root, kept, "secrets")); errorValue != nil {
			t.Fatalf("%s went with the other company's directory: %v", kept, errorValue)
		}
	}
}

func TestNoDirectoryIsForgottenForTheServedCompanyOrANameThatIsNoCompanyId(t *testing.T) {
	root := aCompaniesRootHolding(t, joiningCompanyID, "lost+found")

	for _, refused := range []string{joiningCompanyID, "lost+found", "../" + joiningCompanyID} {
		if errorValue := forgetCompanyDirectoryIn(root, joiningCompanyID, refused); errorValue == nil {
			t.Fatalf("removing %q was allowed", refused)
		}
	}
	for _, kept := range []string{joiningCompanyID, "lost+found"} {
		if _, errorValue := os.Stat(filepath.Join(root, kept)); errorValue != nil {
			t.Fatalf("%s was removed: %v", kept, errorValue)
		}
	}
}
