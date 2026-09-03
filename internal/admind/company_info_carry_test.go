package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type companyHoldingTheProfile struct {
	server  *httptest.Server
	written []map[string]any
	refuse  bool
}

func startCompanyForProfileCarry(t *testing.T) *companyHoldingTheProfile {
	t.Helper()
	company := &companyHoldingTheProfile{}
	company.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/agent/session":
			_, _ = responseWriter.Write([]byte(`{"memberID":"member-admin","accessToken":"token","expiresAt":4102444800}`))
		case "/api/v1/tools/company_info_set/invoke":
			var payload struct {
				Input map[string]any `json:"input"`
			}
			_ = json.NewDecoder(request.Body).Decode(&payload)
			if company.refuse {
				responseWriter.WriteHeader(http.StatusBadRequest)
				_, _ = responseWriter.Write([]byte(`{"error":"the record refused this profile"}`))
				return
			}
			company.written = append(company.written, payload.Input)
			_, _ = responseWriter.Write([]byte(`{"tool":"company_info_set","result":{"language":"ko"}}`))
		default:
			responseWriter.WriteHeader(http.StatusNotFound)
			_, _ = responseWriter.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(company.server.Close)
	return company
}

func newProfileCarryTestService(t *testing.T, planeURL string) *Service {
	t.Helper()
	configuration := Configuration{
		StateDirectory:        t.TempDir(),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
	}
	if planeURL != "" {
		configuration.CentralPlaneAppURL = planeURL
		configuration.CentralPlaneProjectURL = planeURL
		configuration.CentralPlanePublishableKey = "publishable"
		configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	}
	return NewService(configuration)
}

func writeHeldProfileForTest(t *testing.T, service *Service, document string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(service.companyProfilePath()), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(service.companyProfilePath(), []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

const heldProfileDocument = `{
	"name": {"ko": "주식회사 샘플거리", "en": "Sample Street Inc."},
	"representative": {"ko": "이샘플"},
	"legalAttributes": {"ko": {"사업자등록번호": "123-45-67890"}},
	"phone": "02-1234-5678",
	"employeeCount": 12
}`

func TestTheProfileCarryWritesOneLanguageSlotAtATime(t *testing.T) {
	company := startCompanyForProfileCarry(t)
	service := newProfileCarryTestService(t, company.server.URL)
	writeHeldProfileForTest(t, service, heldProfileDocument)

	report, errorValue := service.carryTheCompanyProfileIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(report.Languages) != 2 || len(company.written) != 2 {
		t.Fatalf("report = %+v written = %+v", report, company.written)
	}
	byLanguage := map[string]map[string]any{}
	for _, written := range company.written {
		byLanguage[written["language"].(string)] = written
	}
	korean, isKorean := byLanguage["ko"]
	if !isKorean || korean["name"] != "주식회사 샘플거리" || korean["representative"] != "이샘플" {
		t.Fatalf("the Korean slot = %+v", korean)
	}
	if korean["legalAttributes"] != `{"사업자등록번호":"123-45-67890"}` {
		t.Fatalf("country-specific labels are written as a document: %+v", korean["legalAttributes"])
	}
	english, isEnglish := byLanguage["en"]
	if !isEnglish || english["name"] != "Sample Street Inc." {
		t.Fatalf("the English slot = %+v", english)
	}
	if _, hasRepresentative := english["representative"]; hasRepresentative {
		t.Fatalf("a slot nothing was written in carries nothing: %+v", english)
	}
	for _, written := range company.written {
		if written["phone"] != "02-1234-5678" || written["employeeCount"] != float64(12) {
			t.Fatalf("what is not localized goes into every slot: %+v", written)
		}
	}
}

func TestTheProfileFileGoesOnceTheRecordHoldsIt(t *testing.T) {
	company := startCompanyForProfileCarry(t)
	service := newProfileCarryTestService(t, company.server.URL)
	writeHeldProfileForTest(t, service, heldProfileDocument)

	service.sweepTheCompanyProfileTheRecordNowHolds(context.Background())

	if _, errorValue := os.Stat(service.companyProfilePath()); !os.IsNotExist(errorValue) {
		t.Fatal("the file stayed after the record took everything in it")
	}
}

// A half-carried profile is worse than a file: the record would hold one
// language and the device the other, and nothing would say so.
func TestTheProfileFileStaysWhenALanguageWasRefused(t *testing.T) {
	company := startCompanyForProfileCarry(t)
	company.refuse = true
	service := newProfileCarryTestService(t, company.server.URL)
	writeHeldProfileForTest(t, service, heldProfileDocument)

	service.sweepTheCompanyProfileTheRecordNowHolds(context.Background())

	if _, errorValue := os.Stat(service.companyProfilePath()); errorValue != nil {
		t.Fatal("the file went while the record was still refusing it")
	}
}

func TestTheProfileCarryRefusesWithoutAClaimedAdministrator(t *testing.T) {
	company := startCompanyForProfileCarry(t)
	service := newProfileCarryTestService(t, company.server.URL)
	service.Configuration.ClaimedAdminEmailPath = writeTestFile(t, "")
	writeHeldProfileForTest(t, service, heldProfileDocument)

	if _, errorValue := service.carryTheCompanyProfileIntoTheRecord(context.Background()); errorValue == nil {
		t.Fatal("the master profile is an administrator's to record, and the carry wrote one as nobody")
	}
}

func TestADeviceNamingNoCompanyKeepsTheProfileFile(t *testing.T) {
	service := newProfileCarryTestService(t, "")
	writeHeldProfileForTest(t, service, heldProfileDocument)

	service.sweepTheCompanyProfileTheRecordNowHolds(context.Background())

	if _, errorValue := os.Stat(service.companyProfilePath()); errorValue != nil {
		t.Fatal("a device that names no company let go of the profile nobody else holds")
	}
}
