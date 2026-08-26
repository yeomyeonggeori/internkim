package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func rosterPolicyWithEmails(emails ...string) map[string]any {
	people := []any{}
	for _, email := range emails {
		people = append(people, map[string]any{"personID": "person-" + email, "emails": []any{email}})
	}
	return map[string]any{"people": people}
}

func rosterPolicyEmails(policyDocument map[string]any) []string {
	people, _ := policyDocument["people"].([]any)
	var emails []string
	for _, value := range people {
		person, _ := value.(map[string]any)
		emails = append(emails, rosterPersonEmails(person)...)
	}
	return emails
}

func TestRosterReconcileAdoptsDirectoryRecords(t *testing.T) {
	policyDocument := rosterPolicyWithEmails("member@example.com")
	records := []adminUserMutation{
		{MemberID: "user-1", Email: "member@example.com", Name: "Member", Role: "member"},
		{MemberID: "user-2", Email: "Newcomer@Example.com", Name: "Newcomer", Role: "admin"},
	}

	reconcileRosterPeople(policyDocument, records, nil)

	emails := rosterPolicyEmails(policyDocument)
	if !slices.Contains(emails, "newcomer@example.com") {
		t.Fatalf("expected the new directory record on the roster, got %v", emails)
	}
	people, _ := policyDocument["people"].([]any)
	newcomer := blueclawPersonWithEmail(people, "newcomer@example.com")
	if newcomer["personID"] != "user-2" {
		t.Fatalf("expected the directory user ID as the person ID, got %v", newcomer["personID"])
	}
	if isAdmin, _ := newcomer["isAdmin"].(bool); !isAdmin {
		t.Fatalf("expected the directory admin role to carry onto the roster, got %v", newcomer)
	}
}

func TestRosterReconcileDropsPeopleTheDirectoryNoLongerKnows(t *testing.T) {
	policyDocument := rosterPolicyWithEmails("member@example.com", "departed@example.com")
	records := []adminUserMutation{{MemberID: "user-1", Email: "member@example.com", Role: "member"}}

	reconcileRosterPeople(policyDocument, records, nil)

	emails := rosterPolicyEmails(policyDocument)
	if slices.Contains(emails, "departed@example.com") {
		t.Fatalf("expected the departed person off the roster, got %v", emails)
	}
	if !slices.Contains(emails, "member@example.com") {
		t.Fatalf("expected the remaining person on the roster, got %v", emails)
	}
}

func TestRosterReconcileKeepsTheAdminEmailAndLocalTestPeople(t *testing.T) {
	policyDocument := rosterPolicyWithEmails("owner@example.com", "probe@internkim.test", "departed@example.com")
	records := []adminUserMutation{{MemberID: "user-1", Email: "member@example.com", Role: "member"}}

	reconcileRosterPeople(policyDocument, records, []string{"owner@example.com"})

	emails := rosterPolicyEmails(policyDocument)
	for _, retainedEmail := range []string{"owner@example.com", "probe@internkim.test"} {
		if !slices.Contains(emails, retainedEmail) {
			t.Fatalf("expected %q to survive the reconcile, got %v", retainedEmail, emails)
		}
	}
	if slices.Contains(emails, "departed@example.com") {
		t.Fatalf("expected the departed person off the roster, got %v", emails)
	}
}

func TestRosterReconcileKeepsAdminsAndRemovesNobodyOnAnEmptyDirectory(t *testing.T) {
	policyDocument := map[string]any{"people": []any{
		map[string]any{"personID": "person-1", "emails": []any{"boss@example.com"}, "isAdmin": true},
		map[string]any{"personID": "person-2", "emails": []any{"member@example.com"}},
	}}

	reconcileRosterPeople(policyDocument, nil, nil)

	emails := rosterPolicyEmails(policyDocument)
	if len(emails) != 2 {
		t.Fatalf("expected an empty directory answer to remove nobody, got %v", emails)
	}
}

func TestReconcileBlueclawRosterDeliversTheRosterFromTheHost(t *testing.T) {
	deviceDirectory := t.TempDir()
	deliveredPolicyPath := filepath.Join(deviceDirectory, "policy.json")
	service := NewService(Configuration{
		CentralPlaneAppURL:         "https://company.example.test",
		CentralPlaneProjectURL:     "https://project.example.test",
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeTestFile(t, "agent-key"),
		BlueclawBaseURL:            "http://blueclaw.local",
		BlueclawPolicyDeliveryPath: deliveredPolicyPath,
		AdminEmailPath:             writeTestFile(t, "owner@example.com"),
		StateDirectory:             t.TempDir(),
	})
	policyReloaded := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "https://company.example.test/api/agent/member":
			return jsonResponse(http.StatusOK, `{"members":[{"memberID":"user-1","email":"member@example.com","name":"Member","role":"member","status":"active","circles":["c-level"]}]}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"person-old","emails":["departed@example.com"]},{"personID":"person-owner","emails":["owner@example.com"]}]}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/policy/reload":
			policyReloaded = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.reconcileBlueclawRoster(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}

	if !policyReloaded {
		t.Fatal("expected the agent to be told to re-read the delivered roster")
	}
	document, readError := os.ReadFile(deliveredPolicyPath)
	if readError != nil {
		t.Fatalf("expected the host to write the roster: %v", readError)
	}
	var deliveredPolicy map[string]any
	if errorValue := json.Unmarshal(document, &deliveredPolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	emails := rosterPolicyEmails(deliveredPolicy)
	if !slices.Contains(emails, "member@example.com") || !slices.Contains(emails, "owner@example.com") {
		t.Fatalf("expected the directory member and the admin email on the delivered roster, got %v", emails)
	}
	if slices.Contains(emails, "departed@example.com") {
		t.Fatalf("expected the departed person off the delivered roster, got %v", emails)
	}
	people, _ := deliveredPolicy["people"].([]any)
	circles := policyStringList(blueclawPersonWithEmail(people, "member@example.com")["circles"])
	if !slices.Contains(circles, "c-level") {
		t.Fatalf("a circle the company keeps must reach the delivered roster, got %v", circles)
	}
}

func TestReconcileBlueclawRosterLeavesAnUnchangedRosterAlone(t *testing.T) {
	deliveredPolicyPath := filepath.Join(t.TempDir(), "policy.json")
	service := NewService(Configuration{
		CentralPlaneAppURL:         "https://company.example.test",
		CentralPlaneProjectURL:     "https://project.example.test",
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeTestFile(t, "agent-key"),
		BlueclawBaseURL:            "http://blueclaw.local",
		BlueclawPolicyDeliveryPath: deliveredPolicyPath,
		AdminEmailPath:             writeTestFile(t, "owner@example.com"),
		StateDirectory:             t.TempDir(),
	})
	settledPolicy := `{"people":[{"circles":["staff","c-level"],"displayName":"Member","emails":["member@example.com"],"grantedClasses":["internal"],"isAdmin":false,"personID":"user-1","securityLevelName":"member","securityLevelRank":10}]}`
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "https://company.example.test/api/agent/member":
			return jsonResponse(http.StatusOK, `{"members":[{"memberID":"user-1","email":"member@example.com","name":"Member","role":"member","status":"active","circles":["c-level"]}]}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, settledPolicy, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.reconcileBlueclawRoster(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, statError := os.Stat(deliveredPolicyPath); !os.IsNotExist(statError) {
		t.Fatalf("expected a settled roster to stay undelivered, got %v", statError)
	}
}

func TestRosterReconcileCarriesTheCirclesTheCompanyKeeps(t *testing.T) {
	policyDocument := rosterPolicyWithEmails("member@example.com")
	records := []adminUserMutation{
		{MemberID: "user-1", Email: "member@example.com", Name: "Member", Role: "member", Circles: []string{"staff", "c-level"}},
	}

	reconcileRosterPeople(policyDocument, records, nil)

	delivered, errorValue := json.Marshal(policyDocument)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var served map[string]any
	if errorValue := json.Unmarshal(delivered, &served); errorValue != nil {
		t.Fatal(errorValue)
	}
	people, _ := served["people"].([]any)
	circles := policyStringList(blueclawPersonWithEmail(people, "member@example.com")["circles"])
	if !slices.Contains(circles, "c-level") {
		t.Fatalf("a circle the company keeps must reach the roster, got %v", circles)
	}
}

func TestADirectoryRecordReadsTheCirclesFieldTheCompanySends(t *testing.T) {
	var record adminUserMutation
	if errorValue := json.Unmarshal([]byte(`{"email":"member@example.com","role":"member","circles":["staff","c-level"]}`), &record); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(record.Circles, []string{"staff", "c-level"}) {
		t.Fatalf("the field name the company sends must be the one this reads, got %v", record.Circles)
	}
}
