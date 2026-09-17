package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func companyDeviceForTest(t *testing.T, companyURL string, agentKey string) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	agentKeyPath := filepath.Join(stateDirectory, "agent-key")
	if errorValue := os.WriteFile(agentKeyPath, []byte(agentKey+"\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return &Service{
		Configuration: Configuration{
			CentralPlaneAppURL:         companyURL,
			CentralPlaneProjectURL:     "https://project.example.test",
			CentralPlanePublishableKey: "publishable",
			CentralPlaneAgentKeyPath:   agentKeyPath,
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
			{"memberID":"m2","email":"colleague@example.com","name":"박예시","role":"member","status":"active"},
			{"memberID":"m3","email":"gone@example.com","name":"최견본","role":"admin","status":"withdrawn"}
		]}`))
	}))
	defer company.Close()

	service := companyDeviceForTest(t, company.URL, "the-company-key")
	records, errorValue := service.currentUserRecords(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(records) != 2 || records[0].MemberID != "m1" || records[1].Role != "member" {
		t.Fatalf("the company's answer arrived reshaped: %+v", records)
	}
}

func TestACompanyThatDoesNotAnswerIsAFailureNotAnEmptyCompany(t *testing.T) {
	company := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Error(responseWriter, "the record is down", http.StatusBadGateway)
	}))
	defer company.Close()

	service := companyDeviceForTest(t, company.URL, "the-company-key")
	if _, errorValue := service.currentUserRecords(context.Background()); errorValue == nil {
		t.Fatal("a company that did not answer was reported as an empty company")
	}
}
