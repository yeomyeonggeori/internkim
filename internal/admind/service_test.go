package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic-lab/internkim/internal/capabilities"
	companionruntime "github.com/anthropic-lab/internkim/internal/companion"
)

func TestGatewayRoutesAdminAndMattermost(t *testing.T) {
	service := NewService(Configuration{
		MattermostBaseURL: "http://mattermost.local",
		AdminEmailPath:    writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(bytes.NewBufferString("mattermost")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    request,
		}, nil
	})}
	handler := service.router()

	adminRequest := httptest.NewRequest(http.MethodGet, "/admin/api/health", nil)
	adminRequest.RemoteAddr = "127.0.0.1:12345"
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin health status = %d", adminResponse.Code)
	}

	mattermostRequest := httptest.NewRequest(http.MethodGet, "/team/channels/town-square", nil)
	mattermostResponse := httptest.NewRecorder()
	handler.ServeHTTP(mattermostResponse, mattermostRequest)
	if mattermostResponse.Code != http.StatusAccepted {
		t.Fatalf("mattermost proxy status = %d", mattermostResponse.Code)
	}
	if mattermostResponse.Body.String() != "mattermost" {
		t.Fatalf("mattermost proxy body = %q", mattermostResponse.Body.String())
	}
}

func TestGatewayRedirectsAdminPage(t *testing.T) {
	adminUIPath := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "index.html"), []byte("admin ui"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{
		AdminEmailPath: writeTestFile(t, "admin@example.com"),
		AdminUIPath:    adminUIPath,
	})
	handler := service.router()

	request := httptest.NewRequest(http.MethodGet, "https://dc719d8e.intern.kim/admin", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("admin page status = %d", response.Code)
	}
	if response.Header().Get("Location") != "/admin/" {
		t.Fatalf("admin page location = %q", response.Header().Get("Location"))
	}

	request = httptest.NewRequest(http.MethodGet, "https://dc719d8e.intern.kim/admin/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin ui status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "admin ui") {
		t.Fatalf("admin ui body = %q", response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "https://dc719d8e.intern.kim/admin/companion", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin fallback status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "admin ui") {
		t.Fatalf("admin fallback body = %q", response.Body.String())
	}
}

func TestAdminRejectsUnauthorizedRemoteCaller(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()

	request := httptest.NewRequest(http.MethodGet, "/admin/api/health", nil)
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unauthorized status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/admin/api/health", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authorized status = %d", response.Code)
	}
}

func TestAdminUsersProxyUsesDeviceAuth(t *testing.T) {
	deviceIDPath := writeTestFile(t, "dc719d8e")
	deviceSecretPath := writeTestFile(t, "secret-value")
	service := NewService(Configuration{
		APIBaseURL:        "https://api.intern.kim",
		AdminEmailPath:    writeTestFile(t, "admin@example.com"),
		DeviceIDPath:      deviceIDPath,
		DeviceSecretPath:  deviceSecretPath,
		StateDirectory:    t.TempDir(),
		CompanionJobPath:  filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:       t.TempDir(),
		MattermostBaseURL: "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://api.intern.kim/api/users?device_id=dc719d8e" {
			t.Fatalf("proxy url = %s", request.URL.String())
		}
		if request.Header.Get("X-InternKim-Device-ID") != "dc719d8e" {
			t.Fatalf("device id header = %q", request.Header.Get("X-InternKim-Device-ID"))
		}
		if request.Header.Get("X-InternKim-Device-Secret") != "secret-value" {
			t.Fatalf("device secret header = %q", request.Header.Get("X-InternKim-Device-Secret"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"users":["admin@example.com"]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    request,
		}, nil
	})}
	handler := service.router()

	request := httptest.NewRequest(http.MethodGet, "/admin/api/users", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("users proxy status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "admin@example.com") {
		t.Fatalf("users proxy body = %q", response.Body.String())
	}
}

func TestAdminUsersGetEnsuresBotDirectChannelsForInvitedUsers(t *testing.T) {
	deviceIDPath := writeTestFile(t, "dc719d8e")
	deviceSecretPath := writeTestFile(t, "secret-value")
	adminPasswordPath := writeTestFile(t, "admin-pass")
	directChannelCreated := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.intern.kim",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[{"email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			var payload []string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(payload) != 2 || payload[0] != "user-1" || payload[1] != "bot-1" {
				t.Fatalf("direct channel payload = %#v", payload)
			}
			directChannelCreated = true
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	request := httptest.NewRequest(http.MethodGet, "/admin/api/users", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("users proxy status = %d body = %s", response.Code, response.Body.String())
	}
	if !directChannelCreated {
		t.Fatal("bot direct channel was not created")
	}
}

func TestAdminInviteCreatesMattermostUserAndReturnsTemporaryPasswordOnce(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	var pagesPayload map[string]any
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.intern.kim",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/member@example.com":
			return jsonResponse(http.StatusNotFound, `{"message":"not found"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users" && request.Method == http.MethodPost:
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["email"] != "member@example.com" {
				t.Fatalf("mattermost email = %q", payload["email"])
			}
			if payload["password"] == "" {
				t.Fatal("mattermost password empty")
			}
			return jsonResponse(http.StatusCreated, `{"id":"user-1","email":"member@example.com","username":"member"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["roles"] != "system_user" {
				t.Fatalf("mattermost roles = %q", payload["roles"])
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			var payload []string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(payload) != 2 || payload[0] != "user-1" || payload[1] != "bot-1" {
				t.Fatalf("direct channel payload = %#v", payload)
			}
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
			if request.Header.Get("X-InternKim-Device-Secret") != "secret-value" {
				t.Fatalf("device secret header = %q", request.Header.Get("X-InternKim-Device-Secret"))
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&pagesPayload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if _, exists := pagesPayload["temporaryPassword"]; exists {
				t.Fatal("temporary password leaked to Pages")
			}
			return jsonResponse(http.StatusOK, `{"records":[{"email":"member@example.com","role":"member"}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"email":"member@example.com","role":"member"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("invite status = %d body = %s", response.Code, response.Body.String())
	}

	var document map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document["temporaryPassword"] == "" {
		t.Fatalf("temporary password missing: %#v", document)
	}
	if document["temporaryPasswordEmail"] != "member@example.com" {
		t.Fatalf("temporary password email = %#v", document["temporaryPasswordEmail"])
	}
	if pagesPayload["mattermostUserID"] != "user-1" || pagesPayload["mattermostUsername"] != "member" {
		t.Fatalf("pages payload = %#v", pagesPayload)
	}
}

func TestAdminRemoveDeactivatesMattermostUserByStoredID(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	deactivatedUserID := ""
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.intern.kim",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"},{"email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodDelete:
			deactivatedUserID = "user-1"
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users/member@example.com?device_id=dc719d8e" && request.Method == http.MethodDelete:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	request := httptest.NewRequest(http.MethodDelete, "/admin/api/users/member%40example.com", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("remove status = %d body = %s", response.Code, response.Body.String())
	}
	if deactivatedUserID != "user-1" {
		t.Fatalf("deactivated user id = %q", deactivatedUserID)
	}
}

func TestAdminRemoveSkipsProtectedMattermostUserDeactivation(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	pagesDeleteCalled := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.intern.kim",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "owner@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"owner@example.com","role":"admin"},{"email":"admin@example.com","role":"admin","mattermostUserID":"admin-id","mattermostUsername":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin-id"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin-id","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin-id/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin-id" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"admin-id","email":"admin@example.com","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users/admin@example.com?device_id=dc719d8e" && request.Method == http.MethodDelete:
			pagesDeleteCalled = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	request := httptest.NewRequest(http.MethodDelete, "/admin/api/users/admin%40example.com", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "owner@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("remove protected status = %d body = %s", response.Code, response.Body.String())
	}
	if !pagesDeleteCalled {
		t.Fatal("Pages delete should be called for protected Mattermost user")
	}
}

func TestAdminCompanionReleasesAreSameOrigin(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()

	request := httptest.NewRequest(http.MethodGet, "/admin/api/companion/releases", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("companion releases status = %d", response.Code)
	}

	var document companionReleaseResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(document.Platforms) == 0 {
		t.Fatal("companion releases empty")
	}
	if document.Platforms[0].Platform != "macos" {
		t.Fatalf("first companion release platform = %q", document.Platforms[0].Platform)
	}
}

func writeFile(t *testing.T, path string, document string) {
	t.Helper()
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func jsonResponse(statusCode int, body string, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     header,
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	directoryPath := t.TempDir()
	plainPath := filepath.Join(directoryPath, "plain.tar.gz")
	encryptedPath := filepath.Join(directoryPath, "backup.ikbak")
	decryptedPath := filepath.Join(directoryPath, "decrypted.tar.gz")
	document := []byte("backup document")
	if errorValue := os.WriteFile(plainPath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := encryptFile(plainPath, encryptedPath, "passphrase"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := decryptFile(encryptedPath, decryptedPath, "passphrase"); errorValue != nil {
		t.Fatal(errorValue)
	}
	decryptedDocument, errorValue := os.ReadFile(decryptedPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(decryptedDocument) != string(document) {
		t.Fatalf("decrypted document = %q", string(decryptedDocument))
	}
}

func TestRestoreUploadAssembly(t *testing.T) {
	directoryPath := t.TempDir()
	chunksPath := filepath.Join(directoryPath, "chunks")
	if errorValue := os.MkdirAll(chunksPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(chunksPath, "0"), []byte("hello "), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(chunksPath, "1"), []byte("world"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	service := NewService(Configuration{})
	bundlePath := filepath.Join(directoryPath, "bundle.ikbak")
	errorValue := service.assembleRestoreUpload(&RestoreUpload{DirectoryPath: directoryPath}, 2, bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := os.ReadFile(bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != "hello world" {
		t.Fatalf("assembled document = %q", string(document))
	}
}

func TestRestoreUploadAssemblyRequiresEveryChunk(t *testing.T) {
	service := NewService(Configuration{})
	errorValue := service.assembleRestoreUpload(&RestoreUpload{DirectoryPath: t.TempDir()}, 1, filepath.Join(t.TempDir(), "bundle.ikbak"))
	if errorValue == nil {
		t.Fatal("expected missing chunk error")
	}
}

func TestExtractBundleRejectsUnsafePath(t *testing.T) {
	bundlePath := filepath.Join(t.TempDir(), "backup.tar.gz")
	bundleFile, errorValue := os.Create(bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	gzipWriter := gzip.NewWriter(bundleFile)
	tarWriter := tar.NewWriter(gzipWriter)
	if errorValue := tarWriter.WriteHeader(&tar.Header{Name: "../evil", Mode: 0o600, Size: 4}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := tarWriter.Write([]byte("evil")); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := tarWriter.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := gzipWriter.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := bundleFile.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue = extractBundle(bundlePath, t.TempDir())
	if errorValue == nil {
		t.Fatal("expected unsafe path error")
	}
}

func TestCompanionPairHeartbeatAndJobLifecycle(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	pairingResponse := httptest.NewRecorder()
	pairingRequest := httptest.NewRequest(http.MethodPost, "/admin/api/companion/pairing-codes", nil)
	pairingRequest.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(pairingResponse, pairingRequest)
	if pairingResponse.Code != http.StatusOK {
		t.Fatalf("pairing code status = %d", pairingResponse.Code)
	}
	var pairingCode companionPairingCodeResponse
	if errorValue := json.NewDecoder(pairingResponse.Body).Decode(&pairingCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	if pairingCode.Code == "" || time.Until(pairingCode.ExpiresAt) <= 0 {
		t.Fatalf("unexpected pairing code response: %+v", pairingCode)
	}

	pairResponse := httptest.NewRecorder()
	pairRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{
		"code":"`+pairingCode.Code+`",
		"displayName":"test companion",
		"publicKey":"`+keyPair.PublicKey+`",
		"localOnly":true,
		"capabilities":[{"name":"user.confirm","version":"1","privacyClass":"user_input","estimatedLatency":"interactive","requiresUserPresence":true,"worksOffline":true}]
	}`))
	handler.ServeHTTP(pairResponse, pairRequest)
	if pairResponse.Code != http.StatusOK {
		t.Fatalf("pair status = %d: %s", pairResponse.Code, pairResponse.Body.String())
	}
	var pairResult companionPairResponse
	if errorValue := json.NewDecoder(pairResponse.Body).Decode(&pairResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	if pairResult.CompanionID == "" || pairResult.Token == "" {
		t.Fatalf("unexpected pair result: %+v", pairResult)
	}

	reuseResponse := httptest.NewRecorder()
	reuseRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{"code":"`+pairingCode.Code+`"}`))
	handler.ServeHTTP(reuseResponse, reuseRequest)
	if reuseResponse.Code != http.StatusForbidden {
		t.Fatalf("expected reused pairing code to fail, got %d", reuseResponse.Code)
	}

	statusResponse := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/admin/api/companion/status", nil)
	statusRequest.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status code = %d", statusResponse.Code)
	}
	var status companionStatusResponse
	if errorValue := json.NewDecoder(statusResponse.Body).Decode(&status); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(status.Companions) != 1 || !status.Companions[0].IsOnline {
		t.Fatalf("unexpected companion status: %+v", status)
	}

	unsignedResponse := httptest.NewRecorder()
	unsignedRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/jobs/next", nil)
	unsignedRequest.Header.Set("X-InternKim-Companion-ID", pairResult.CompanionID)
	unsignedRequest.Header.Set("X-InternKim-Companion-Token", pairResult.Token)
	handler.ServeHTTP(unsignedResponse, unsignedRequest)
	if unsignedResponse.Code != http.StatusForbidden {
		t.Fatalf("expected unsigned companion request to fail, got %d", unsignedResponse.Code)
	}

	resultChannel := make(chan capabilities.ToolInvokeResponse, 1)
	errorChannel := make(chan error, 1)
	go func() {
		response, errorValue := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
			ToolName:      "user.confirm",
			Input:         json.RawMessage(`{"message":"continue?"}`),
			TimeoutSecond: 2,
		})
		if errorValue != nil {
			errorChannel <- errorValue
			return
		}
		resultChannel <- response
	}()

	nextResponse := httptest.NewRecorder()
	nextRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/jobs/next", nil)
	setCompanionHeaders(t, nextRequest, pairResult, keyPair.PrivateKey)
	handler.ServeHTTP(nextResponse, nextRequest)
	if nextResponse.Code != http.StatusOK {
		t.Fatalf("next job status = %d: %s", nextResponse.Code, nextResponse.Body.String())
	}
	var companionJob CompanionJob
	if errorValue := json.NewDecoder(nextResponse.Body).Decode(&companionJob); errorValue != nil {
		t.Fatal(errorValue)
	}
	if companionJob.JobID == "" || companionJob.Request.ToolName != "user.confirm" {
		t.Fatalf("unexpected companion job: %+v", companionJob)
	}

	completeResponse := httptest.NewRecorder()
	completeRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/jobs/"+companionJob.JobID+"/complete", strings.NewReader(`{
		"provider":"companion",
		"selectedBackend":"companion_local",
		"toolName":"user.confirm",
		"result":{"confirmed":true}
	}`))
	setCompanionHeaders(t, completeRequest, pairResult, keyPair.PrivateKey)
	handler.ServeHTTP(completeResponse, completeRequest)
	if completeResponse.Code != http.StatusOK {
		t.Fatalf("complete status = %d: %s", completeResponse.Code, completeResponse.Body.String())
	}

	select {
	case response := <-resultChannel:
		if response.ToolName != "user.confirm" {
			t.Fatalf("unexpected invoke response: %+v", response)
		}
	case errorValue := <-errorChannel:
		t.Fatal(errorValue)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for companion job result")
	}
}

func TestCompanionDenyReturnsStructuredObservation(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pairingResponse := httptest.NewRecorder()
	pairingRequest := httptest.NewRequest(http.MethodPost, "/admin/api/companion/pairing-codes", nil)
	pairingRequest.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(pairingResponse, pairingRequest)
	var pairingCode companionPairingCodeResponse
	if errorValue := json.NewDecoder(pairingResponse.Body).Decode(&pairingCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	pairResponse := httptest.NewRecorder()
	pairRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{
		"code":"`+pairingCode.Code+`",
		"displayName":"test companion",
		"publicKey":"`+keyPair.PublicKey+`",
		"capabilities":[{"name":"browser.open","version":"1","privacyClass":"user_browser","estimatedLatency":"interactive","requiresUserPresence":true,"worksOffline":true}]
	}`))
	handler.ServeHTTP(pairResponse, pairRequest)
	var pairResult companionPairResponse
	if errorValue := json.NewDecoder(pairResponse.Body).Decode(&pairResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	resultChannel := make(chan capabilities.ToolInvokeResponse, 1)
	go func() {
		response, _ := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
			ToolName:      "browser.open",
			Input:         json.RawMessage(`{"url":"https://github.com"}`),
			PrivacyClass:  "user_browser",
			TimeoutSecond: 2,
		})
		resultChannel <- response
	}()
	nextResponse := httptest.NewRecorder()
	nextRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/jobs/next", nil)
	setCompanionHeaders(t, nextRequest, pairResult, keyPair.PrivateKey)
	handler.ServeHTTP(nextResponse, nextRequest)
	var companionJob CompanionJob
	if errorValue := json.NewDecoder(nextResponse.Body).Decode(&companionJob); errorValue != nil {
		t.Fatal(errorValue)
	}
	denyResponse := httptest.NewRecorder()
	denyRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/jobs/"+companionJob.JobID+"/deny", strings.NewReader(`{
		"status":"denied",
		"code":"user_denied",
		"userReason":"not this site",
		"suggestedConstraint":"ask for text"
	}`))
	setCompanionHeaders(t, denyRequest, pairResult, keyPair.PrivateKey)
	handler.ServeHTTP(denyResponse, denyRequest)
	if denyResponse.Code != http.StatusOK {
		t.Fatalf("deny status = %d: %s", denyResponse.Code, denyResponse.Body.String())
	}
	select {
	case response := <-resultChannel:
		if response.Status != "denied" {
			t.Fatalf("expected denied response, got %+v", response)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for denial")
	}
}

func TestCompanionJobPersistenceRestoresPendingAndCompletedJobs(t *testing.T) {
	stateDirectory := t.TempDir()
	configuration := Configuration{StateDirectory: stateDirectory, AdminEmailPath: writeTestFile(t, "admin@example.com")}
	service := NewService(configuration)
	now := time.Now().UTC()
	service.companionJobs["pending-job"] = &CompanionJob{
		JobID:     "pending-job",
		Status:    "pending",
		ToolName:  "user.confirm",
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}
	service.companionJobs["running-job"] = &CompanionJob{
		JobID:       "running-job",
		Status:      "running",
		CompanionID: "companion-1",
		ToolName:    "user.confirm",
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(time.Minute),
	}
	service.companionJobs["completed-job"] = &CompanionJob{
		JobID:     "completed-job",
		Status:    "completed",
		ToolName:  "user.confirm",
		Response:  &capabilities.ToolInvokeResponse{ToolName: "user.confirm"},
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}
	if errorValue := service.saveCompanionJobs(); errorValue != nil {
		t.Fatal(errorValue)
	}

	reloadedService := NewService(configuration)

	if reloadedService.companionJobs["pending-job"].Status != "pending" {
		t.Fatalf("expected pending job to reload, got %+v", reloadedService.companionJobs["pending-job"])
	}
	if reloadedService.companionJobs["running-job"].Status != "pending" || reloadedService.companionJobs["running-job"].CompanionID != "" {
		t.Fatalf("expected running job to become pending on restart, got %+v", reloadedService.companionJobs["running-job"])
	}
	if reloadedService.companionJobs["completed-job"].Status != "completed" {
		t.Fatalf("expected completed job to reload, got %+v", reloadedService.companionJobs["completed-job"])
	}
}

func TestCompanionJobClaimRequeuesStaleRunningJob(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	staleCompanion := &CompanionRecord{
		CompanionID: "stale-companion",
		Capabilities: []capabilities.Descriptor{
			{Name: "user.confirm"},
		},
		LastSeenAt: now.Add(-2 * companionOnlineWindow),
	}
	activeCompanion := &CompanionRecord{
		CompanionID: "active-companion",
		Capabilities: []capabilities.Descriptor{
			{Name: "user.confirm"},
		},
		LastSeenAt: now,
	}
	service.companions[staleCompanion.CompanionID] = staleCompanion
	service.companions[activeCompanion.CompanionID] = activeCompanion
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:       "job-1",
		Status:      "running",
		CompanionID: staleCompanion.CompanionID,
		ToolName:    "user.confirm",
		Request:     capabilities.ToolInvokeRequest{ToolName: "user.confirm"},
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(time.Minute),
	}

	claimedJob := service.claimNextCompanionJob(activeCompanion)

	if claimedJob == nil || claimedJob.JobID != "job-1" || claimedJob.CompanionID != activeCompanion.CompanionID {
		t.Fatalf("expected stale running job to be claimed by active companion, got %+v", claimedJob)
	}
	reloadedService := NewService(service.Configuration)
	if reloadedService.companionJobs["job-1"].Status != "pending" {
		t.Fatalf("expected restart recovery to make running job retryable, got %+v", reloadedService.companionJobs["job-1"])
	}
}

func TestCompanionFileUploadLifecycle(t *testing.T) {
	service := NewService(Configuration{
		StateDirectory:         t.TempDir(),
		CompanionFileDirectory: t.TempDir(),
		AdminEmailPath:         writeTestFile(t, "admin@example.com"),
	})
	handler := service.router()
	keyPair, pairResult := pairTestCompanion(t, handler, keyPairCapabilityRequest{
		KeyPair:    keyPairForTest(t),
		Capability: `{"name":"file.pick","version":"1","privacyClass":"local_file","estimatedLatency":"interactive","requiresUserPresence":true,"worksOffline":true}`,
	})
	_ = keyPair

	resultChannel := make(chan capabilities.ToolInvokeResponse, 1)
	errorChannel := make(chan error, 1)
	go func() {
		response, errorValue := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
			ToolName:      "file.pick",
			PrivacyClass:  "local_file",
			TimeoutSecond: 2,
		})
		if errorValue != nil {
			errorChannel <- errorValue
			return
		}
		resultChannel <- response
	}()

	nextResponse := httptest.NewRecorder()
	nextRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/jobs/next", nil)
	setCompanionHeaders(t, nextRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(nextResponse, nextRequest)
	if nextResponse.Code != http.StatusOK {
		t.Fatalf("next status = %d: %s", nextResponse.Code, nextResponse.Body.String())
	}
	var companionJob CompanionJob
	if errorValue := json.NewDecoder(nextResponse.Body).Decode(&companionJob); errorValue != nil {
		t.Fatal(errorValue)
	}

	unsignedUploadResponse := httptest.NewRecorder()
	unsignedUploadRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/files/uploads", strings.NewReader(`{"jobID":"`+companionJob.JobID+`","filename":"report.txt","sizeBytes":11}`))
	unsignedUploadRequest.Header.Set("X-InternKim-Companion-ID", pairResult.CompanionID)
	unsignedUploadRequest.Header.Set("X-InternKim-Companion-Token", pairResult.Token)
	handler.ServeHTTP(unsignedUploadResponse, unsignedUploadRequest)
	if unsignedUploadResponse.Code != http.StatusForbidden {
		t.Fatalf("expected unsigned upload to fail, got %d", unsignedUploadResponse.Code)
	}

	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/files/uploads", strings.NewReader(`{"jobID":"`+companionJob.JobID+`","filename":"../report.txt","sizeBytes":11,"contentType":"text/plain","ttlSeconds":300}`))
	setCompanionHeaders(t, createRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("upload create status = %d: %s", createResponse.Code, createResponse.Body.String())
	}
	var createResult companionFileUploadCreateResponse
	if errorValue := json.NewDecoder(createResponse.Body).Decode(&createResult); errorValue != nil {
		t.Fatal(errorValue)
	}

	chunkResponse := httptest.NewRecorder()
	chunkRequest := httptest.NewRequest(http.MethodPut, "/_internkim/companion/files/uploads/"+createResult.UploadID+"/chunks/0", strings.NewReader("hello world"))
	setCompanionHeaders(t, chunkRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(chunkResponse, chunkRequest)
	if chunkResponse.Code != http.StatusOK {
		t.Fatalf("chunk status = %d: %s", chunkResponse.Code, chunkResponse.Body.String())
	}

	completeUploadResponse := httptest.NewRecorder()
	completeUploadRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/files/uploads/"+createResult.UploadID+"/complete", strings.NewReader(`{"chunks":1}`))
	setCompanionHeaders(t, completeUploadRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(completeUploadResponse, completeUploadRequest)
	if completeUploadResponse.Code != http.StatusOK {
		t.Fatalf("upload complete status = %d: %s", completeUploadResponse.Code, completeUploadResponse.Body.String())
	}
	var uploadResult companionFileUploadCompleteResponse
	if errorValue := json.NewDecoder(completeUploadResponse.Body).Decode(&uploadResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	if uploadResult.DevicePath != filepath.Join(service.Configuration.CompanionFileDirectory, "report.txt") {
		t.Fatalf("unexpected device path: %s", uploadResult.DevicePath)
	}
	document, errorValue := os.ReadFile(uploadResult.DevicePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != "hello world" {
		t.Fatalf("unexpected uploaded document: %s", string(document))
	}

	completeJobResponse := httptest.NewRecorder()
	completeJobRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/jobs/"+companionJob.JobID+"/complete", strings.NewReader(toolResponseJSON(t, "file.pick", uploadResult)))
	setCompanionHeaders(t, completeJobRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(completeJobResponse, completeJobRequest)
	if completeJobResponse.Code != http.StatusOK {
		t.Fatalf("job complete status = %d: %s", completeJobResponse.Code, completeJobResponse.Body.String())
	}
	select {
	case response := <-resultChannel:
		if response.ToolName != "file.pick" {
			t.Fatalf("unexpected response: %+v", response)
		}
	case errorValue := <-errorChannel:
		t.Fatal(errorValue)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for file pick response")
	}
}

func TestCompanionFileUploadOverwriteAndCleanup(t *testing.T) {
	service := NewService(Configuration{CompanionFileDirectory: t.TempDir(), StateDirectory: t.TempDir()})
	uploadOne := writeCompanionUploadChunks(t, "upload-1", "report.txt", "first")
	resultOne, errorValue := service.finishCompanionFileUpload(uploadOne, 1)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	uploadTwo := writeCompanionUploadChunks(t, "upload-2", "report.txt", "second")
	uploadTwo.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	resultTwo, errorValue := service.finishCompanionFileUpload(uploadTwo, 1)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if resultOne.DevicePath != resultTwo.DevicePath {
		t.Fatal("expected same filename to overwrite the same temp path")
	}
	document, errorValue := os.ReadFile(resultTwo.DevicePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != "second" {
		t.Fatalf("expected overwrite content, got %s", string(document))
	}
	metadataLessPath := filepath.Join(service.Configuration.CompanionFileDirectory, "manual.txt")
	if errorValue := os.WriteFile(metadataLessPath, []byte("keep"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.cleanupExpiredCompanionFiles(time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(resultTwo.DevicePath); !os.IsNotExist(errorValue) {
		t.Fatalf("expected expired upload to be removed, got %v", errorValue)
	}
	if _, errorValue := os.Stat(metadataLessPath); errorValue != nil {
		t.Fatalf("expected metadata-less file to remain: %v", errorValue)
	}
}

func setCompanionHeaders(t *testing.T, request *http.Request, pairResult companionPairResponse, privateKey string) {
	t.Helper()
	request.Header.Set("X-InternKim-Companion-ID", pairResult.CompanionID)
	request.Header.Set("X-InternKim-Companion-Token", pairResult.Token)
	body, _ := io.ReadAll(request.Body)
	request.Body = io.NopCloser(bytes.NewReader(body))
	if errorValue := companionruntime.SignRequest(request, body, privateKey); errorValue != nil {
		t.Fatal(errorValue)
	}
}

type testPairResult struct {
	companionPairResponse
	privateKey string
}

type keyPairCapabilityRequest struct {
	KeyPair    companionruntime.KeyPair
	Capability string
}

func pairTestCompanion(t *testing.T, handler http.Handler, request keyPairCapabilityRequest) (companionruntime.KeyPair, testPairResult) {
	t.Helper()
	keyPair := request.KeyPair
	pairingResponse := httptest.NewRecorder()
	pairingRequest := httptest.NewRequest(http.MethodPost, "/admin/api/companion/pairing-codes", nil)
	pairingRequest.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(pairingResponse, pairingRequest)
	var pairingCode companionPairingCodeResponse
	if errorValue := json.NewDecoder(pairingResponse.Body).Decode(&pairingCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	pairResponse := httptest.NewRecorder()
	pairRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{
		"code":"`+pairingCode.Code+`",
		"displayName":"test companion",
		"publicKey":"`+keyPair.PublicKey+`",
		"capabilities":[`+request.Capability+`]
	}`))
	handler.ServeHTTP(pairResponse, pairRequest)
	if pairResponse.Code != http.StatusOK {
		t.Fatalf("pair status = %d: %s", pairResponse.Code, pairResponse.Body.String())
	}
	var pairResult companionPairResponse
	if errorValue := json.NewDecoder(pairResponse.Body).Decode(&pairResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	return keyPair, testPairResult{companionPairResponse: pairResult, privateKey: keyPair.PrivateKey}
}

func keyPairForTest(t *testing.T) companionruntime.KeyPair {
	t.Helper()
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return keyPair
}

func toolResponseJSON(t *testing.T, toolName string, result any) string {
	t.Helper()
	resultDocument, errorValue := json.Marshal(result)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	responseDocument, errorValue := json.Marshal(capabilities.ToolInvokeResponse{
		Provider:        "companion",
		SelectedBackend: capabilities.LLMBackendCompanionLocal,
		ToolName:        toolName,
		Result:          resultDocument,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(responseDocument)
}

func writeCompanionUploadChunks(t *testing.T, uploadID string, filename string, document string) *CompanionFileUpload {
	t.Helper()
	directoryPath := filepath.Join(t.TempDir(), uploadID)
	chunksPath := filepath.Join(directoryPath, "chunks")
	if errorValue := os.MkdirAll(chunksPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(chunksPath, "0"), []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return &CompanionFileUpload{
		UploadID:       uploadID,
		JobID:          "job-" + uploadID,
		CompanionID:    "companion-1",
		Filename:       filename,
		ContentType:    "text/plain",
		SizeBytes:      int64(len(document)),
		ExpiresAt:      time.Now().UTC().Add(time.Hour),
		DirectoryPath:  directoryPath,
		ReceivedChunks: map[int]bool{0: true},
	}
}

func writeTestFile(t *testing.T, document string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
