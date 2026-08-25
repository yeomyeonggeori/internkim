package admind

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const companyDirectoryURLForTest = "https://app.example.test"

func seatPeopleInACompanyDirectoryForTest(t *testing.T, service *Service) {
	t.Helper()
	service.centralPlaneOnce = sync.Once{}
	service.centralPlaneClient = nil
	service.Configuration.CentralPlaneAppURL = companyDirectoryURLForTest
	service.Configuration.CentralPlaneProjectURL = "https://project.example.test"
	service.Configuration.CentralPlanePublishableKey = "publishable"
	service.Configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	service.Configuration.CentralPlaneAppURLPath = filepath.Join(t.TempDir(), "central-plane-app-url")
}

func isCompanyDirectoryRequest(request *http.Request) bool {
	return strings.HasPrefix(request.URL.String(), companyDirectoryURLForTest+"/api/agent/member")
}

func companyDirectoryResponse(t *testing.T, request *http.Request) (*http.Response, error) {
	t.Helper()
	email := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	if request.Method == http.MethodPost {
		var asked struct {
			Email string `json:"email"`
		}
		if errorValue := json.NewDecoder(request.Body).Decode(&asked); errorValue != nil {
			t.Fatal(errorValue)
		}
		email = strings.ToLower(strings.TrimSpace(asked.Email))
	}
	if email == "" {
		return jsonResponse(http.StatusOK, `{"members":[]}`, nil), nil
	}
	return jsonResponse(http.StatusOK, `{"member":`+memberJSONForTest(email)+`}`, nil), nil
}

func memberJSONForTest(email string) string {
	return `{"memberID":"` + memberIDForTest(email) + `","email":"` + email + `","role":"member","status":"active"}`
}

func memberIDForTest(email string) string {
	return "member-" + strings.ReplaceAll(strings.Split(email, "@")[0], ".", "-")
}
