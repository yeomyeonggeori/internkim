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
		APIBaseURL:            "https://api.intern.kim",
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
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"setup@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
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
	request.Header.Set("Cf-Access-Authenticated-User-Email", "lee@dawn.kim")
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
		APIBaseURL:                  "https://api.intern.kim",
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
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@dawn.kim","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertFirstAdminPasswordPolicyPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@dawn.kim":
			return jsonResponse(http.StatusNotFound, `{"message":"not found"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"user-1","email":"lee@dawn.kim","username":"lee"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
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
		case isBlueclawAdminPolicySave(t, request, "lee@dawn.kim"):
			blueclawInvited = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "lee@dawn.kim")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin page status = %d body = %s", response.Code, response.Body.String())
	}
	if strings.TrimSpace(readTrimmedFile(claimedAdminEmailPath)) != "lee@dawn.kim" {
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
		APIBaseURL:            "https://api.intern.kim",
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
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@dawn.kim","role":"admin"}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseDocument := requestAdminSession(t, service, "lee@dawn.kim")
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
		APIBaseURL:                  "https://api.intern.kim",
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
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
			var payload adminUserMutation
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.Email != "lee@dawn.kim" || payload.Role != "admin" {
				t.Fatalf("unexpected user role payload: %+v", payload)
			}
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@dawn.kim","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertFirstAdminPasswordPolicyPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@dawn.kim":
			return jsonResponse(http.StatusNotFound, `{"message":"not found"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users" && request.Method == http.MethodPost:
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["email"] != "lee@dawn.kim" || payload["password"] == "" {
				t.Fatalf("unexpected Mattermost create payload: %#v", payload)
			}
			createdMattermostPassword = payload["password"]
			return jsonResponse(http.StatusCreated, `{"id":"user-1","email":"lee@dawn.kim","username":"lee"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
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
		case isBlueclawAdminPolicySave(t, request, "lee@dawn.kim"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	firstResponse := requestAdminSession(t, service, "lee@dawn.kim")
	if createdMattermostPassword != firstAdminMattermostPassword {
		t.Fatalf("first admin Mattermost password = %q", createdMattermostPassword)
	}
	if firstResponse["temporaryPassword"] != firstAdminMattermostPassword {
		t.Fatalf("temporary password not returned: response=%#v created=%q", firstResponse, createdMattermostPassword)
	}
	if firstResponse["temporaryPasswordEmail"] != "lee@dawn.kim" {
		t.Fatalf("temporary password email = %#v", firstResponse["temporaryPasswordEmail"])
	}

	secondResponse := requestAdminSession(t, service, "lee@dawn.kim")
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
		APIBaseURL:                  "https://api.intern.kim",
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
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@dawn.kim","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertFirstAdminPasswordPolicyPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@dawn.kim":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"lee@dawn.kim","username":"lee"}`, nil), nil
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
		case isBlueclawAdminPolicySave(t, request, "lee@dawn.kim"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	response := requestAdminSession(t, service, "lee@dawn.kim")
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
	writeFile(t, claimedAdminEmailPath, "lee@dawn.kim")
	writeFile(t, adminPasswordPath, "admin-pass")
	writeFile(t, filepath.Join(stateDirectory, "first-admin-bootstrap.json"), `{"email":"lee@dawn.kim","status":"claimed"}`)

	passwordReset := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.intern.kim",
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
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/lee@dawn.kim":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"lee@dawn.kim","username":"lee"}`, nil), nil
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
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@dawn.kim","role":"admin"}]}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, blueclawPolicyWithClaimedMember(), nil), nil
		case isBlueclawAdminPolicySave(t, request, "lee@dawn.kim"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	response := requestAdminSession(t, service, "lee@dawn.kim")
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
		APIBaseURL:            "https://api.intern.kim",
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
		ClaimedAdminEmailPath:       writeTestFile(t, "admin@example.com"),
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
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostFullNameDisplayPatch(t, request)
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
			if payload["username"] != "member" || payload["first_name"] != "Member" || payload["last_name"] != "One" || payload["nickname"] != "Member One" {
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
		APIBaseURL:                  "https://api.intern.kim",
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
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"oldhandle","name":"Old Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"oldhandle"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostFullNameDisplayPatch(t, request)
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
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			assertBotDirectChannelShown(t, request, "user-1", "bot-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
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
	if mattermostPatch["username"] != "newhandle" || mattermostPatch["first_name"] != "New" || mattermostPatch["last_name"] != "Name" || mattermostPatch["nickname"] != "New Name" {
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
		APIBaseURL:                  "https://api.intern.kim",
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
		case request.URL.String() == "https://api.intern.kim/api/users?device_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostFullNameDisplayPatch(t, request)
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
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
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
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","first_name":"Intern Kim","nickname":"Intern Kim","roles":"system_user"}`, nil), nil
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
	if !strings.Contains(patchBody, `"first_name":"김비서"`) || !strings.Contains(patchBody, `"position":"업무를 빠르게 돕습니다"`) {
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

	_, errorValue := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
		ToolName:      "browser.handoff",
		Input:         json.RawMessage(`{"url":"https://example.com"}`),
		TimeoutSecond: 1,
	})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "requester identity is required") {
		t.Fatalf("expected requester identity error, got %v", errorValue)
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

func assertMattermostFullNameDisplayPatch(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	teamSettings := payload["TeamSettings"]
	if teamSettings["TeammateNameDisplay"] != "full_name" {
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
	return `{"people":[{"personID":"00000000-0000-0000-0000-000000000001","displayName":"Intern Kim Admin","emails":["eastriver0720@gmail.com"],"securityLevelName":"admin","securityLevelRank":100,"grantedClasses":["internal","executive"],"isAdmin":true}],"channels":[],"retention":{"rawEventDays":60}}`
}

func blueclawPolicyWithClaimedMember() string {
	return `{"people":[{"personID":"00000000-0000-0000-0000-000000000001","displayName":"Intern Kim Admin","emails":["eastriver0720@gmail.com"],"securityLevelName":"admin","securityLevelRank":100,"grantedClasses":["internal","executive"],"isAdmin":true},{"personID":"member-1","displayName":"lee","emails":["lee@dawn.kim"],"securityLevelName":"member","securityLevelRank":10,"grantedClasses":["internal"],"isAdmin":false}],"channels":[],"retention":{"rawEventDays":60}}`
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
