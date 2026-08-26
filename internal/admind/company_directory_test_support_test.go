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

// The account directory moved from the fleet register to the company. A stub
// that already says who works here in the register's shape can answer the
// company's question from the same words, so a test states its people once.
func withCompanyDirectoryForTest(t *testing.T, registerBody string, inner roundTripFunc) roundTripFunc {
	t.Helper()
	return func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/api/agent/key"):
			return jsonResponse(http.StatusOK, `{"key":"test-agent-key"}`, nil), nil
		case strings.HasSuffix(request.URL.Path, "/api/agent/member"):
			return companyDirectoryAnswerForTest(t, request, registerBody), nil
		}
		return inner(request)
	}
}

func companyDirectoryAnswerForTest(t *testing.T, request *http.Request, registerBody string) *http.Response {
	t.Helper()
	if request.Method != http.MethodGet {
		return jsonResponse(http.StatusOK, `{"member":`+memberJSONForTest(askedEmailForTest(t, request))+`}`, nil)
	}
	members := companyMembersOfRegisterBodyForTest(t, registerBody)
	askedEmail := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	if askedEmail == "" {
		document, errorValue := json.Marshal(map[string]any{"members": members})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		return jsonResponse(http.StatusOK, string(document), nil)
	}
	for _, member := range members {
		if member["email"] == askedEmail {
			document, errorValue := json.Marshal(map[string]any{"member": member})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, string(document), nil)
		}
	}
	return jsonResponse(http.StatusOK, `{"member":null}`, nil)
}

func companyMembersOfRegisterBodyForTest(t *testing.T, registerBody string) []map[string]any {
	t.Helper()
	members := []map[string]any{}
	if strings.TrimSpace(registerBody) == "" {
		return members
	}
	var registered pagesUsersResponse
	if errorValue := json.Unmarshal([]byte(registerBody), &registered); errorValue != nil {
		t.Fatalf("the register body a stub serves is not a users response: %v", errorValue)
	}
	for _, record := range registered.Records {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		members = append(members, map[string]any{
			"memberID":  firstNonEmpty(record.MemberID, memberIDForTest(email)),
			"email":     email,
			"name":      record.Name,
			"note":      record.Note,
			"role":      normalizeAdminUserRole(record.Role),
			"status":    firstNonEmpty(record.Status, "active"),
			"circles":   record.Circles,
			"messenger": companyDirectoryMessengerForTest(record),
		})
	}
	return members
}

func companyDirectoryMessengerForTest(record adminUserMutation) map[string]string {
	accounts := map[string]string{}
	if account := strings.TrimSpace(record.MattermostUserID); account != "" {
		accounts["mattermost"] = account
	}
	if account := strings.TrimSpace(record.MattermostUsername); account != "" {
		accounts["mattermostUsername"] = account
	}
	return accounts
}

func askedEmailForTest(t *testing.T, request *http.Request) string {
	t.Helper()
	if request.Body == nil {
		return strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	}
	var asked struct {
		Email string `json:"email"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&asked); errorValue != nil {
		return strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	}
	return strings.ToLower(strings.TrimSpace(asked.Email))
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
