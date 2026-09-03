package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func blueclawServingSkills(asked *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		*asked = append(*asked, request.URL.Path)
		if request.URL.Path != "/admin/api/skills" {
			http.NotFound(responseWriter, request)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"skills":[{"name":"calendar","path":"/delivery/skills/calendar"}],"unavailableSkills":[]}`))
	}))
}

func TestSkillInventoryIsRefusedWithoutAnActor(t *testing.T) {
	asked := []string{}
	blueclaw := blueclawServingSkills(&asked)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	recorder := httptest.NewRecorder()
	service.handleSkillInventory(recorder, httptest.NewRequest(http.MethodGet, skillInventoryPath, nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated read answered %d, want 401", recorder.Code)
	}
	if len(asked) != 0 {
		t.Fatalf("Blueclaw was asked anyway: %v", asked)
	}
}

func TestSkillInventoryIsRefusedForSomebodyWhoIsNotAnAdministrator(t *testing.T) {
	asked := []string{}
	blueclaw := blueclawServingSkills(&asked)
	defer blueclaw.Close()
	service := NewService(Configuration{
		BlueclawBaseURL: blueclaw.URL,
		AdminEmailPath:  writeTestFile(t, "admin@example.com"),
	})

	request := httptest.NewRequest(http.MethodGet, skillInventoryPath, nil)
	request.Header.Set(requesterEmailHeader, "member@example.com")
	recorder := httptest.NewRecorder()
	markRequestsAsAssertedByTheListener(http.HandlerFunc(service.handleSkillInventory)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("a member's read answered %d, want 403", recorder.Code)
	}
	if len(asked) != 0 {
		t.Fatalf("Blueclaw was asked anyway: %v", asked)
	}
}

func TestSkillInventoryIsProxiedForAnAdministrator(t *testing.T) {
	asked := []string{}
	blueclaw := blueclawServingSkills(&asked)
	defer blueclaw.Close()
	service := NewService(Configuration{
		BlueclawBaseURL: blueclaw.URL,
		AdminEmailPath:  writeTestFile(t, "admin@example.com"),
	})

	request := httptest.NewRequest(http.MethodGet, skillInventoryPath, nil)
	request.Header.Set(requesterEmailHeader, "admin@example.com")
	recorder := httptest.NewRecorder()
	markRequestsAsAssertedByTheListener(http.HandlerFunc(service.handleSkillInventory)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("an administrator's read answered %d, want 200", recorder.Code)
	}
	if len(asked) != 1 || asked[0] != "/admin/api/skills" {
		t.Fatalf("Blueclaw was asked for %v", asked)
	}
	if !strings.Contains(recorder.Body.String(), `"calendar"`) {
		t.Fatalf("the inventory did not reach the caller: %s", recorder.Body.String())
	}
}

func TestSkillInventoryIsReadOnly(t *testing.T) {
	asked := []string{}
	blueclaw := blueclawServingSkills(&asked)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	recorder := httptest.NewRecorder()
	service.handleSkillInventory(recorder, httptest.NewRequest(http.MethodPost, skillInventoryPath, strings.NewReader("{}")))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a POST answered %d, want 404", recorder.Code)
	}
}

func TestSkillInventoryPathIsRoutedAndCounted(t *testing.T) {
	service := &Service{}
	recorder := httptest.NewRecorder()
	service.router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, skillInventoryPath, nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated read through the router answered %d, want 401", recorder.Code)
	}
	if !isInternKimAPIPath(skillInventoryPath) {
		t.Fatal("the skill inventory is not counted as an internkim API path")
	}
	if isAdminStaticPath(skillInventoryPath) {
		t.Fatal("the skill inventory is counted as a static page")
	}
}
