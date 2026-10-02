package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
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

	reconcileRosterPeople(policyDocument, records, nil, workspaceLanguageKorean)

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

	reconcileRosterPeople(policyDocument, records, nil, workspaceLanguageKorean)

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

	reconcileRosterPeople(policyDocument, records, []string{"owner@example.com"}, workspaceLanguageKorean)

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

func TestADirectoryThatAnsweredNothingRemovesNobody(t *testing.T) {
	policyDocument := map[string]any{"people": []any{
		map[string]any{"personID": "person-1", "emails": []any{"boss@example.com"}, "isAdmin": true},
		map[string]any{"personID": "person-2", "emails": []any{"member@example.com"}},
	}}

	reconcileRosterPeople(policyDocument, nil, nil, workspaceLanguageKorean)

	emails := rosterPolicyEmails(policyDocument)
	if len(emails) != 2 {
		t.Fatalf("a directory that answered nothing is a directory nobody heard from, got %v", emails)
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
		case request.Method == http.MethodGet && request.URL.String() == "https://company.example.test/api/agent/company":
			return jsonResponse(http.StatusOK, `{"company":{"name":"예시회사","profileImage":"","timezone":"Asia/Seoul"}}`, nil), nil
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
	settledPolicy := `{"company":{"brandName":"","description":"","locale":"ko","name":"","representative":"","slogan":"","timeZone":"Asia/Seoul","website":""},"people":[{"circles":["member","c-level"],"displayName":"Member","emails":["member@example.com"],"grantedClasses":["internal"],"isAdmin":false,"personID":"user-1","securityLevelName":"member","securityLevelRank":10}]}`
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "https://company.example.test/api/agent/company":
			return jsonResponse(http.StatusOK, `{"company":{"name":"예시회사","profileImage":"","timezone":"Asia/Seoul"}}`, nil), nil
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
		{MemberID: "user-1", Email: "member@example.com", Name: "Member", Role: "member", Circles: []string{"member", "c-level"}},
	}

	reconcileRosterPeople(policyDocument, records, nil, workspaceLanguageKorean)

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

func TestRosterReconcileRendersNamesForTheWorkspaceLanguage(t *testing.T) {
	koreanPolicy := rosterPolicyWithEmails("korean@example.com")
	reconcileRosterPeople(koreanPolicy, []adminUserMutation{{MemberID: "korean", Email: "korean@example.com", Name: "샘플 이", Role: "member"}}, nil, workspaceLanguageKorean)
	koreanPeople, _ := koreanPolicy["people"].([]any)
	if displayName := blueclawPersonWithEmail(koreanPeople, "korean@example.com")["displayName"]; displayName != "이샘플" {
		t.Fatalf("expected Korean display name to be rendered, got %v", displayName)
	}

	englishPolicy := rosterPolicyWithEmails("english@example.com")
	reconcileRosterPeople(englishPolicy, []adminUserMutation{{MemberID: "english", Email: "english@example.com", Name: "샘플 이", Role: "member"}}, nil, workspaceLanguageEnglish)
	englishPeople, _ := englishPolicy["people"].([]any)
	if displayName := blueclawPersonWithEmail(englishPeople, "english@example.com")["displayName"]; displayName != "샘플 이" {
		t.Fatalf("expected English workspace to preserve the recorded Korean name, got %v", displayName)
	}

	latinPolicy := rosterPolicyWithEmails("latin@example.com")
	reconcileRosterPeople(latinPolicy, []adminUserMutation{{MemberID: "latin", Email: "latin@example.com", Name: "Sample Lee", Role: "member"}}, nil, workspaceLanguageEnglish)
	latinPeople, _ := latinPolicy["people"].([]any)
	if displayName := blueclawPersonWithEmail(latinPeople, "latin@example.com")["displayName"]; displayName != "Sample Lee" {
		t.Fatalf("expected Latin display name to remain in recorded order, got %v", displayName)
	}
}

func TestADirectoryRecordReadsTheCirclesFieldTheCompanySends(t *testing.T) {
	var record adminUserMutation
	if errorValue := json.Unmarshal([]byte(`{"email":"member@example.com","role":"member","circles":["member","c-level"]}`), &record); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(record.Circles, []string{"member", "c-level"}) {
		t.Fatalf("the field name the company sends must be the one this reads, got %v", record.Circles)
	}
}

func TestAnAdminTheDirectoryNoLongerNamesGoes(t *testing.T) {
	policyDocument := map[string]any{"people": []any{
		map[string]any{"personID": "person-1", "emails": []any{"boss@example.com"}, "isAdmin": true},
		map[string]any{"personID": "sample", "emails": []any{"admin@example.test"}, "isAdmin": true},
	}}
	records := []adminUserMutation{{MemberID: "user-1", Email: "boss@example.com", Role: "admin"}}

	reconcileRosterPeople(policyDocument, records, nil, workspaceLanguageKorean)

	emails := rosterPolicyEmails(policyDocument)
	if slices.Contains(emails, "admin@example.test") {
		t.Fatalf("an admin the company no longer names must not outlive it, got %v", emails)
	}
	if !slices.Contains(emails, "boss@example.com") {
		t.Fatalf("an admin the company still names stays, got %v", emails)
	}
}

func TestTheSeedAdminStaysEvenWhenTheDirectoryDoesNotNameThem(t *testing.T) {
	policyDocument := map[string]any{"people": []any{
		map[string]any{"personID": "seed", "emails": []any{"seed@example.com"}, "isAdmin": true},
	}}
	records := []adminUserMutation{{MemberID: "user-1", Email: "member@example.com", Role: "member"}}

	reconcileRosterPeople(policyDocument, records, []string{"seed@example.com"}, workspaceLanguageKorean)

	if !slices.Contains(rosterPolicyEmails(policyDocument), "seed@example.com") {
		t.Fatal("the address this device lets an admin in by must survive a directory that forgot them")
	}
}

func TestTheDeviceReadsTheSettingsTheCompanyKeeps(t *testing.T) {
	service := newCompanyPolicyService(t, companyRowAnswering("Asia/Seoul", "en"))

	if timeZone := service.companyTimeZoneName(context.Background()); timeZone != "Asia/Seoul" {
		t.Fatalf("the device reads the zone the company keeps, got %q", timeZone)
	}
	if language := service.workspaceLanguage(context.Background()); language != workspaceLanguageEnglish {
		t.Fatalf("the device works in the language the company keeps, got %q", language)
	}
}

// The profile document is read as a person, and a device nobody has signed
// into names none, so a device that reads its settings through that document
// reads the clock in its own zone and calls it the company's.
func TestAnUnclaimedDeviceStillCarriesTheCompanySettingsIntoThePolicy(t *testing.T) {
	service := newCompanyPolicyService(t, companyRowAnswering("America/Los_Angeles", "en"))

	if errorValue := service.deliverRosterReconciledWith(context.Background(), nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	if timeZone := deliveredCompanyField(t, service, "timeZone"); timeZone != "America/Los_Angeles" {
		t.Fatalf("the delivered policy must carry the company's zone, got %q", timeZone)
	}
	if locale := deliveredCompanyField(t, service, "locale"); locale != workspaceLanguageEnglish {
		t.Fatalf("the delivered policy must carry the company's language, got %q", locale)
	}
}

func TestACompanyTheDeviceCouldNotAskKeepsTheSettingsThePolicyCarries(t *testing.T) {
	service := newCompanyPolicyService(t, companyRowRefusing())

	if errorValue := service.deliverRosterReconciledWith(context.Background(), nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, statError := os.Stat(service.Configuration.BlueclawPolicyDeliveryPath); !os.IsNotExist(statError) {
		t.Fatalf("a settled policy the device could not improve on stays undelivered, got %v", statError)
	}
}

// A profile save writes the company block whole, and a plane that cannot
// answer the settings read must not turn that into a blank zone.
func TestAProfileSaveKeepsTheSettingsThePolicyCarriesWhenThePlaneIsSilent(t *testing.T) {
	service := newCompanyPolicyService(t, companyRowRefusing())

	if errorValue := service.syncCompanySnapshotToBlueclaw(context.Background(), centralplane.CompanyProfile{Name: "예시회사"}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if timeZone := deliveredCompanyField(t, service, "timeZone"); timeZone != "Asia/Seoul" {
		t.Fatalf("a profile save must not blank the zone the policy carries, got %q", timeZone)
	}
	if locale := deliveredCompanyField(t, service, "locale"); locale != workspaceLanguageKorean {
		t.Fatalf("a profile save must not blank the locale the policy carries, got %q", locale)
	}
	if name := deliveredCompanyField(t, service, "name"); name != "예시회사" {
		t.Fatalf("a profile save still writes the profile it was given, got %q", name)
	}
}

func companyRowAnswering(timeZone string, locale string) func() (int, string) {
	return func() (int, string) {
		return http.StatusOK, `{"company":{"name":"예시회사","profileImage":"","timezone":"` + timeZone + `","locale":"` + locale + `"}}`
	}
}

func companyRowRefusing() func() (int, string) {
	return func() (int, string) {
		return http.StatusBadGateway, `{"error":"the record did not answer"}`
	}
}

func newCompanyPolicyService(t *testing.T, companyRow func() (int, string)) *Service {
	t.Helper()
	temporaryDirectory := t.TempDir()
	service := NewService(Configuration{
		CentralPlaneAppURL:         "https://company.example.test",
		CentralPlaneProjectURL:     "https://project.example.test",
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeTestFile(t, "agent-key"),
		BlueclawBaseURL:            "http://blueclaw.local",
		StateDirectory:             temporaryDirectory,
		BlueclawPolicyDeliveryPath: filepath.Join(temporaryDirectory, "policy.json"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/agent/company":
			status, body := companyRow()
			return jsonResponse(status, body, nil), nil
		case "/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"company":{"brandName":"","description":"","locale":"ko","name":"","representative":"","slogan":"","timeZone":"Asia/Seoul","website":""},"people":[]}`, nil), nil
		case "/admin/api/policy/reload":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			return jsonResponse(http.StatusUnauthorized, `{"error":"this call named nobody"}`, nil), nil
		}
	})}
	return service
}

func deliveredCompanyField(t *testing.T, service *Service, field string) string {
	t.Helper()
	document, errorValue := os.ReadFile(service.Configuration.BlueclawPolicyDeliveryPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var policyDocument struct {
		Company map[string]string `json:"company"`
	}
	if errorValue := json.Unmarshal(document, &policyDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	return policyDocument.Company[field]
}

func rosterReadinessAnswer(t *testing.T, service *Service) int {
	t.Helper()
	recorder := httptest.NewRecorder()
	service.handleAdmin(recorder, httptest.NewRequest(http.MethodGet, blueclawruntime.AdmindRosterReadinessPath, nil))
	return recorder.Code
}

func TestTheRosterReachesBlueclawAsSoonAsItAnswersRatherThanAtTheNextTick(t *testing.T) {
	firstWaitForBlueclaw, longestWaitForBlueclaw = 10*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { firstWaitForBlueclaw, longestWaitForBlueclaw = 250*time.Millisecond, 5*time.Second })
	healthRefusalsLeft := atomic.Int32{}
	healthRefusalsLeft.Store(3)
	policyRefusalsLeft := atomic.Int32{}
	policyRefusalsLeft.Store(2)
	reloads := atomic.Int32{}
	service := serviceWhoseDirectorySays(t, `{"members":[
		{"memberID":"member-1","email":"active@example.com","name":"최견본","role":"member","status":"active"}
	]}`)
	service.Configuration.BlueclawPolicyDeliveryPath = filepath.Join(t.TempDir(), "policy.json")
	answering := service.HTTPClient.Transport
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "blueclaw.local" && request.URL.Path == "/admin/api/health" && healthRefusalsLeft.Add(-1) >= 0 {
			return nil, errors.New("connection refused")
		}
		if request.URL.Host == "blueclaw.local" && request.URL.Path == "/admin/api/policy" && policyRefusalsLeft.Add(-1) >= 0 {
			return nil, errors.New("connection refused")
		}
		if request.URL.Path == "/admin/api/policy/reload" {
			reloads.Add(1)
		}
		return answering.RoundTrip(request)
	})}
	if code := rosterReadinessAnswer(t, service); code != http.StatusServiceUnavailable {
		t.Fatalf("the roster readiness answered %d before any roster reached blueclaw", code)
	}
	ctx, stop := context.WithCancel(t.Context())
	defer stop()

	service.startBlueclawRosterReconcile(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for rosterReadinessAnswer(t, service) != http.StatusOK && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if code := rosterReadinessAnswer(t, service); code != http.StatusOK {
		t.Fatalf("blueclaw came up and the roster readiness still answered %d, so a member who writes now is unknown to the agent", code)
	}
	if reloads.Load() != 1 {
		t.Fatalf("the roster was delivered %d times, want once", reloads.Load())
	}
	delivered, errorValue := os.ReadFile(service.Configuration.BlueclawPolicyDeliveryPath)
	if errorValue != nil {
		t.Fatalf("read the delivered roster: %v", errorValue)
	}
	var policyDocument map[string]any
	if errorValue := json.Unmarshal(delivered, &policyDocument); errorValue != nil {
		t.Fatalf("parse the delivered roster: %v", errorValue)
	}
	if !slices.Contains(rosterPolicyEmails(policyDocument), "active@example.com") {
		t.Fatalf("the delivered roster does not name the member: %s", delivered)
	}
}
