package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func companyDeviceForTest(t *testing.T, companyURL string, fleetIndexURL string, agentKey string) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	agentKeyPath := filepath.Join(stateDirectory, "agent-key")
	if agentKey != "" {
		if errorValue := os.WriteFile(agentKeyPath, []byte(agentKey+"\n"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	fleetIDPath := filepath.Join(stateDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(stateDirectory, "fleet-secret")
	for path, value := range map[string]string{fleetIDPath: "fleet9", fleetSecretPath: "secret"} {
		if errorValue := os.WriteFile(path, []byte(value+"\n"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return &Service{
		Configuration: Configuration{
			CentralPlaneAppURL:       companyURL,
			CentralPlaneAgentKeyPath: agentKeyPath,
			APIBaseURL:               fleetIndexURL,
			FleetIDPath:              fleetIDPath,
			FleetSecretPath:          fleetSecretPath,
		},
		HTTPClient: http.DefaultClient,
	}
}

func TestADeviceWithACompanyAsksTheCompanyWhoWorksThere(t *testing.T) {
	company := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/agent/member" {
			t.Errorf("the company was asked %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer the-company-key" {
			t.Errorf("the device did not present its own key: %q", request.Header.Get("Authorization"))
		}
		responseWriter.Write([]byte(`{"members":[
			{"memberID":"m1","email":"lead@example.com","name":"이샘플","role":"admin","status":"active"},
			{"memberID":"m2","email":"colleague@example.com","name":"박예시","role":"member","status":"active"}
		]}`))
	}))
	defer company.Close()
	fleetIndex := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a device that belongs to a company asked the fleet index anyway")
	}))
	defer fleetIndex.Close()

	service := companyDeviceForTest(t, company.URL, fleetIndex.URL, "the-company-key")
	records, errorValue := service.currentUserRecords(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(records) != 2 || records[0].MemberID != "m1" || records[1].Role != "member" {
		t.Fatalf("the company's answer arrived reshaped: %+v", records)
	}
}

func TestADeviceWithoutACompanyStillAsksTheFleetIndex(t *testing.T) {
	fleetIndex := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Write([]byte(`{"records":[{"email":"legacy@example.com","role":"admin"}]}`))
	}))
	defer fleetIndex.Close()

	service := companyDeviceForTest(t, "http://127.0.0.1:1", fleetIndex.URL, "")
	records, errorValue := service.currentUserRecords(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(records) != 1 || records[0].Email != "legacy@example.com" {
		t.Fatalf("the legacy path changed shape: %+v", records)
	}
}

func TestACompanyThatDoesNotAnswerIsAFailureNotACueToAskElsewhere(t *testing.T) {
	company := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Error(responseWriter, "the record is down", http.StatusBadGateway)
	}))
	defer company.Close()
	fleetIndex := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a failing company lookup fell back to the fleet index")
	}))
	defer fleetIndex.Close()

	service := companyDeviceForTest(t, company.URL, fleetIndex.URL, "the-company-key")
	if _, errorValue := service.currentUserRecords(context.Background()); errorValue == nil {
		t.Fatal("a company that did not answer was reported as an empty company")
	}
}
