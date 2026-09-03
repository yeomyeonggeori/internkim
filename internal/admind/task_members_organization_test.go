package admind

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestTaskMembersOnlyIncludeOrganizationChartPeople(t *testing.T) {
	temporaryDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory: temporaryDirectory,
		DatabasePath:   filepath.Join(temporaryDirectory, "state.sqlite"),
		ListenAddress:  "127.0.0.1:0",
	})
	company := startCompanyHoldingTheseMembers(t, `{"members":[
		{"memberID":"user-ada","email":"ada@example.com","name":"Ada Kim","role":"member","status":"active"},
		{"memberID":"user-grace","email":"grace@example.com","name":"Grace Lee","role":"member","status":"active"},
		{"memberID":"user-hidden","email":"hidden@example.com","name":"Hidden Lee","role":"member","status":"departed"},
		{"memberID":"user-resigned","email":"resigned@example.com","name":"Resigned Park","role":"member","status":"withdrawn"}
	]}`)
	useCompanyForTest(service, company.URL)

	request := httptest.NewRequest(http.MethodGet, "/task/api/state", nil)
	request.Header.Set("X-Forwarded-Email", "grace@example.com")
	emails := map[string]bool{}
	for _, member := range service.taskMembers(request) {
		emails[member.Email] = true
	}

	for _, expectedEmail := range []string{"ada@example.com", "grace@example.com"} {
		if !emails[expectedEmail] {
			t.Fatalf("missing %s in %#v", expectedEmail, emails)
		}
	}
	for _, unexpectedEmail := range []string{"hidden@example.com", "resigned@example.com"} {
		if emails[unexpectedEmail] {
			t.Fatalf("kept %s outside the organization chart: %#v", unexpectedEmail, emails)
		}
	}
}

func startCompanyHoldingTheseMembers(t *testing.T, membersDocument string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/agent/member" {
			_, _ = responseWriter.Write([]byte(membersDocument))
			return
		}
		responseWriter.WriteHeader(http.StatusNotFound)
		_, _ = responseWriter.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server
}
