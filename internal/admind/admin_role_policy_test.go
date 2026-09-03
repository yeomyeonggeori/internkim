package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminSessionReadsTheRoleFromTheDirectoryNotTheOrganizationCache(t *testing.T) {
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
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"colleague@example.com","role":"member"}]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	personPayload, errorValue := json.Marshal(organizationCachedPerson{Record: newOrganizationCachedUserRecord(adminUserMutation{Email: "colleague@example.com", Role: "admin"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "email:colleague@example.com"}
	if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(context.Background(), key, 0, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write organization person cache: written = %t error = %v", written, errorValue)
	}

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
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
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
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"users":["colleague@example.com","admin@example.com"],"records":[{"email":"colleague@example.com","role":"member"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		}
		if strings.Contains(request.URL.Path, "/api/agent/key") {
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
}
