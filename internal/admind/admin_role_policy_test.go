package admind

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminSessionReadsTheRoleFromTheDirectory(t *testing.T) {
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, claimedAdminEmailPath, "owner@example.com")
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		StateDirectory:        t.TempDir(),
		AdminUIPath:           t.TempDir(),
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	directory := companyDirectoryHolding(memberForTest("colleague@example.com", "박예시", "member"))
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return directory.respond(t, request)
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	responseDocument := requestAdminSession(t, service, "colleague@example.com")
	if responseDocument["role"] != "member" || responseDocument["isAdmin"] != false {
		t.Fatalf("admin session = %#v", responseDocument)
	}
}

func TestAdminSessionPreservesClaimedAdminRole(t *testing.T) {
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, claimedAdminEmailPath, "owner@example.com")
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		StateDirectory:        t.TempDir(),
		AdminUIPath:           t.TempDir(),
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	directory := companyDirectoryHolding()
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return directory.respond(t, request)
		}
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		if request.Method == http.MethodPost && strings.Contains(request.URL.String(), "/admin/api/policy") {
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	responseDocument := requestAdminSession(t, service, "owner@example.com")
	if responseDocument["role"] != "admin" || responseDocument["isAdmin"] != true {
		t.Fatalf("admin session = %#v", responseDocument)
	}
}

func TestSomebodyTheDirectoryDoesNotCallAdminIsRefusedTheAdminConsole(t *testing.T) {
	service := newAdminConsoleAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/admin/api/users", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "colleague@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("users status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestAdminCannotDeleteReservedAdminCircle(t *testing.T) {
	service := newAdminConsoleAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodDelete, "/admin/api/circles/admin", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "owner@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("delete circle status = %d body = %s", response.Code, response.Body.String())
	}
}

func newAdminConsoleAuthorizationTestService(t *testing.T) *Service {
	t.Helper()
	rootPath := t.TempDir()
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		ClaimedAdminEmailPath: writeTestFile(t, "owner@example.com"),
		FleetIDPath:           writeTestFile(t, "dc719d8e"),
		FleetSecretPath:       writeTestFile(t, "secret-value"),
		CalendarDatabasePath:  filepath.Join(rootPath, "calendar.sqlite"),
		StateDirectory:        filepath.Join(rootPath, "state"),
		AdminUIPath:           t.TempDir(),
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	directory := companyDirectoryHolding(memberForTest("colleague@example.com", "박예시", "member"), memberForTest("admin@example.com", "이샘플", "admin"))
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return directory.respond(t, request)
		}
		if strings.Contains(request.URL.Path, "/api/agent/key") {
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
}
