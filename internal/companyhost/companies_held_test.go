package companyhost

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	joiningCompanyID = "00000000-0000-4000-8000-0000000000c1"
	earlierCompanyID = "00000000-0000-4000-8000-0000000000e1"
)

func aCompaniesRootHolding(t *testing.T, companyIDs ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "companies")
	for _, companyID := range companyIDs {
		if errorValue := os.MkdirAll(filepath.Join(root, companyID, "secrets"), 0o700); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return root
}

func TestADirectoryOfAnotherCompanyIsDataOfAnotherCompanyWithNoCurrentLinkLeft(t *testing.T) {
	held, errorValue := companiesHeldIn(aCompaniesRootHolding(t, earlierCompanyID))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !holdsAnyOtherThan(held, joiningCompanyID) {
		t.Fatalf("a box holding %v is said to hold nothing of a company other than %s", held, joiningCompanyID)
	}
}

func TestOnlyTheJoiningCompanysDirectoryIsNoOtherCompanysData(t *testing.T) {
	held, errorValue := companiesHeldIn(aCompaniesRootHolding(t, joiningCompanyID))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if holdsAnyOtherThan(held, joiningCompanyID) {
		t.Fatalf("a box holding only %s is said to hold another company", joiningCompanyID)
	}
}

func TestABoxHoldingBothCompaniesHoldsAnother(t *testing.T) {
	held, errorValue := companiesHeldIn(aCompaniesRootHolding(t, joiningCompanyID, earlierCompanyID))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Contains(held, earlierCompanyID) || !holdsAnyOtherThan(held, joiningCompanyID) {
		t.Fatalf("a box holding %v is said to hold nothing of another company", held)
	}
}

func TestABoxWithNoCompaniesDirectoryHoldsNoCompany(t *testing.T) {
	held, errorValue := companiesHeldIn(filepath.Join(t.TempDir(), "companies"))

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(held) != 0 {
		t.Fatalf("a box with no companies directory holds %v", held)
	}
}
