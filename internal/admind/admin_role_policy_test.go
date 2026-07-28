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

func TestAdminSessionReportsOperationsAdminRoleDespiteOrganizationCache(t *testing.T) {
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, claimedAdminEmailPath, "owner@example.com")
	service := NewService(Configuration{
		APIBaseURL:            "https://api.intern.kim",
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		StateDirectory:        t.TempDir(),
		AdminUIPath:           t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.intern.kim/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"operator@example.com","role":"operationsAdmin"}]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	personPayload, errorValue := json.Marshal(organizationCachedPerson{Record: newOrganizationCachedUserRecord(adminUserMutation{Email: "operator@example.com", Role: "admin"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "email:operator@example.com"}
	if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(context.Background(), key, 0, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write organization person cache: written = %t error = %v", written, errorValue)
	}

	responseDocument := requestAdminSession(t, service, "operator@example.com")
	if responseDocument["role"] != "operationsAdmin" || responseDocument["isAdmin"] != false {
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
		APIBaseURL:            "https://api.intern.kim",
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		StateDirectory:        t.TempDir(),
		AdminUIPath:           t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.intern.kim/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	responseDocument := requestAdminSession(t, service, "owner@example.com")
	if responseDocument["role"] != "admin" || responseDocument["isAdmin"] != true {
		t.Fatalf("admin session = %#v", responseDocument)
	}
}



func TestOperationsAdminCanUseAllowedAdminEndpoint(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/admin/api/users", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("users status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "operator@example.com") {
		t.Fatalf("users body = %q", response.Body.String())
	}
}

func TestOperationsAdminCannotGrantAdminRole(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"email":"new@example.com","handle":"newuser","name":"New User","role":"admin"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("users status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotGrantAdminRoleInBatch(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", strings.NewReader(`{"users":[{"email":"new@example.com","handle":"newuser","name":"New User","role":"admin"}]}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("users batch status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotDemoteAdminRole(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"email":"admin@example.com","handle":"adminuser","name":"Admin User","role":"member"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("users status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotDemoteAdminRoleInBatch(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", strings.NewReader(`{"users":[{"email":"admin@example.com","handle":"adminuser","name":"Admin User","role":"member"}]}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("users batch status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotResetAdminPassword(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/users/admin%40example.com/password-reset", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("password reset status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotDeleteAdminUser(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodDelete, "/admin/api/users/admin%40example.com", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("delete user status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotSaveReservedAdminCircle(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/admin/api/circles", strings.NewReader(`{"circleID":"admin","displayName":"Admin","isMattermostManaged":true}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("save circle status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotDeleteReservedAdminCircle(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodDelete, "/admin/api/circles/admin", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("delete circle status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestAdminCannotDeleteReservedAdminCircle(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodDelete, "/admin/api/circles/admin", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "owner@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("delete circle status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminCannotUseFullAdminEndpoint(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/admin/api/bot-profile", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("bot profile status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestOperationsAdminPathPolicy(t *testing.T) {
	if !isOperationsAdminPath(http.MethodGet, "/users") {
		t.Fatal("operations admin should access users")
	}
	if !isOperationsAdminPath(http.MethodPost, "/users/person@example.com/password-reset") {
		t.Fatal("operations admin should reset user passwords")
	}
	if !isOperationsAdminPath(http.MethodPut, "/workspace-settings") {
		t.Fatal("operations admin should update workspace settings")
	}
	if isOperationsAdminPath(http.MethodGet, "/bot-profile") {
		t.Fatal("operations admin should not access bot profile")
	}
	if isOperationsAdminPath(http.MethodPost, "/updates/apply") {
		t.Fatal("operations admin should not apply updates")
	}
}

func newOperationsAdminAuthorizationTestService(t *testing.T) *Service {
	t.Helper()
	rootPath := t.TempDir()
	service := NewService(Configuration{
		APIBaseURL:               "https://api.intern.kim",
		ClaimedAdminEmailPath:    writeTestFile(t, "owner@example.com"),
		FleetIDPath:              writeTestFile(t, "dc719d8e"),
		FleetSecretPath:          writeTestFile(t, "secret-value"),
		CalendarDatabasePath:     filepath.Join(rootPath, "calendar.sqlite"),
		CalendarSecretsDirectory: filepath.Join(rootPath, "secrets", "google-oauth"),
		StateDirectory:           filepath.Join(rootPath, "state"),
		AdminUIPath:              t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.intern.kim/api/users?fleet_id=dc719d8e" {
			return jsonResponse(http.StatusOK, `{"users":["operator@example.com","admin@example.com"],"records":[{"email":"operator@example.com","role":"operationsAdmin"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
}
