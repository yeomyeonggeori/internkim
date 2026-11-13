package capabilityd

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

type personReferenceCases struct {
	Companies map[string][]struct {
		PersonID string `json:"personID"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		JobTitle string `json:"jobTitle"`
	} `json:"companies"`
	Cases []struct {
		Company    string   `json:"company"`
		Hint       string   `json:"hint"`
		Outcome    string   `json:"outcome"`
		Match      string   `json:"match"`
		Candidates []string `json:"candidates"`
	} `json:"cases"`
}

func TestEveryPersonReferenceNamesWhoTheSharedCasesSay(t *testing.T) {
	document, errorValue := os.ReadFile("../../web/src/lib/person-reference-cases.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var shared personReferenceCases
	if errorValue := json.Unmarshal(document, &shared); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(shared.Cases) == 0 {
		t.Fatal("the shared cases are what keeps the two ladders the same")
	}
	for _, testCase := range shared.Cases {
		resolution := resolveHint(testCase.Hint, directoryPeopleOfCompany(shared, testCase.Company), nil)
		if string(resolution.Outcome) != testCase.Outcome {
			t.Errorf("%q in %s: outcome %s, want %s", testCase.Hint, testCase.Company, resolution.Outcome, testCase.Outcome)
			continue
		}
		if resolution.Outcome == hintResolved && resolution.Match.MemberID != testCase.Match {
			t.Errorf("%q in %s: resolved %s, want %s", testCase.Hint, testCase.Company, resolution.Match.MemberID, testCase.Match)
		}
		if resolution.Outcome != hintResolved && !sameMemberIDs(resolution.Candidates, testCase.Candidates) {
			t.Errorf("%q in %s: candidates %+v, want %v", testCase.Hint, testCase.Company, resolution.Candidates, testCase.Candidates)
		}
	}
}

func directoryPeopleOfCompany(shared personReferenceCases, company string) []directoryPerson {
	people := []directoryPerson{}
	for _, person := range shared.Companies[company] {
		handle, _, _ := strings.Cut(person.Email, "@")
		people = append(people, directoryPerson{
			MemberID:  person.PersonID,
			Email:     person.Email,
			Name:      person.Name,
			JobTitle:  person.JobTitle,
			Messenger: map[string]string{"chat": handle},
		})
	}
	return people
}

func sameMemberIDs(people []directoryPerson, memberIDs []string) bool {
	held := []string{}
	for _, person := range people {
		held = append(held, person.MemberID)
	}
	return slices.Equal(held, memberIDs)
}
