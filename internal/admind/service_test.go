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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
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

func TestDefaultConfigurationUsesCanonicalHostPaths(t *testing.T) {
	configuration := DefaultConfiguration()

	if configuration.StateDirectory != "/root/.internkim/state/admin" {
		t.Fatalf("state directory = %q", configuration.StateDirectory)
	}
	if configuration.AdminEmailPath != "/root/.internkim/config/admin-email" {
		t.Fatalf("admin email path = %q", configuration.AdminEmailPath)
	}
	if configuration.ClaimedAdminEmailPath != "/root/.internkim/state/admin/claimed-admin-email" {
		t.Fatalf("claimed admin email path = %q", configuration.ClaimedAdminEmailPath)
	}
	if configuration.BotProfilePath != "/root/.internkim/config/bot-profile.yaml" {
		t.Fatalf("bot profile path = %q", configuration.BotProfilePath)
	}
}

func TestAdminIdentityReadsLegacyTopLevelFallbacks(t *testing.T) {
	rootPath := t.TempDir()
	configuration := Configuration{
		AdminEmailPath:        filepath.Join(rootPath, "config", "admin-email"),
		ClaimedAdminEmailPath: filepath.Join(rootPath, "state", "admin", "claimed-admin-email"),
	}
	writeFile(t, filepath.Join(rootPath, "admin-email"), "Seed@Example.COM")
	writeFile(t, filepath.Join(rootPath, "claimed-admin-email"), "Claimed@Example.COM")
	service := NewService(configuration)

	if service.seedAdminEmail() != "seed@example.com" {
		t.Fatalf("seed admin email = %q", service.seedAdminEmail())
	}
	if service.claimedAdminEmail() != "claimed@example.com" {
		t.Fatalf("claimed admin email = %q", service.claimedAdminEmail())
	}
}

func TestBackupIncludedPathsUseCanonicalConfigurationAndStateDirectories(t *testing.T) {
	paths := strings.Join(backupIncludedPaths(), "\n")

	for _, fragment := range []string{
		"/root/.internkim/config",
		"/root/.internkim/state",
		"/root/.internkim/secrets",
		"/root/.blueclaw/config",
		"/root/.blueclaw/workspace",
	} {
		if !strings.Contains(paths, fragment) {
			t.Fatalf("expected backup paths to include %q", fragment)
		}
	}
	if strings.Contains(paths, "/root/.internkim/admin-email") {
		t.Fatalf("backup paths should not include legacy admin email file: %s", paths)
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

	request := httptest.NewRequest(http.MethodGet, "https://dc719d8e.example.test/admin", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusFound {
		t.Fatalf("admin page status = %d", response.Code)
	}
	if response.Header().Get("Location") != "/admin/" {
		t.Fatalf("admin page location = %q", response.Header().Get("Location"))
	}

	request = httptest.NewRequest(http.MethodGet, "https://dc719d8e.example.test/admin/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin ui status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "admin ui") {
		t.Fatalf("admin ui body = %q", response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "https://dc719d8e.example.test/admin/companion", nil)
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

func TestAdminHealthDoesNotClaimFirstAuthenticatedCaller(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminEmailPath := filepath.Join(deviceDirectory, "admin-email")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminEmailPath, "setup@example.com")

	var roleWrites []adminUserMutation
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        adminEmailPath,
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		DeviceIDPath:          deviceIDPath,
		DeviceSecretPath:      deviceSecretPath,
		StateDirectory:        t.TempDir(),
		CompanionJobPath:      filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:           t.TempDir(),
		MattermostBaseURL:     "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"setup@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			var payload adminUserMutation
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			roleWrites = append(roleWrites, payload)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/health", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "lee@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("admin health status = %d body = %s", response.Code, response.Body.String())
	}
	if strings.TrimSpace(readTrimmedFile(claimedAdminEmailPath)) != "" {
		t.Fatalf("claimed admin = %q", readTrimmedFile(claimedAdminEmailPath))
	}
	if len(roleWrites) != 0 {
		t.Fatalf("unexpected role writes: %+v", roleWrites)
	}
}

func TestAdminPageRequestClaimsFirstAuthenticatedCaller(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")
	adminUIPath := t.TempDir()
	writeFile(t, filepath.Join(adminUIPath, "index.html"), "admin ui")

	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:       claimedAdminEmailPath,
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 adminUIPath,
	})
	blueclawInvited := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertFirstAdminPasswordPolicyPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@example.com":
			return jsonResponse(http.StatusNotFound, `{"message":"not found"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"user-1","email":"lee@example.com","username":"lee"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, blueclawPolicyWithSeedAdmin(), nil), nil
		case isBlueclawAdminPolicySave(t, request, "lee@example.com"):
			blueclawInvited = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "lee@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin page status = %d body = %s", response.Code, response.Body.String())
	}
	if strings.TrimSpace(readTrimmedFile(claimedAdminEmailPath)) != "lee@example.com" {
		t.Fatalf("claimed admin = %q", readTrimmedFile(claimedAdminEmailPath))
	}
	if !blueclawInvited {
		t.Fatal("first admin was not invited in Blueclaw policy")
	}
	bootstrapResult := service.readFirstAdminBootstrapResult()
	if bootstrapResult.PolicyVersion != firstAdminPolicyVersion {
		t.Fatalf("bootstrap policy version = %#v", bootstrapResult)
	}
}

func TestAdminSessionReportsMissingAccessIdentity(t *testing.T) {
	service := NewService(Configuration{
		AdminEmailPath: writeTestFile(t, ""),
		StateDirectory: t.TempDir(),
		AdminUIPath:    t.TempDir(),
	})
	request := httptest.NewRequest(http.MethodGet, "/admin/api/session", nil)
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin session status = %d body = %s", response.Code, response.Body.String())
	}
	var document map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document["bootstrapStatus"] != firstAdminBootstrapIdentityMissing {
		t.Fatalf("bootstrap status = %#v", document)
	}
}

func TestAdminSessionReportsFirstAdminBootstrapFailure(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")

	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		MattermostBaseURL:     "http://mattermost.local",
		AdminEmailPath:        filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		DeviceIDPath:          deviceIDPath,
		DeviceSecretPath:      deviceSecretPath,
		StateDirectory:        t.TempDir(),
		CompanionJobPath:      filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:           t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@example.com","role":"admin"}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseDocument := requestAdminSession(t, service, "lee@example.com")
	if responseDocument["bootstrapStatus"] != firstAdminBootstrapFailed {
		t.Fatalf("bootstrap status = %#v", responseDocument)
	}
	if responseDocument["bootstrapError"] == "" {
		t.Fatalf("bootstrap error missing: %#v", responseDocument)
	}
	if strings.TrimSpace(readTrimmedFile(claimedAdminEmailPath)) != "" {
		t.Fatalf("claimed admin = %q", readTrimmedFile(claimedAdminEmailPath))
	}
}

func TestAdminSessionReturnsFirstAdminTemporaryPasswordOnce(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	createdMattermostPassword := ""
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:       claimedAdminEmailPath,
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			var payload adminUserMutation
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.Email != "lee@example.com" || payload.Role != "admin" {
				t.Fatalf("unexpected user role payload: %+v", payload)
			}
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertFirstAdminPasswordPolicyPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@example.com":
			return jsonResponse(http.StatusNotFound, `{"message":"not found"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users" && request.Method == http.MethodPost:
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["email"] != "lee@example.com" || payload["password"] == "" {
				t.Fatalf("unexpected Mattermost create payload: %#v", payload)
			}
			createdMattermostPassword = payload["password"]
			return jsonResponse(http.StatusCreated, `{"id":"user-1","email":"lee@example.com","username":"lee"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["roles"] != "system_admin system_user" {
				t.Fatalf("mattermost roles = %q", payload["roles"])
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, blueclawPolicyWithSeedAdmin(), nil), nil
		case isBlueclawAdminPolicySave(t, request, "lee@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	firstResponse := requestAdminSession(t, service, "lee@example.com")
	if createdMattermostPassword != firstAdminMattermostPassword {
		t.Fatalf("first admin Mattermost password = %q", createdMattermostPassword)
	}
	if firstResponse["temporaryPassword"] != firstAdminMattermostPassword {
		t.Fatalf("temporary password not returned: response=%#v created=%q", firstResponse, createdMattermostPassword)
	}
	if firstResponse["temporaryPasswordEmail"] != "lee@example.com" {
		t.Fatalf("temporary password email = %#v", firstResponse["temporaryPasswordEmail"])
	}

	secondResponse := requestAdminSession(t, service, "lee@example.com")
	if _, exists := secondResponse["temporaryPassword"]; exists {
		t.Fatalf("temporary password returned twice: %#v", secondResponse)
	}
}

func TestAdminSessionResetsExistingFirstAdminMattermostPassword(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	passwordReset := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:       claimedAdminEmailPath,
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertFirstAdminPasswordPolicyPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@example.com":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"lee@example.com","username":"lee"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/password" && request.Method == http.MethodPut:
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["new_password"] != firstAdminMattermostPassword {
				t.Fatalf("Mattermost password update = %#v", payload)
			}
			passwordReset = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, blueclawPolicyWithSeedAdmin(), nil), nil
		case isBlueclawAdminPolicySave(t, request, "lee@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	response := requestAdminSession(t, service, "lee@example.com")
	if !passwordReset {
		t.Fatal("Mattermost password was not reset")
	}
	if response["temporaryPassword"] != firstAdminMattermostPassword {
		t.Fatalf("temporary password not returned: %#v", response)
	}
}

func TestAdminSessionRepairsClaimedFirstAdminPasswordFromOldBootstrap(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	stateDirectory := t.TempDir()
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, claimedAdminEmailPath, "lee@example.com")
	writeFile(t, adminPasswordPath, "admin-pass")
	writeFile(t, filepath.Join(stateDirectory, "first-admin-bootstrap.json"), `{"email":"lee@example.com","status":"claimed"}`)

	passwordReset := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:       claimedAdminEmailPath,
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              stateDirectory,
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertFirstAdminPasswordPolicyPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@example.com":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"lee@example.com","username":"lee"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/password" && request.Method == http.MethodPut:
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["new_password"] != firstAdminMattermostPassword {
				t.Fatalf("Mattermost password update = %#v", payload)
			}
			passwordReset = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@example.com","role":"admin"}]}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, blueclawPolicyWithClaimedMember(), nil), nil
		case isBlueclawAdminPolicySave(t, request, "lee@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	response := requestAdminSession(t, service, "lee@example.com")
	if !passwordReset {
		t.Fatal("Mattermost password was not repaired")
	}
	if response["bootstrapStatus"] != firstAdminBootstrapClaimed {
		t.Fatalf("bootstrap status = %#v", response)
	}
	bootstrapResult := service.readFirstAdminBootstrapResult()
	if bootstrapResult.MattermostPasswordVersion != firstAdminMattermostPasswordVersion {
		t.Fatalf("bootstrap result = %#v", bootstrapResult)
	}
}

func TestAdminUsersProxyUsesDeviceAuth(t *testing.T) {
	deviceIDPath := writeTestFile(t, "dc719d8e")
	deviceSecretPath := writeTestFile(t, "secret-value")
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
		DeviceIDPath:          deviceIDPath,
		DeviceSecretPath:      deviceSecretPath,
		StateDirectory:        t.TempDir(),
		CompanionJobPath:      filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:           t.TempDir(),
		MattermostBaseURL:     "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://api.example.test/api/users?device_id=dc719d8e" {
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
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath:       writeTestFile(t, "admin@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e":
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
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
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
	blueclawInvited := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
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
		case isMattermostConnectCommandSetupRequest(request):
			return mattermostConnectCommandSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostNicknameDisplayPatch(t, request)
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
			if payload["username"] != "member" || payload["first_name"] != "Member" || payload["last_name"] != "One" || payload["nickname"] != "Member" {
				t.Fatalf("mattermost identity payload = %#v", payload)
			}
			if payload["password"] == "" {
				t.Fatal("mattermost password empty")
			}
			return jsonResponse(http.StatusCreated, `{"id":"user-1","email":"member@example.com","username":"member"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostFlowSetupResponse(t, request), nil
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
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
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
		case isBlueclawInviteRequest(t, request, "member@example.com"):
			blueclawInvited = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"handle":"member","name":"Member One","email":"member@example.com","role":"member"}`))
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
	if pagesPayload["handle"] != "member" || pagesPayload["name"] != "Member One" {
		t.Fatalf("pages identity payload = %#v", pagesPayload)
	}
	if !blueclawInvited {
		t.Fatal("invited Mattermost user was not invited in Blueclaw policy")
	}
}

func TestAdminUserSavePatchesMattermostIdentityByStoredID(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, filepath.Join(deviceDirectory, "admin-email"), "admin@example.com")
	var pagesPayload map[string]any
	var mattermostPatch map[string]string
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		BlueclawBaseURL:             "http://127.0.0.1:8080",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-password"),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"oldhandle","name":"Old Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"oldhandle"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isMattermostConnectCommandSetupRequest(request):
			return mattermostConnectCommandSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostNicknameDisplayPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"oldhandle","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch" && request.Method == http.MethodPut:
			if errorValue := json.NewDecoder(request.Body).Decode(&mattermostPatch); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"newhandle"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			if errorValue := json.NewDecoder(request.Body).Decode(&pagesPayload); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"newhandle","name":"New Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"newhandle"}]}`, nil), nil
		case isBlueclawInviteRequest(t, request, "member@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"userID":"user-member","handle":"newhandle","name":"New Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"oldhandle"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("user save status = %d body = %s", response.Code, response.Body.String())
	}
	if mattermostPatch["username"] != "newhandle" || mattermostPatch["first_name"] != "New" || mattermostPatch["last_name"] != "Name" || mattermostPatch["nickname"] != "New" {
		t.Fatalf("mattermost patch = %#v", mattermostPatch)
	}
	if pagesPayload["userID"] != "user-member" || pagesPayload["handle"] != "newhandle" || pagesPayload["name"] != "New Name" {
		t.Fatalf("pages payload = %#v", pagesPayload)
	}
}

func TestAdminInvitePreservesCurrentAdminRole(t *testing.T) {
	deviceDirectory := t.TempDir()
	deviceIDPath := filepath.Join(deviceDirectory, "device-id")
	deviceSecretPath := filepath.Join(deviceDirectory, "device-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, deviceSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	var pagesPayload map[string]any
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath:       writeTestFile(t, "admin@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isMattermostConnectCommandSetupRequest(request):
			return mattermostConnectCommandSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostNicknameDisplayPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/admin@example.com":
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"user-1","email":"admin@example.com","username":"admin-example"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["roles"] != "system_admin system_user" {
				t.Fatalf("mattermost roles = %q", payload["roles"])
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			if errorValue := json.NewDecoder(request.Body).Decode(&pagesPayload); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case isBlueclawInviteRequest(t, request, "admin@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"handle":"admin-example","name":"Admin Example","email":"admin@example.com","role":"member"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("invite status = %d body = %s", response.Code, response.Body.String())
	}
	if pagesPayload["role"] != "admin" {
		t.Fatalf("expected current admin role to be preserved, got %#v", pagesPayload)
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
	systemPostsDeleted := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		DeviceIDPath:                deviceIDPath,
		DeviceSecretPath:            deviceSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		if name == "systemctl" && strings.Join(arguments, " ") == "start internkim-users-sync.service" {
			return nil, nil
		}
		if name != "sh" || len(arguments) != 2 || arguments[0] != "-c" {
			t.Fatalf("unexpected command: %s %#v", name, arguments)
		}
		command := arguments[1]
		if !strings.Contains(command, "UPDATE posts SET deleteat") || !strings.Contains(command, "%member%") {
			t.Fatalf("unexpected cleanup command: %s", command)
		}
		systemPostsDeleted = true
		return nil, nil
	}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"},{"email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isMattermostConnectCommandSetupRequest(request):
			return mattermostConnectCommandSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodDelete:
			deactivatedUserID = "user-1"
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users/member@example.com?device_id=dc719d8e" && request.Method == http.MethodDelete:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case isBlueclawRemoveRequest(t, request, "member@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
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
	if !systemPostsDeleted {
		t.Fatal("system posts were not deleted")
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
		APIBaseURL:                  "https://api.example.test",
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
		case request.URL.String() == "https://api.example.test/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"owner@example.com","role":"admin"},{"email":"admin@example.com","role":"admin","mattermostUserID":"admin-id","mattermostUsername":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin-id"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin-id","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin-id/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isMattermostConnectCommandSetupRequest(request):
			return mattermostConnectCommandSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin-id" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"admin-id","email":"admin@example.com","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users/admin@example.com?device_id=dc719d8e" && request.Method == http.MethodDelete:
			pagesDeleteCalled = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isBlueclawRemoveRequest(t, request, "admin@example.com"):
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

func TestBotProfileUpdatePatchesMattermostAndWorkspaceProfile(t *testing.T) {
	workspacePath := t.TempDir()
	profilePath := filepath.Join(t.TempDir(), "bot-profile.yaml")
	patchBody := ""
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-password"),
		BotProfilePath:              profilePath,
		BlueclawWorkspacePath:       workspacePath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","first_name":"Intern","last_name":"Kim","nickname":"김인턴","roles":"system_user"}`, nil), nil
		case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/users/bot-1/patch":
			body, _ := io.ReadAll(request.Body)
			patchBody = string(body)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodPut, "/admin/api/bot-profile", strings.NewReader(`{
		"displayName":"김비서",
		"englishDisplayName":"Kim Secretary",
		"aliases":["비서"],
		"publicDescription":"업무를 빠르게 돕습니다",
		"identityExtension":"Always use the display name."
	}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("bot profile status = %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(patchBody, `"first_name":"Kim"`) || !strings.Contains(patchBody, `"last_name":"Secretary"`) || !strings.Contains(patchBody, `"nickname":"김비서"`) || !strings.Contains(patchBody, `"position":"업무를 빠르게 돕습니다"`) {
		t.Fatalf("unexpected Mattermost patch body: %s", patchBody)
	}
	workspaceDocument, errorValue := os.ReadFile(filepath.Join(workspacePath, "BOT_PROFILE.yaml"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	workspaceText := string(workspaceDocument)
	if !strings.Contains(workspaceText, `displayName: "김비서"`) || !strings.Contains(workspaceText, "Always use the display name.") {
		t.Fatalf("unexpected workspace bot profile: %s", workspaceText)
	}
	if strings.Contains(workspaceText, "# IDENTITY.md") {
		t.Fatalf("workspace bot profile should not contain full identity document: %s", workspaceText)
	}
}

func TestBotProfileMigratesLegacyJSONStateToYAML(t *testing.T) {
	stateDirectory := t.TempDir()
	profilePath := filepath.Join(stateDirectory, "bot-profile.yaml")
	legacyPath := filepath.Join(stateDirectory, "bot-profile.json")
	writeFile(t, legacyPath, `{"displayName":"김비서","englishDisplayName":"Kim Secretary","aliases":["비서"],"identityExtension":"Be crisp."}`)
	service := NewService(Configuration{
		BotProfilePath:        profilePath,
		BlueclawWorkspacePath: t.TempDir(),
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
	})

	profile, found := service.loadBotProfile()

	if !found {
		t.Fatal("expected legacy bot profile to load")
	}
	if profile.DisplayName != "김비서" {
		t.Fatalf("display name = %q", profile.DisplayName)
	}
	document, errorValue := os.ReadFile(profilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(document), `displayName: "김비서"`) {
		t.Fatalf("expected yaml profile, got %s", string(document))
	}
}

func TestBotProfileMigratesLegacyStateProfileToCanonicalConfiguration(t *testing.T) {
	rootPath := t.TempDir()
	profilePath := filepath.Join(rootPath, "config", "bot-profile.yaml")
	legacyPath := filepath.Join(rootPath, "state", "bot-profile.yaml")
	if errorValue := os.MkdirAll(filepath.Dir(legacyPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, legacyPath, `displayName: "김비서"
englishDisplayName: "Kim Secretary"
aliases:
  - "비서"
identityExtension: "Be crisp."
`)
	service := NewService(Configuration{
		BotProfilePath:        profilePath,
		BlueclawWorkspacePath: t.TempDir(),
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
	})

	profile, found := service.loadBotProfile()

	if !found {
		t.Fatal("expected legacy state bot profile to load")
	}
	if profile.DisplayName != "김비서" {
		t.Fatalf("display name = %q", profile.DisplayName)
	}
	document, errorValue := os.ReadFile(profilePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(document), `displayName: "김비서"`) {
		t.Fatalf("expected canonical yaml profile, got %s", string(document))
	}
}

func TestFlowSizeDefinitionsPersist(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	ctx := context.Background()
	definitions, errorValue := service.readFlowDefinitions(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	definitions.Sizes = []flowSizeDefinition{
		sizeDefinition("T", 21, 64, "테스트 개발", "테스트 기타", "테스트 비고"),
	}
	if errorValue := service.writeFlowDefinitions(ctx, definitions); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloadedDefinitions, errorValue := service.readFlowDefinitions(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(reloadedDefinitions.Sizes) != 1 || reloadedDefinitions.Sizes[0].Name != "T" || reloadedDefinitions.Sizes[0].DistanceKM != 21 {
		t.Fatalf("sizes = %#v", reloadedDefinitions.Sizes)
	}
}

func TestFlowAPIRejectsUnauthenticatedRemoteCaller(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil),
		httptest.NewRequest(http.MethodPost, "/flow/api/tasks", strings.NewReader(`{}`)),
		httptest.NewRequest(http.MethodPut, "/flow/api/definitions", strings.NewReader(`{}`)),
	} {
		request.RemoteAddr = "198.51.100.10:443"
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s status = %d body = %s", request.Method, request.URL.Path, response.Code, response.Body.String())
		}
	}
}

func TestFlowAPIAllowsStaffSummaryAndOwnTask(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()

	summaryRequest := httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil)
	summaryRequest.RemoteAddr = "198.51.100.10:443"
	summaryRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	summaryResponse := httptest.NewRecorder()
	handler.ServeHTTP(summaryResponse, summaryRequest)
	if summaryResponse.Code != http.StatusOK {
		t.Fatalf("summary status = %d body = %s", summaryResponse.Code, summaryResponse.Body.String())
	}
	var summary flowSummaryResponse
	if errorValue := json.NewDecoder(summaryResponse.Body).Decode(&summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	if summary.CurrentUserEmail != "staff@example.com" || summary.IsAdmin {
		t.Fatalf("summary current user email=%q isAdmin=%v", summary.CurrentUserEmail, summary.IsAdmin)
	}

	taskRequest := newFlowTaskRequest("staff@example.com", "staff@example.com")
	taskResponse := httptest.NewRecorder()
	handler.ServeHTTP(taskResponse, taskRequest)
	if taskResponse.Code != http.StatusOK {
		t.Fatalf("task status = %d body = %s", taskResponse.Code, taskResponse.Body.String())
	}
	var task flowTask
	if errorValue := json.NewDecoder(taskResponse.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if task.Status != "진행" || task.OwnerID != stableFlowID("staff@example.com") {
		t.Fatalf("task = %+v", task)
	}
}

func TestFlowAPIForcesStaffTaskForOtherMemberToRequest(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, newFlowTaskRequest("staff@example.com", "other@example.com"))

	if response.Code != http.StatusOK {
		t.Fatalf("task status = %d body = %s", response.Code, response.Body.String())
	}
	var task flowTask
	if errorValue := json.NewDecoder(response.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if task.Status != "요청" {
		t.Fatalf("status = %q", task.Status)
	}
	if !containsString(task.ParticipantIDs, stableFlowID("staff@example.com")) {
		t.Fatalf("expected requester participant, got %+v", task.ParticipantIDs)
	}
}

func TestFlowAPIDefinitionsRequireAdmin(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffRequest := newFlowDefinitionsRequest("staff@example.com")
	staffResponse := httptest.NewRecorder()
	handler.ServeHTTP(staffResponse, staffRequest)
	if staffResponse.Code != http.StatusForbidden {
		t.Fatalf("staff status = %d body = %s", staffResponse.Code, staffResponse.Body.String())
	}

	adminRequest := newFlowDefinitionsRequest("admin@example.com")
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin status = %d body = %s", adminResponse.Code, adminResponse.Body.String())
	}
}

func TestFlowAPILocalCapabilityRequiresRequesterActor(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("local summary status = %d body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set(flowRequesterEmailHeader, "staff@example.com")
	response = httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("local requester summary status = %d body = %s", response.Code, response.Body.String())
	}
	var summary flowSummaryResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	if summary.CurrentUserEmail != "staff@example.com" || summary.IsAdmin {
		t.Fatalf("local requester summary current user email=%q isAdmin=%v", summary.CurrentUserEmail, summary.IsAdmin)
	}
}

func TestFlowMattermostNotificationCreatesUpdatesAndDeletesPost(t *testing.T) {
	service, requests := newFlowNotificationTestService(t)
	task := flowNotificationTestTask("요청")
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	task = service.syncFlowMattermostNotification(context.Background(), task)
	if task.MattermostPostID != "flow-post-1" {
		t.Fatalf("created post id = %q", task.MattermostPostID)
	}
	if requests.createdMessages[0] == "" || !strings.Contains(requests.createdMessages[0], "요청 · 김민수 · 10분 회의") {
		t.Fatalf("created messages = %+v", requests.createdMessages)
	}

	task.Status = "완료"
	task.Content = "회의 완료"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	task = service.syncFlowMattermostNotification(context.Background(), task)
	if len(requests.createdMessages) != 1 {
		t.Fatalf("expected one created post, got %+v", requests.createdMessages)
	}
	if len(requests.updatedMessages) != 1 || !strings.Contains(requests.updatedMessages[0], "완료 · 김민수 · 회의 완료") {
		t.Fatalf("updated messages = %+v", requests.updatedMessages)
	}

	task.Status = "일시정지"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	task = service.syncFlowMattermostNotification(context.Background(), task)
	if task.MattermostPostID != "" || len(requests.deletedPostIDs) != 1 || requests.deletedPostIDs[0] != "flow-post-1" {
		t.Fatalf("post id=%q deleted=%+v", task.MattermostPostID, requests.deletedPostIDs)
	}
	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded task: found=%v error=%v", found, errorValue)
	}
	if reloadedTask.MattermostPostID != "" {
		t.Fatalf("reloaded post id = %q", reloadedTask.MattermostPostID)
	}
}

func TestFlowMattermostNotificationSkipsQuietStatuses(t *testing.T) {
	service, requests := newFlowNotificationTestService(t)
	for _, status := range []string{"예정", "일시정지"} {
		task := flowNotificationTestTask(status)
		task.ID = "task-" + status
		task = service.syncFlowMattermostNotification(context.Background(), task)
		if task.MattermostPostID != "" {
			t.Fatalf("status %q post id = %q", status, task.MattermostPostID)
		}
	}
	if len(requests.createdMessages) != 0 || len(requests.updatedMessages) != 0 || len(requests.deletedPostIDs) != 0 {
		t.Fatalf("unexpected notification requests: %+v", requests)
	}
}

func TestFlowTaskFromRequestForOtherMemberForcesRequest(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	members := []flowMember{
		{ID: "me", Name: "me", Email: "me@example.com"},
		{ID: "lee", Name: "lee", Email: "lee@example.com"},
	}
	payload := flowTaskWriteRequest{
		OwnerID:        "lee",
		ParticipantIDs: []string{"lee"},
		Type:           "회의",
		Content:        "10분 회의",
		Size:           "XS",
		Status:         "진행",
		WeekCode:       "26W18",
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", bytes.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "me@example.com")
	task, errorValue := service.flowTaskFromRequest(request, members, flowDefinitions{
		Types: []string{"회의"},
		Sizes: defaultFlowSizeDefinitions(),
	}, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if task.Status != "요청" {
		t.Fatalf("status = %q", task.Status)
	}
	if task.RequestReason == "" {
		t.Fatalf("request reason was empty")
	}
}

type flowNotificationRequests struct {
	createdMessages []string
	updatedMessages []string
	deletedPostIDs  []string
}

func newFlowNotificationTestService(t *testing.T) (*Service, *flowNotificationRequests) {
	t.Helper()
	requests := &flowNotificationRequests{}
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		FlowDatabasePath:            filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			requests.createdMessages = append(requests.createdMessages, mattermostPostMessage(t, request))
			return jsonResponse(http.StatusCreated, `{"id":"flow-post-1"}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostExistingFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-post-1/patch" && request.Method == http.MethodPut:
			requests.updatedMessages = append(requests.updatedMessages, mattermostPostMessage(t, request))
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-post-1" && request.Method == http.MethodDelete:
			requests.deletedPostIDs = append(requests.deletedPostIDs, "flow-post-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service, requests
}

func mattermostExistingFlowSetupResponse(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	switch {
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
		return jsonResponse(http.StatusOK, `{"id":"flow-channel"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
		assertMattermostFlowChannelPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
		return jsonResponse(http.StatusOK, `{"order":["flow-entry"],"posts":{"flow-entry":{"id":"flow-entry","props":{"internkim_flow_entry":true}}}}`, nil)
	default:
		t.Fatalf("unexpected existing Flow setup request %s %s", request.Method, request.URL.String())
		return nil
	}
}

func mattermostPostMessage(t *testing.T, request *http.Request) string {
	t.Helper()
	var payload map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.TrimSpace(payload["message"].(string))
}

func flowNotificationTestTask(status string) flowTask {
	return flowTask{
		ID:               "task-1",
		OwnerID:          "member-1",
		OwnerName:        "김민수",
		ParticipantIDs:   []string{"member-1"},
		ParticipantNames: []string{"김민수"},
		Type:             "회의",
		Content:          "10분 회의",
		Goal:             "정리",
		Size:             "XS",
		Status:           status,
		WeekCode:         "26W18",
		RequestReason:    "검토 요청",
	}
}

func newFlowAuthorizationTestService(t *testing.T) *Service {
	t.Helper()
	deviceIDPath := writeTestFile(t, "device-1")
	deviceSecretPath := writeTestFile(t, "secret-1")
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
		DeviceIDPath:          deviceIDPath,
		DeviceSecretPath:      deviceSecretPath,
		FlowDatabasePath:      filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "https://api.example.test/api/users?device_id=device-1" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","name":"Admin","role":"admin","status":"active"},{"email":"staff@example.com","name":"Staff","role":"member","status":"active"},{"email":"other@example.com","name":"Other","role":"member","status":"active"}]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
}

func newFlowTaskRequest(callerEmail string, ownerEmail string) *http.Request {
	payload := flowTaskWriteRequest{
		OwnerID:        stableFlowID(ownerEmail),
		ParticipantIDs: []string{stableFlowID(ownerEmail)},
		Type:           "회의",
		Content:        "10분 회의",
		Size:           "XS",
		Status:         "진행",
		WeekCode:       "26W18",
	}
	document, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", bytes.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", callerEmail)
	return request
}

func newFlowDefinitionsRequest(callerEmail string) *http.Request {
	document := `{"types":["회의"],"sizes":[{"name":"XS","distanceKM":1,"maxHours":1,"developmentExample":"dev","otherExample":"other","note":"note"}]}`
	request := httptest.NewRequest(http.MethodPut, "/flow/api/definitions", strings.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", callerEmail)
	return request
}

func TestBotProfileDoesNotKeepLegacyDefaultPublicDescription(t *testing.T) {
	profile := normalizeBotProfile(botProfile{
		DisplayName:       "김인턴",
		PublicDescription: string([]byte{237, 154, 140, 236, 130, 172, 32, 236, 157, 188, 236, 157, 132, 32, 235, 185, 160, 235, 165, 180, 234, 178, 140, 32, 235, 143, 149, 235, 138, 148, 32, 65, 73, 32, 116, 101, 97, 109, 109, 97, 116, 101}),
	})
	if profile.PublicDescription != "" {
		t.Fatalf("public description = %q", profile.PublicDescription)
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

func isMattermostFlowSetupRequest(request *http.Request) bool {
	switch {
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members":
		return true
	default:
		return false
	}
}

func mattermostFlowSetupResponse(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	switch {
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
		return jsonResponse(http.StatusOK, `{"id":"flow-channel"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
		assertMattermostFlowChannelPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
		assertMattermostFlowEntryPost(t, request)
		return jsonResponse(http.StatusCreated, `{"id":"flow-entry"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil)
	default:
		t.Fatalf("unexpected Flow setup request %s %s", request.Method, request.URL.String())
		return nil
	}
}

func isMattermostConnectCommandSetupRequest(request *http.Request) bool {
	return request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim" ||
		request.URL.String() == "http://mattermost.local/api/v4/commands?team_id=team-1" ||
		request.URL.String() == "http://mattermost.local/api/v4/commands"
}

func mattermostConnectCommandSetupResponse(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	switch {
	case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/commands?team_id=team-1":
		return jsonResponse(http.StatusOK, `[]`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/commands" && request.Method == http.MethodPost:
		assertMattermostConnectCommandPayload(t, request)
		return jsonResponse(http.StatusCreated, `{"id":"connect-command","token":"connect-token","team_id":"team-1","trigger":"connect"}`, nil)
	default:
		t.Fatalf("unexpected Mattermost command setup request %s %s", request.Method, request.URL.String())
		return nil
	}
}

func assertMattermostConnectCommandPayload(t *testing.T, request *http.Request) {
	t.Helper()
	var payload mattermostCommandRecord
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload.TeamID != "team-1" || payload.Trigger != "connect" || payload.Method != "P" {
		t.Fatalf("connect command payload = %+v", payload)
	}
	if payload.URL != "http://127.0.0.1:18080/_internkim/mattermost/commands" || !payload.Autocomplete {
		t.Fatalf("connect command url/autocomplete = %+v", payload)
	}
}

func assertMattermostFlowChannelPatch(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["display_name"] != "Flow" || payload["header"] != mattermostFlowChannelLink || payload["purpose"] != mattermostFlowChannelLink {
		t.Fatalf("flow channel patch = %#v", payload)
	}
}

func assertMattermostFlowChannelCreate(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["team_id"] != "team-1" || payload["name"] != "flow" || payload["display_name"] != "Flow" || payload["type"] != "O" {
		t.Fatalf("flow channel create = %#v", payload)
	}
}

func assertMattermostChannelMember(t *testing.T, request *http.Request, expectedUserID string) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["user_id"] != expectedUserID {
		t.Fatalf("channel member = %#v", payload)
	}
}

func assertMattermostFlowEntryPost(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	props, _ := payload["props"].(map[string]any)
	if payload["channel_id"] != "flow-channel" || payload["message"] != mattermostFlowEntryPostMessage || props["internkim_flow_entry"] != true {
		t.Fatalf("flow entry post = %#v", payload)
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
			Context:       capabilities.ToolInvokeContext{RequesterEmail: "admin@example.com"},
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

func TestLocalCompanionPairingCodeStoresMattermostOwner(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()

	pairingRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pairing-codes", strings.NewReader(`{
		"ownerPlatform":"mattermost",
		"ownerPlatformUserID":"user-1",
		"ownerEmail":"Alice@Example.com",
		"ownerName":"Alice",
		"deviceURL":"https://device.example.com"
	}`))
	pairingRequest.RemoteAddr = "127.0.0.1:1234"
	pairingResponse := httptest.NewRecorder()
	handler.ServeHTTP(pairingResponse, pairingRequest)
	if pairingResponse.Code != http.StatusOK {
		t.Fatalf("pairing code status = %d body = %s", pairingResponse.Code, pairingResponse.Body.String())
	}
	var pairingCode companionPairingCodeResponse
	if errorValue := json.Unmarshal(pairingResponse.Body.Bytes(), &pairingCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(pairingCode.DeepLink, url.QueryEscape("https://device.example.com")) {
		t.Fatalf("expected deep link device url override, got %q", pairingCode.DeepLink)
	}

	pairRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{
		"code":"`+pairingCode.Code+`",
		"displayName":"Alice Mac",
		"publicKey":"test-key",
		"capabilities":[{"name":"llm.text"}]
	}`))
	pairResponse := httptest.NewRecorder()
	handler.ServeHTTP(pairResponse, pairRequest)
	if pairResponse.Code != http.StatusOK {
		t.Fatalf("pair status = %d body = %s", pairResponse.Code, pairResponse.Body.String())
	}

	statuses := service.companionStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected one companion, got %+v", statuses)
	}
	status := statuses[0]
	if status.OwnerPlatform != "mattermost" || status.OwnerPlatformUserID != "user-1" || status.OwnerEmail != "alice@example.com" || status.OwnerName != "Alice" {
		t.Fatalf("unexpected owner status: %+v", status)
	}
}

func TestMattermostConnectCommandRejectsInvalidToken(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	writeFile(t, service.mattermostConnectCommandTokenPath(), "expected-token")
	handler := service.router()

	form := url.Values{
		"command": []string{"/connect"},
		"token":   []string{"wrong-token"},
		"user_id": []string{"user-1"},
	}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/commands", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("connect command status = %d body = %s", response.Code, response.Body.String())
	}
	if len(service.pairingCodes) != 0 {
		t.Fatalf("unexpected pairing codes: %+v", service.pairingCodes)
	}
}

func TestMattermostConnectCommandCreatesEphemeralOwnerPairing(t *testing.T) {
	stateDirectory := t.TempDir()
	deviceIDPath := filepath.Join(stateDirectory, "device-id")
	adminPasswordPath := filepath.Join(stateDirectory, "admin-pass")
	writeFile(t, deviceIDPath, "dc719d8e")
	writeFile(t, adminPasswordPath, "admin-pass")
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		DeviceIDPath:                deviceIDPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	writeFile(t, service.mattermostConnectCommandTokenPath(), "connect-token")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"Alice@Example.com","username":"alice","nickname":"Alice"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	form := url.Values{
		"command":   []string{"/connect"},
		"token":     []string{"connect-token"},
		"user_id":   []string{"user-1"},
		"user_name": []string{"alice"},
	}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/commands", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("connect command status = %d body = %s", response.Code, response.Body.String())
	}
	var slashResponse mattermostSlashCommandResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&slashResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if slashResponse.ResponseType != "ephemeral" || slashResponse.Username != "김인턴" || slashResponse.IconURL != "https://dc719d8e.example.test/logo.svg" || !strings.Contains(slashResponse.Text, "Companion 연결 코드") {
		t.Fatalf("slash response = %+v", slashResponse)
	}
	if len(service.pairingCodes) != 1 {
		t.Fatalf("pairing codes = %+v", service.pairingCodes)
	}
	for _, pairingCode := range service.pairingCodes {
		if pairingCode.OwnerPlatform != "mattermost" || pairingCode.OwnerPlatformUserID != "user-1" || pairingCode.OwnerEmail != "alice@example.com" || pairingCode.OwnerName != "Alice" {
			t.Fatalf("pairing owner = %+v", pairingCode)
		}
		if !strings.Contains(slashResponse.Text, pairingCode.Code) || !strings.Contains(slashResponse.Text, "[Companion 앱 열기](internkim://pair?") || !strings.Contains(slashResponse.Text, url.QueryEscape("https://dc719d8e.example.test")) {
			t.Fatalf("slash response text = %q", slashResponse.Text)
		}
	}
}

func TestMattermostConnectCommandProvisioningCreatesCommandToken(t *testing.T) {
	service := NewService(Configuration{
		StateDirectory:    t.TempDir(),
		MattermostBaseURL: "http://mattermost.local",
		AdminEmailPath:    writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isMattermostConnectCommandSetupRequest(request) {
			return mattermostConnectCommandSetupResponse(t, request), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	if errorValue := service.ensureMattermostConnectCommand(context.Background(), "admin-token"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.TrimSpace(readTrimmedFile(service.mattermostConnectCommandTokenPath())) != "connect-token" {
		t.Fatalf("stored command token = %q", readTrimmedFile(service.mattermostConnectCommandTokenPath()))
	}
}

func TestMattermostProvisionerAccountCreatesDefaultFlowChannel(t *testing.T) {
	stateDirectory := t.TempDir()
	envDirectory := filepath.Join(stateDirectory, "env")
	if errorValue := os.MkdirAll(envDirectory, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	adminPasswordPath := filepath.Join(stateDirectory, "admin-pass")
	deviceIDPath := filepath.Join(envDirectory, "device-id")
	writeFile(t, adminPasswordPath, "admin-pass")
	writeFile(t, deviceIDPath, "device-1")
	flowChannelCreated := false
	flowChannelPatched := false
	flowEntryPostCreated := false
	adminJoinedFlowChannel := false
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		DeviceIDPath:                deviceIDPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"town-square-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels" && request.Method == http.MethodPost:
			flowChannelCreated = true
			assertMattermostFlowChannelCreate(t, request)
			return jsonResponse(http.StatusCreated, `{"id":"flow-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
			flowChannelPatched = true
			assertMattermostFlowChannelPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			flowEntryPostCreated = true
			assertMattermostFlowEntryPost(t, request)
			return jsonResponse(http.StatusCreated, `{"id":"flow-entry"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members" && request.Method == http.MethodPost:
			adminJoinedFlowChannel = true
			assertMattermostChannelMember(t, request, "admin")
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostConnectCommandSetupRequest(request):
			return mattermostConnectCommandSetupResponse(t, request), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.ensureMattermostProvisionerDefaults(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !flowChannelCreated || !flowChannelPatched || !flowEntryPostCreated || !adminJoinedFlowChannel {
		t.Fatalf("flow setup flags created=%v patched=%v posted=%v joined=%v", flowChannelCreated, flowChannelPatched, flowEntryPostCreated, adminJoinedFlowChannel)
	}
}

func TestMattermostConnectCommandProvisioningRecreatesCommandWithoutToken(t *testing.T) {
	service := NewService(Configuration{
		StateDirectory:    t.TempDir(),
		MattermostBaseURL: "http://mattermost.local",
		AdminEmailPath:    writeTestFile(t, "admin@example.com"),
	})
	archivedCommand := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/commands?team_id=team-1":
			return jsonResponse(http.StatusOK, `[{"id":"old-command","team_id":"team-1","trigger":"connect"}]`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/commands/old-command" && request.Method == http.MethodDelete:
			archivedCommand = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/commands" && request.Method == http.MethodPost:
			assertMattermostConnectCommandPayload(t, request)
			return jsonResponse(http.StatusCreated, `{"id":"connect-command","token":"new-token","team_id":"team-1","trigger":"connect"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.ensureMattermostConnectCommand(context.Background(), "admin-token"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !archivedCommand {
		t.Fatal("old command was not archived")
	}
	if strings.TrimSpace(readTrimmedFile(service.mattermostConnectCommandTokenPath())) != "new-token" {
		t.Fatalf("stored command token = %q", readTrimmedFile(service.mattermostConnectCommandTokenPath()))
	}
}

func TestCompanionLLMJobWithRequesterOnlyClaimsRequesterOwner(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	alice := &CompanionRecord{
		CompanionID:  "alice-companion",
		OwnerEmail:   "alice@example.com",
		Capabilities: capabilities.CompanionLLMDescriptors(),
		LastSeenAt:   now,
	}
	bob := &CompanionRecord{
		CompanionID:  "bob-companion",
		OwnerEmail:   "bob@example.com",
		Capabilities: capabilities.CompanionLLMDescriptors(),
		LastSeenAt:   now,
	}
	job := &CompanionJob{
		JobID:          "job-1",
		Status:         "pending",
		ToolName:       "llm.text",
		PrivacyClass:   "model_input",
		RequesterEmail: "alice@example.com",
		Request: capabilities.ToolInvokeRequest{
			ToolName: "llm.text",
			Context: capabilities.ToolInvokeContext{
				RequesterEmail: "alice@example.com",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}
	service.companions[alice.CompanionID] = alice
	service.companions[bob.CompanionID] = bob
	service.companionJobs[job.JobID] = job

	if claimedJob := service.claimNextCompanionJob(bob); claimedJob != nil {
		t.Fatalf("expected Bob companion not to claim Alice job, got %+v", claimedJob)
	}
	claimedJob := service.claimNextCompanionJob(alice)
	if claimedJob == nil || claimedJob.CompanionID != alice.CompanionID {
		t.Fatalf("expected Alice companion to claim job, got %+v", claimedJob)
	}
}

func TestCompanionAuthCheckRequiresSignedCompanion(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()
	_, pairResult := pairTestCompanion(t, handler, keyPairCapabilityRequest{
		KeyPair:    keyPairForTest(t),
		Capability: `{"name":"user.confirm","version":"1","privacyClass":"user_input","estimatedLatency":"interactive","requiresUserPresence":true,"worksOffline":true}`,
	})

	unsignedResponse := httptest.NewRecorder()
	unsignedRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/auth/check", nil)
	unsignedRequest.Header.Set("X-InternKim-Companion-ID", pairResult.CompanionID)
	unsignedRequest.Header.Set("X-InternKim-Companion-Token", pairResult.Token)
	handler.ServeHTTP(unsignedResponse, unsignedRequest)
	if unsignedResponse.Code != http.StatusForbidden {
		t.Fatalf("expected unsigned auth check to fail, got %d", unsignedResponse.Code)
	}

	signedResponse := httptest.NewRecorder()
	signedRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/auth/check", nil)
	setCompanionHeaders(t, signedRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(signedResponse, signedRequest)
	if signedResponse.Code != http.StatusOK {
		t.Fatalf("expected signed auth check to succeed, got %d: %s", signedResponse.Code, signedResponse.Body.String())
	}
	var document map[string]string
	if errorValue := json.NewDecoder(signedResponse.Body).Decode(&document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document["status"] != "ok" || document["companionID"] != pairResult.CompanionID {
		t.Fatalf("unexpected auth check response: %+v", document)
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
			Context:       capabilities.ToolInvokeContext{RequesterEmail: "admin@example.com"},
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

func TestCompanionJobRequiresRequesterIdentityForUserLocalTool(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.companions["companion-1"] = &CompanionRecord{
		CompanionID: "companion-1",
		OwnerEmail:  "admin@example.com",
		Capabilities: []capabilities.Descriptor{
			{Name: "browser.handoff"},
		},
		LastSeenAt: time.Now().UTC(),
	}

	response, errorValue := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
		ToolName:      "browser.handoff",
		Input:         json.RawMessage(`{"url":"https://example.com"}`),
		TimeoutSecond: 1,
	})

	if errorValue != nil {
		t.Fatalf("expected structured denial: %v", errorValue)
	}
	var denial capabilities.DenialResult
	if errorValue := json.Unmarshal(response.Result, &denial); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "denied" || denial.Code != capabilities.CapabilityNotAllowed {
		t.Fatalf("expected not_allowed denial, got response=%+v denial=%+v", response, denial)
	}
}

func TestCompanionJobReportsNotReadyWhenOwnerBrowserCapabilityMissing(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.companions["companion-1"] = &CompanionRecord{
		CompanionID: "companion-1",
		OwnerEmail:  "admin@example.com",
		Capabilities: []capabilities.Descriptor{
			{Name: "user.confirm"},
		},
		LastSeenAt: time.Now().UTC(),
	}

	response, errorValue := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
		ToolName:     "browser.open",
		Input:        json.RawMessage(`{"url":"https://example.com"}`),
		Context:      capabilities.ToolInvokeContext{RequesterEmail: "admin@example.com"},
		PrivacyClass: "user_browser",
	})

	if errorValue != nil {
		t.Fatalf("expected structured denial: %v", errorValue)
	}
	var denial capabilities.DenialResult
	if errorValue := json.Unmarshal(response.Result, &denial); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "denied" || denial.Code != capabilities.CapabilityNotReady || strings.Contains(response.Content, "/connect") {
		t.Fatalf("expected not_ready denial without reconnect advice, got response=%+v denial=%+v", response, denial)
	}
	if denial.Recovery != nil {
		t.Fatalf("expected not_ready denial without recovery action, got %+v", denial.Recovery)
	}
}

func TestCompanionJobExpiryReportsNotReady(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.companions["companion-1"] = &CompanionRecord{
		CompanionID: "companion-1",
		OwnerEmail:  "admin@example.com",
		Capabilities: []capabilities.Descriptor{
			{Name: "browser.open"},
		},
		LastSeenAt: time.Now().UTC(),
	}

	response, errorValue := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
		ToolName:      "browser.open",
		Input:         json.RawMessage(`{"url":"https://example.com"}`),
		Context:       capabilities.ToolInvokeContext{RequesterEmail: "admin@example.com"},
		PrivacyClass:  "user_browser",
		TimeoutSecond: 1,
	})

	if errorValue != nil {
		t.Fatalf("expected structured timeout denial: %v", errorValue)
	}
	var denial capabilities.DenialResult
	if errorValue := json.Unmarshal(response.Result, &denial); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "denied" || denial.Code != capabilities.CapabilityNotReady || denial.Recovery != nil {
		t.Fatalf("expected not_ready denial without recovery, got response=%+v denial=%+v", response, denial)
	}
}

func TestCompanionJobClaimRequiresMatchingOwner(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	aliceCompanion := &CompanionRecord{
		CompanionID: "alice-companion",
		OwnerEmail:  "alice@example.com",
		Capabilities: []capabilities.Descriptor{
			{Name: "browser.handoff"},
		},
		LastSeenAt: now,
	}
	bobCompanion := &CompanionRecord{
		CompanionID: "bob-companion",
		OwnerEmail:  "bob@example.com",
		Capabilities: []capabilities.Descriptor{
			{Name: "browser.handoff"},
		},
		LastSeenAt: now,
	}
	service.companions[aliceCompanion.CompanionID] = aliceCompanion
	service.companions[bobCompanion.CompanionID] = bobCompanion
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:          "job-1",
		Status:         "pending",
		RequesterEmail: "alice@example.com",
		ToolName:       "browser.handoff",
		Request:        capabilities.ToolInvokeRequest{ToolName: "browser.handoff"},
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      now.Add(time.Minute),
	}

	if claimedJob := service.claimNextCompanionJob(bobCompanion); claimedJob != nil {
		t.Fatalf("expected Bob companion not to claim Alice job, got %+v", claimedJob)
	}
	claimedJob := service.claimNextCompanionJob(aliceCompanion)
	if claimedJob == nil || claimedJob.JobID != "job-1" || claimedJob.CompanionID != aliceCompanion.CompanionID {
		t.Fatalf("expected Alice companion to claim Alice job, got %+v", claimedJob)
	}
}

func TestCompanionJobClaimMatchesPlatformUserIDOwner(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	companion := &CompanionRecord{
		CompanionID:         "mattermost-companion",
		OwnerPlatform:       "mattermost",
		OwnerPlatformUserID: "mattermost-user-1",
		Capabilities: []capabilities.Descriptor{
			{Name: "browser.open"},
		},
		LastSeenAt: now,
	}
	service.companions[companion.CompanionID] = companion
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:    "job-1",
		Status:   "pending",
		ToolName: "browser.open",
		Request: capabilities.ToolInvokeRequest{
			ToolName: "browser.open",
			Context: capabilities.ToolInvokeContext{
				RequesterPlatformUserID: "mattermost-user-1",
				Platform:                "mattermost",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}

	claimedJob := service.claimNextCompanionJob(companion)
	if claimedJob == nil || claimedJob.JobID != "job-1" || claimedJob.CompanionID != companion.CompanionID {
		t.Fatalf("expected platform-owned companion to claim job, got %+v", claimedJob)
	}
}

func TestCompanionBrowserResourceScopeFallsBackToParentOrigin(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.companionJobs["parent-job"] = &CompanionJob{
		JobID:         "parent-job",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://example.com"},
	}

	resourceScope := service.inferCompanionResourceScope(capabilities.ToolInvokeRequest{
		ToolName:    "browser.click",
		ParentJobID: "parent-job",
		Input:       json.RawMessage(`{"target":"@e1"}`),
	})

	if resourceScope.Kind != "web_origin" || resourceScope.Value != "https://example.com" {
		t.Fatalf("expected parent web origin, got %+v", resourceScope)
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

func TestCompanionWatchCreatesOwnerLocalAttentionJob(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	companion := &CompanionRecord{
		CompanionID:  "alice-companion",
		OwnerEmail:   "alice@example.com",
		Capabilities: append([]capabilities.Descriptor{{Name: "user.confirm"}}, capabilities.CompanionLLMDescriptors()...),
		LastSeenAt:   now,
	}
	service.companions[companion.CompanionID] = companion
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:             "job-1",
		Status:            "pending",
		RequesterEmail:    "alice@example.com",
		ToolName:          "user.confirm",
		PrivacyClass:      "user_input",
		WatchStatus:       companionWatchStatusOpen,
		NextWatchAt:       now.Add(-time.Second),
		WatchAttemptCount: 0,
		Request: capabilities.ToolInvokeRequest{
			ToolName: "user.confirm",
			Context:  capabilities.ToolInvokeContext{RequesterEmail: "alice@example.com"},
		},
		CreatedAt: now.Add(-6 * time.Minute),
		UpdatedAt: now.Add(-6 * time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}

	claimedJob := service.claimNextCompanionJob(companion)

	if claimedJob == nil || claimedJob.ToolName != capabilities.AttentionTriageToolName || claimedJob.ParentJobID != "job-1" {
		t.Fatalf("expected attention triage job, got %+v", claimedJob)
	}
	parentJob := service.companionJobs["job-1"]
	if parentJob.WatchAttemptCount != 1 || !parentJob.NextWatchAt.IsZero() {
		t.Fatalf("expected parent watch to move into local triage, got %+v", parentJob)
	}
	var triageRequest capabilities.AttentionTriageRequest
	if errorValue := json.Unmarshal(claimedJob.Request.Input, &triageRequest); errorValue != nil {
		t.Fatal(errorValue)
	}
	if triageRequest.JobID != "job-1" || triageRequest.ToolName != "user.confirm" || triageRequest.WatchAttemptCount != 1 {
		t.Fatalf("unexpected triage request: %+v", triageRequest)
	}
}

func TestCompanionWatchRoutesOnlyToOwningCompanion(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	aliceCompanion := &CompanionRecord{
		CompanionID:  "alice-companion",
		OwnerEmail:   "alice@example.com",
		Capabilities: capabilities.CompanionLLMDescriptors(),
		LastSeenAt:   now,
	}
	bobCompanion := &CompanionRecord{
		CompanionID:  "bob-companion",
		OwnerEmail:   "bob@example.com",
		Capabilities: capabilities.CompanionLLMDescriptors(),
		LastSeenAt:   now,
	}
	service.companions[aliceCompanion.CompanionID] = aliceCompanion
	service.companions[bobCompanion.CompanionID] = bobCompanion
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:          "job-1",
		Status:         "running",
		CompanionID:    aliceCompanion.CompanionID,
		RequesterEmail: "alice@example.com",
		ToolName:       "browser.handoff",
		PrivacyClass:   "user_browser",
		WatchStatus:    companionWatchStatusOpen,
		NextWatchAt:    now.Add(-time.Second),
		Request: capabilities.ToolInvokeRequest{
			ToolName: "browser.handoff",
			Context:  capabilities.ToolInvokeContext{RequesterEmail: "alice@example.com"},
		},
		CreatedAt: now.Add(-6 * time.Minute),
		UpdatedAt: now.Add(-6 * time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}

	if claimedJob := service.claimNextCompanionJob(bobCompanion); claimedJob != nil {
		t.Fatalf("expected Bob not to claim Alice attention watch, got %+v", claimedJob)
	}
	claimedJob := service.claimNextCompanionJob(aliceCompanion)
	if claimedJob == nil || claimedJob.ToolName != capabilities.AttentionTriageToolName {
		t.Fatalf("expected Alice attention triage claim, got %+v", claimedJob)
	}
}

func TestCompanionWatchFallsBackWithoutLocalAttentionModel(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	companion := &CompanionRecord{
		CompanionID:  "alice-companion",
		OwnerEmail:   "alice@example.com",
		Capabilities: []capabilities.Descriptor{{Name: "user.confirm"}},
		LastSeenAt:   now,
	}
	service.companions[companion.CompanionID] = companion
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:          "job-1",
		Status:         "running",
		CompanionID:    companion.CompanionID,
		RequesterEmail: "alice@example.com",
		ToolName:       "user.confirm",
		PrivacyClass:   "user_input",
		WatchStatus:    companionWatchStatusOpen,
		NextWatchAt:    now.Add(-time.Second),
		Request: capabilities.ToolInvokeRequest{
			ToolName: "user.confirm",
			Context:  capabilities.ToolInvokeContext{RequesterEmail: "alice@example.com"},
		},
		CreatedAt: now.Add(-6 * time.Minute),
		UpdatedAt: now.Add(-6 * time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}

	if claimedJob := service.claimNextCompanionJob(companion); claimedJob != nil {
		t.Fatalf("expected fallback to avoid local triage job, got %+v", claimedJob)
	}
	job := service.companionJobs["job-1"]
	if job.WatchAttemptCount != 1 || job.Attention == nil || job.Attention.LocalDecision == nil {
		t.Fatalf("expected deterministic fallback decision, got %+v", job)
	}
	if len(job.Attention.LocalDecision.ReasonCodes) != 1 || job.Attention.LocalDecision.ReasonCodes[0] != "local_triage_unavailable" {
		t.Fatalf("unexpected fallback reason: %+v", job.Attention.LocalDecision)
	}
	if job.NextWatchAt.Before(time.Now().Add(9*time.Minute)) || job.NextWatchAt.After(time.Now().Add(11*time.Minute)) {
		t.Fatalf("expected second watch around 10m, got %s", job.NextWatchAt)
	}
}

func TestCompanionAttentionCompletionSchedulesBackoffAndStoresNoReplyTarget(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:             "job-1",
		Status:            "running",
		CompanionID:       "alice-companion",
		RequesterEmail:    "alice@example.com",
		ToolName:          "user.confirm",
		PrivacyClass:      "user_input",
		WatchStatus:       companionWatchStatusOpen,
		WatchAttemptCount: 1,
		Request: capabilities.ToolInvokeRequest{
			ToolName: "user.confirm",
			Context:  capabilities.ToolInvokeContext{RequesterEmail: "alice@example.com"},
		},
		CreatedAt: now.Add(-6 * time.Minute),
		UpdatedAt: now.Add(-6 * time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}
	decisionDocument := json.RawMessage(`{
		"shouldEscalate":true,
		"importance":"high",
		"confidence":0.9,
		"reasonCodes":["blocked"],
		"summaryForRemote":"Waiting for a user confirmation.",
		"privacyClass":"user_input"
	}`)
	service.companionJobs["attention-1"] = &CompanionJob{
		JobID:          "attention-1",
		ParentJobID:    "job-1",
		Status:         "running",
		CompanionID:    "alice-companion",
		RequesterEmail: "alice@example.com",
		ToolName:       capabilities.AttentionTriageToolName,
		PrivacyClass:   "model_input",
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      now.Add(time.Minute),
	}

	errorValue := service.finishCompanionJob("alice-companion", "attention-1", &capabilities.ToolInvokeResponse{
		ToolName: capabilities.AttentionTriageToolName,
		Result:   decisionDocument,
	}, "")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	parentJob := service.companionJobs["job-1"]
	if parentJob.Attention == nil || parentJob.Attention.RemoteStatus != companionAttentionRemoteStored {
		t.Fatalf("expected stored attention without reply target, got %+v", parentJob.Attention)
	}
	if parentJob.NextWatchAt.Before(time.Now().Add(9*time.Minute)) || parentJob.NextWatchAt.After(time.Now().Add(11*time.Minute)) {
		t.Fatalf("expected second watch around 10m, got %s", parentJob.NextWatchAt)
	}
}

func TestCompanionTerminalJobClosesWatch(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	now := time.Now().UTC()
	service.companionJobs["job-1"] = &CompanionJob{
		JobID:       "job-1",
		Status:      "running",
		CompanionID: "alice-companion",
		ToolName:    "user.confirm",
		WatchStatus: companionWatchStatusOpen,
		NextWatchAt: now.Add(time.Minute),
		Request:     capabilities.ToolInvokeRequest{ToolName: "user.confirm"},
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(time.Hour),
	}

	errorValue := service.finishCompanionJob("alice-companion", "job-1", &capabilities.ToolInvokeResponse{ToolName: "user.confirm"}, "")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	job := service.companionJobs["job-1"]
	if job.WatchStatus != companionWatchStatusClosed || !job.NextWatchAt.IsZero() {
		t.Fatalf("expected terminal job to close watch, got %+v", job)
	}
}

func TestCompanionRemoteModelReadAndUpdate(t *testing.T) {
	runtimeConfigPath := filepath.Join(t.TempDir(), "runtime.json")
	writeFile(t, runtimeConfigPath, `{"languageModel":{"capability":{"model":"google/old-model"}}}`)
	service := NewService(Configuration{
		StateDirectory:            t.TempDir(),
		AdminEmailPath:            writeTestFile(t, "admin@example.com"),
		BlueclawRuntimeConfigPath: runtimeConfigPath,
	})
	restartCalls := 0
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		_ = ctx
		if name == "systemctl" && strings.Join(arguments, " ") == "restart blueclaw" {
			restartCalls++
			return nil, nil
		}
		t.Fatalf("unexpected command: %s %s", name, strings.Join(arguments, " "))
		return nil, nil
	}
	handler := service.router()
	keyPair, pairResult := pairTestCompanion(t, handler, keyPairCapabilityRequest{
		KeyPair:    keyPairForTest(t),
		Capability: `{"name":"user.confirm","version":"1","privacyClass":"user_input","estimatedLatency":"interactive","requiresUserPresence":true,"worksOffline":true}`,
	})
	_ = keyPair

	readResponse := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/remote-model", nil)
	setCompanionHeaders(t, readRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(readResponse, readRequest)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read status = %d: %s", readResponse.Code, readResponse.Body.String())
	}
	var readResult companionRemoteModelResponse
	if errorValue := json.NewDecoder(readResponse.Body).Decode(&readResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	if readResult.Model != "google/old-model" {
		t.Fatalf("expected old model, got %+v", readResult)
	}

	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/_internkim/companion/remote-model", strings.NewReader(`{"model":"google/new-model"}`))
	setCompanionHeaders(t, updateRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", updateResponse.Code, updateResponse.Body.String())
	}
	if restartCalls != 1 {
		t.Fatalf("expected one blueclaw restart, got %d", restartCalls)
	}
	updatedDocument := readJSONFile(t, runtimeConfigPath)
	if remoteModelFromRuntimeDocument(updatedDocument) != "google/new-model" {
		t.Fatalf("expected updated model, got %+v", updatedDocument)
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
			Context:       capabilities.ToolInvokeContext{RequesterEmail: "admin@example.com"},
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

func requestAdminSession(t *testing.T, service *Service, email string) map[string]any {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/session", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", email)
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin session status = %d body = %s", response.Code, response.Body.String())
	}
	var document map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func assertFirstAdminPasswordPolicyPatch(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	passwordSettings := payload["PasswordSettings"]
	if passwordSettings["MinimumLength"] != float64(5) {
		t.Fatalf("minimum password length = %#v", passwordSettings["MinimumLength"])
	}
	for _, key := range []string{"Lowercase", "Uppercase", "Number", "Symbol"} {
		if passwordSettings[key] != false {
			t.Fatalf("password setting %s = %#v", key, passwordSettings[key])
		}
	}
}

func assertMattermostNicknameDisplayPatch(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	teamSettings := payload["TeamSettings"]
	if teamSettings["TeammateNameDisplay"] != "nickname_full_name" {
		t.Fatalf("teammate name display = %#v", teamSettings["TeammateNameDisplay"])
	}
}

func assertBotDirectChannelShown(t *testing.T, request *http.Request, userID string, botID string) {
	t.Helper()
	var preferences []mattermostPreferenceRecord
	if errorValue := json.NewDecoder(request.Body).Decode(&preferences); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(preferences) != 1 {
		t.Fatalf("preferences = %#v", preferences)
	}
	preference := preferences[0]
	if preference.UserID != userID || preference.Category != "direct_channel_show" || preference.Name != botID || preference.Value != "true" {
		t.Fatalf("direct channel preference = %#v", preference)
	}
}

func isBlueclawInviteRequest(t *testing.T, request *http.Request, expectedEmail string) bool {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.String() != "http://127.0.0.1:8080/admin/api/people/invite" {
		return false
	}
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["email"] != expectedEmail {
		t.Fatalf("Blueclaw invite payload = %#v", payload)
	}
	return true
}

func isBlueclawRemoveRequest(t *testing.T, request *http.Request, expectedEmail string) bool {
	t.Helper()
	if request.Method != http.MethodDelete || request.URL.Path != "/admin/api/people" {
		return false
	}
	if request.URL.Query().Get("email") != expectedEmail {
		t.Fatalf("Blueclaw remove email = %q", request.URL.Query().Get("email"))
	}
	return true
}

func isBlueclawPolicyGet(request *http.Request) bool {
	return request.Method == http.MethodGet && request.URL.String() == "http://127.0.0.1:8080/admin/api/policy"
}

func isBlueclawAdminPolicySave(t *testing.T, request *http.Request, expectedEmail string) bool {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.String() != "http://127.0.0.1:8080/admin/api/policy/save" {
		return false
	}
	var policyDocument map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&policyDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	people, _ := policyDocument["people"].([]any)
	if len(people) == 0 {
		t.Fatalf("Blueclaw policy people = %#v", policyDocument["people"])
	}
	adminPerson, _ := people[0].(map[string]any)
	if adminPerson["personID"] != "00000000-0000-0000-0000-000000000001" || adminPerson["isAdmin"] != true {
		t.Fatalf("Blueclaw admin person = %#v", adminPerson)
	}
	adminEmails, _ := adminPerson["emails"].([]any)
	if len(adminEmails) != 1 || adminEmails[0] != expectedEmail {
		t.Fatalf("Blueclaw admin emails = %#v", adminPerson["emails"])
	}
	for _, value := range people[1:] {
		person, _ := value.(map[string]any)
		for _, emailValue := range personEmailsForTest(person) {
			if emailValue == expectedEmail {
				t.Fatalf("claimed admin left duplicated as member: %#v", policyDocument)
			}
		}
	}
	return true
}

func personEmailsForTest(person map[string]any) []string {
	values, _ := person["emails"].([]any)
	emails := make([]string, 0, len(values))
	for _, value := range values {
		email, _ := value.(string)
		if email != "" {
			emails = append(emails, email)
		}
	}
	return emails
}

func blueclawPolicyWithSeedAdmin() string {
	return `{"people":[{"personID":"00000000-0000-0000-0000-000000000001","displayName":"Intern Kim Admin","emails":["gamyeong0720@example.com"],"securityLevelName":"admin","securityLevelRank":100,"grantedClasses":["internal","executive"],"isAdmin":true}],"channels":[],"retention":{"rawEventDays":60}}`
}

func blueclawPolicyWithClaimedMember() string {
	return `{"people":[{"personID":"00000000-0000-0000-0000-000000000001","displayName":"Intern Kim Admin","emails":["gamyeong0720@example.com"],"securityLevelName":"admin","securityLevelRank":100,"grantedClasses":["internal","executive"],"isAdmin":true},{"personID":"member-1","displayName":"lee","emails":["lee@example.com"],"securityLevelName":"member","securityLevelRank":10,"grantedClasses":["internal"],"isAdmin":false}],"channels":[],"retention":{"rawEventDays":60}}`
}

func writeTestFile(t *testing.T, document string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result map[string]any
	if errorValue := json.Unmarshal(document, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	return result
}

func TestCompanionJobTimeoutSecond(t *testing.T) {
	tests := []struct {
		name     string
		request  capabilities.ToolInvokeRequest
		expected int
	}{
		{
			name:     "non interactive default",
			request:  capabilities.ToolInvokeRequest{ToolName: "filesystem.mount.list"},
			expected: 30,
		},
		{
			name:     "browser default allows user approval",
			request:  capabilities.ToolInvokeRequest{ToolName: "browser.open"},
			expected: 120,
		},
		{
			name:     "user presence default allows user approval",
			request:  capabilities.ToolInvokeRequest{ToolName: "user.confirm", RequiresUserPresence: true},
			expected: 120,
		},
		{
			name:     "explicit timeout is preserved",
			request:  capabilities.ToolInvokeRequest{ToolName: "browser.open", TimeoutSecond: 45},
			expected: 45,
		},
		{
			name:     "explicit timeout is capped",
			request:  capabilities.ToolInvokeRequest{ToolName: "browser.open", TimeoutSecond: 500},
			expected: 300,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := companionJobTimeoutSecond(test.request)
			if actual != test.expected {
				t.Fatalf("timeout = %d, want %d", actual, test.expected)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
