package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
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
	for _, expectedText := range []string{"\"status\":\"ok\"", "admindBuildID", "gitRevision", "recoveryAvailable"} {
		if !strings.Contains(adminResponse.Body.String(), expectedText) {
			t.Fatalf("expected admin health to include %q, got %s", expectedText, adminResponse.Body.String())
		}
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

func TestAdminRequestMetricsClassifiesAndRecordsSlowRequests(t *testing.T) {
	service := NewService(Configuration{})
	staticRequest := httptest.NewRequest(http.MethodGet, "/flow/", nil)
	apiRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/events", nil)
	writeRequest := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", nil)
	proxyRequest := httptest.NewRequest(http.MethodGet, "/team/channels/town-square", nil)

	if adminEndpointClassForRequest(staticRequest) != adminEndpointStatic {
		t.Fatalf("expected static class")
	}
	if adminEndpointClassForRequest(apiRequest) != adminEndpointReadAPI {
		t.Fatalf("expected read api class")
	}
	if adminEndpointClassForRequest(writeRequest) != adminEndpointWriteAPI {
		t.Fatalf("expected write api class")
	}
	if adminEndpointClassForRequest(proxyRequest) != adminEndpointProxy {
		t.Fatalf("expected proxy class")
	}

	service.recordAdminRequest(staticRequest, http.StatusOK, 350*time.Millisecond, time.Now())
	records := service.requestMetrics.RecentSlowRequests()
	if len(records) != 1 || records[0].EndpointClass != adminEndpointStatic || records[0].Path != "/flow/" {
		t.Fatalf("expected slow static request record, got %+v", records)
	}
}

func TestAdminRequestDiagnosticsReturnsRecentSlowRequests(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/requests", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	service.recordAdminRequest(request, http.StatusInternalServerError, 10*time.Millisecond, time.Now())
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected diagnostics success, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "recentSlowRequests") || !strings.Contains(response.Body.String(), "read_api") {
		t.Fatalf("expected recent slow request diagnostics, got %s", response.Body.String())
	}
}

func TestOpenSQLiteDatabaseUsesWALAndBusyTimeout(t *testing.T) {
	service := NewService(Configuration{TaskDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("expected flow database to open: %v", errorValue)
	}
	defer database.Close()

	var journalMode string
	if errorValue := database.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode); errorValue != nil {
		t.Fatalf("expected journal mode: %v", errorValue)
	}
	var busyTimeout int
	if errorValue := database.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&busyTimeout); errorValue != nil {
		t.Fatalf("expected busy timeout: %v", errorValue)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("expected wal journal mode, got %q", journalMode)
	}
	if busyTimeout != 5000 {
		t.Fatalf("expected busy timeout 5000, got %d", busyTimeout)
	}
}

func TestCredentialProviderStatusMasksOpenRouterKey(t *testing.T) {
	rootPath := t.TempDir()
	keyPath := filepath.Join(rootPath, "secrets", "openrouter-api-key")
	if errorValue := os.MkdirAll(filepath.Dir(keyPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(keyPath, []byte("sk-secret-value"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{
		StateDirectory:    filepath.Join(rootPath, "state"),
		AdminEmailPath:    writeTestFile(t, "admin@example.com"),
		OpenRouterKeyPath: keyPath,
	})
	request := httptest.NewRequest(http.MethodGet, "/admin/api/credentials/providers", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status success, got %d: %s", response.Code, response.Body.String())
	}
	responseBody := response.Body.String()
	if strings.Contains(responseBody, "sk-secret-value") {
		t.Fatalf("expected response to omit secret, got %s", responseBody)
	}
	if !strings.Contains(responseBody, `"configured":true`) || !strings.Contains(responseBody, `"fingerprint":"sha256:`) {
		t.Fatalf("expected masked configured status, got %s", responseBody)
	}
}

func TestCredentialProviderSavesValidatedOpenRouterKey(t *testing.T) {
	rootPath := t.TempDir()
	keyPath := filepath.Join(rootPath, "secrets", "openrouter-api-key")
	service := NewService(Configuration{
		StateDirectory:      filepath.Join(rootPath, "state"),
		AdminEmailPath:      writeTestFile(t, "admin@example.com"),
		OpenRouterKeyPath:   keyPath,
		OpenRouterModelsURL: "https://openrouter.test/models",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://openrouter.test/models" {
			t.Fatalf("unexpected validation url: %s", request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer sk-new" {
			t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
		}
		return jsonResponse(http.StatusOK, `{"data":[]}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodPut, "/admin/api/credentials/openrouter-key", strings.NewReader(`{"apiKey":"sk-new"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected save success, got %d: %s", response.Code, response.Body.String())
	}
	if readTrimmedFile(keyPath) != "sk-new" {
		t.Fatalf("expected key to be stored")
	}
	if strings.Contains(response.Body.String(), "sk-new") {
		t.Fatalf("expected response to omit key, got %s", response.Body.String())
	}
}

func TestCredentialProviderRejectsNonAdmin(t *testing.T) {
	service := NewService(Configuration{
		StateDirectory: t.TempDir(),
		AdminEmailPath: writeTestFile(t, "admin@example.com"),
	})
	request := httptest.NewRequest(http.MethodGet, "/admin/api/credentials/providers", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", response.Code)
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
	if configuration.IdentityDocumentPath != "/root/.internkim/config/identity.json" || configuration.SoulDocumentPath != "/root/.internkim/config/soul.json" {
		t.Fatalf("persona paths = %q %q", configuration.IdentityDocumentPath, configuration.SoulDocumentPath)
	}
	if configuration.MattermostTokenPath != "/root/.internkim/secrets/mattermost-bot-token" {
		t.Fatalf("Mattermost token path = %q", configuration.MattermostTokenPath)
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

	request = httptest.NewRequest(http.MethodGet, "https://dc719d8e.example.test/admin/users", nil)
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
	service := NewService(Configuration{})
	seatAdministratorInTheCompanyForTest(t, service, "admin@example.com")
	handler := service.router()

	request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/requests", nil)
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unauthorized status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/requests", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authorized status = %d", response.Code)
	}
}

func TestWorkspaceSettingsDefaultsToKorean(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})

	request := httptest.NewRequest(http.MethodGet, "/admin/api/workspace-settings", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("workspace settings status = %d body = %s", response.Code, response.Body.String())
	}
	var settings workspaceSettings
	if errorValue := json.NewDecoder(response.Body).Decode(&settings); errorValue != nil {
		t.Fatal(errorValue)
	}
	if settings.Language != workspaceLanguageKorean {
		t.Fatalf("workspace language = %q", settings.Language)
	}
	if settings.TimeZone != "" {
		t.Fatalf("a device that names no company knows no company time zone, got %q", settings.TimeZone)
	}
}

func TestWorkspaceSettingsRejectsInvalidLanguage(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})

	request := httptest.NewRequest(http.MethodPut, "/admin/api/workspace-settings", strings.NewReader(`{"language":"jp"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("workspace settings status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestAdminHealthDoesNotClaimFirstAuthenticatedCaller(t *testing.T) {
	deviceDirectory := t.TempDir()
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminEmailPath := filepath.Join(deviceDirectory, "admin-email")
	writeFile(t, adminEmailPath, "setup@example.com")

	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        adminEmailPath,
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		StateDirectory:        t.TempDir(),
		AdminUIPath:           t.TempDir(),
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	directory := companyDirectoryHolding(memberForTest("setup@example.com", "이샘플", "admin"))
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case isCompanyDirectoryRequest(request):
			return directory.respond(t, request)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/health", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member1@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin health status = %d body = %s", response.Code, response.Body.String())
	}
	if strings.TrimSpace(readTrimmedFile(claimedAdminEmailPath)) != "" {
		t.Fatalf("claimed admin = %q", readTrimmedFile(claimedAdminEmailPath))
	}
	if len(directory.writes) != 0 {
		t.Fatalf("unexpected role writes: %+v", directory.writes)
	}
}

func TestAdminPageRequestClaimsFirstAuthenticatedCaller(t *testing.T) {
	deviceDirectory := t.TempDir()
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminUIPath := t.TempDir()
	writeFile(t, filepath.Join(adminUIPath, "index.html"), "admin ui")

	deliveredPolicyPath := filepath.Join(t.TempDir(), "policy.json")
	service := NewService(Configuration{
		BlueclawPolicyDeliveryPath: deliveredPolicyPath,
		APIBaseURL:                 "https://api.example.test",
		AdminEmailPath:             filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:      claimedAdminEmailPath,
		StateDirectory:             t.TempDir(),
		AdminUIPath:                adminUIPath,
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	directory := companyDirectoryHolding()
	blueclawInvited := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case isCompanyDirectoryRequest(request):
			return directory.respond(t, request)
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, blueclawPolicyWithSeedAdmin(), nil), nil
		case isBlueclawAdminPolicyDelivered(t, request, deliveredPolicyPath, "member1@example.com"):
			blueclawInvited = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member1@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin page status = %d body = %s", response.Code, response.Body.String())
	}
	if strings.TrimSpace(readTrimmedFile(claimedAdminEmailPath)) != "member1@example.com" {
		t.Fatalf("claimed admin = %q", readTrimmedFile(claimedAdminEmailPath))
	}
	if !blueclawInvited {
		t.Fatal("first admin was not invited in Blueclaw policy")
	}
	if len(directory.writes) != 1 || directory.writes[0].Email != "member1@example.com" || directory.writes[0].Role != "admin" {
		t.Fatalf("the claim did not reach the company as an admin role write: %+v", directory.writes)
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
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")

	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		StateDirectory:        t.TempDir(),
		AdminUIPath:           t.TempDir(),
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	directory := companyDirectoryHolding()
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case isCompanyDirectoryRequest(request):
			return directory.respond(t, request)
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusBadGateway, `{"error":"the policy is unreadable"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseDocument := requestAdminSession(t, service, "member1@example.com")
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

func adminUsersProxyTestService(t *testing.T, directory *companyDirectoryForTest) *Service {
	t.Helper()
	service := NewService(Configuration{
		AdminEmailPath:             writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath:      writeTestFile(t, "admin@example.com"),
		BlueclawPolicyDeliveryPath: filepath.Join(t.TempDir(), "policy.json"),
		StateDirectory:             t.TempDir(),
		AdminUIPath:                t.TempDir(),
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case isCompanyDirectoryRequest(request):
			if request.Header.Get("Authorization") != "Bearer agent-key" {
				t.Fatalf("the company door was asked without the agent key: %q", request.Header.Get("Authorization"))
			}
			return directory.respond(t, request)
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		case isBlueclawPolicyReload(request), request.URL.Path == "/admin/api/persona/user":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isRecordRequestOutsideTheDirectory(request):
			return jsonResponse(http.StatusBadGateway, `{"message":"the record is down"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service
}

func TestAdminUsersProxyWritesAPersonThroughTheCompanyDoor(t *testing.T) {
	directory := companyDirectoryHolding(
		memberForTest("admin@example.com", "이샘플", "admin"),
		memberForTest("colleague@example.com", "박예시", "member"),
	)
	service := adminUsersProxyTestService(t, directory)

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"email":"colleague@example.com","name":"박예시","role":"admin","note":"HR follow-up"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("users proxy status = %d body = %s", response.Code, response.Body.String())
	}
	if len(directory.writes) != 1 {
		t.Fatalf("the company received %d writes, want one: %+v", len(directory.writes), directory.writes)
	}
	written := directory.writes[0]
	if written.Email != "colleague@example.com" || written.Name != "박예시" || written.Role != "admin" || written.Note != "HR follow-up" {
		t.Fatalf("the company received %+v", written)
	}
	var answered pagesUsersResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&answered); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(answered.Records) != 2 || answered.Records[1].Email != "colleague@example.com" || answered.Records[1].Role != "admin" {
		t.Fatalf("the console was answered with something other than the company's directory: %+v", answered.Records)
	}
}

func TestAdminUsersProxyWithdrawsAPersonThroughTheCompanyDoor(t *testing.T) {
	directory := companyDirectoryHolding(
		memberForTest("admin@example.com", "이샘플", "admin"),
		memberForTest("colleague@example.com", "박예시", "member"),
	)
	service := adminUsersProxyTestService(t, directory)

	request := httptest.NewRequest(http.MethodDelete, "/admin/api/users/colleague%40example.com", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("users proxy status = %d body = %s", response.Code, response.Body.String())
	}
	if len(directory.withdrawn) != 1 || directory.withdrawn[0] != "colleague@example.com" {
		t.Fatalf("the company withdrew %v", directory.withdrawn)
	}
	if strings.Contains(response.Body.String(), "colleague@example.com") {
		t.Fatalf("somebody withdrawn is still listed as working here: %s", response.Body.String())
	}
}

func TestAdminUsersProxyRefusesToRemoveTheLastAdmin(t *testing.T) {
	directory := companyDirectoryHolding(memberForTest("admin@example.com", "이샘플", "admin"))
	service := adminUsersProxyTestService(t, directory)

	request := httptest.NewRequest(http.MethodDelete, "/admin/api/users/admin%40example.com", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code == http.StatusOK || len(directory.withdrawn) != 0 {
		t.Fatalf("the last admin was withdrawn: status %d, withdrawn %v", response.Code, directory.withdrawn)
	}
}

func TestQuickTaskRejectsUnauthenticatedRemoteCaller(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPost, quickTaskPath, strings.NewReader(`{"prompt":"보고서"}`))
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestTheRetiredTaskPathsAnswerNothing(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/task/api/summary", nil),
		httptest.NewRequest(http.MethodGet, "/task/api/state", nil),
		httptest.NewRequest(http.MethodPost, "/task/api/tasks", strings.NewReader(`{}`)),
		httptest.NewRequest(http.MethodDelete, "/task/api/tasks/task-1", nil),
		httptest.NewRequest(http.MethodPut, "/task/api/definitions", strings.NewReader(`{}`)),
		httptest.NewRequest(http.MethodPost, "/flow/api/tasks/quick", strings.NewReader(`{}`)),
	} {
		request.RemoteAddr = "198.51.100.10:443"
		request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s %s answered %d, and the record is the only place a task lives",
				request.Method, request.URL.Path, response.Code)
		}
	}
}

func TestWebSessionRejectsExpiredTamperedAndStaleCookies(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	expiredCookie := expiredWebSessionCookieForTest(t, service, "member@example.com")
	if errorValue := webSessionFailureForTest(service, expiredCookie); errorValue == nil || errorValue.Error() != "expired" {
		t.Fatalf("expired cookie error = %v", errorValue)
	}
	validCookie := webSessionCookieForTest(t, service, "member@example.com")
	encodedPayload, _, _ := strings.Cut(validCookie, ".")
	tamperedCookie := encodedPayload + ".AAAA"
	if errorValue := webSessionFailureForTest(service, tamperedCookie); errorValue == nil || errorValue.Error() != "bad_signature" {
		t.Fatalf("tampered cookie error = %v", errorValue)
	}
	staleCookie := staleWebSessionCookieForTest(t, service, "member@example.com")
	if errorValue := webSessionFailureForTest(service, staleCookie); errorValue == nil || errorValue.Error() != "policy_changed" {
		t.Fatalf("stale cookie error = %v", errorValue)
	}
}

func TestWebSessionDoesNotAuthorizeAdminAPI(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	cookieValue := webSessionCookieForTest(t, service, "member@example.com")
	request := httptest.NewRequest(http.MethodGet, "/admin/api/users", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.AddCookie(&http.Cookie{Name: webSessionCookieName, Value: cookieValue})
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("admin status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestWebLogoutSuppressesImplicitCloudflareSession(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	logoutRequest := httptest.NewRequest(http.MethodPost, "/auth/logout?return=/tasks/", nil)
	logoutRequest.RemoteAddr = "198.51.100.10:443"
	logoutRequest.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	logoutResponse := httptest.NewRecorder()

	service.router().ServeHTTP(logoutResponse, logoutRequest)

	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d body = %s", logoutResponse.Code, logoutResponse.Body.String())
	}
	var logout webLogoutResponse
	if errorValue := json.NewDecoder(logoutResponse.Body).Decode(&logout); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !logout.OK || logout.RedirectURL != "/tasks/" {
		t.Fatalf("logout = %+v", logout)
	}
	logoutCookies := logoutResponse.Result().Cookies()
	if responseCookieByNameForTest(t, logoutCookies, webSessionCookieName).MaxAge != -1 {
		t.Fatalf("logout cookies = %#v", logoutCookies)
	}
	if responseCookieByNameForTest(t, logoutCookies, webLogoutMarkerCookieName).Value != "1" {
		t.Fatalf("logout cookies = %#v", logoutCookies)
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/auth/session?return=/tasks/", nil)
	sessionRequest.RemoteAddr = "198.51.100.10:443"
	sessionRequest.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	for _, cookie := range logoutCookies {
		sessionRequest.AddCookie(cookie)
	}
	sessionResponse := httptest.NewRecorder()

	service.router().ServeHTTP(sessionResponse, sessionRequest)

	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("session status = %d body = %s", sessionResponse.Code, sessionResponse.Body.String())
	}
	var session webSessionResponse
	if errorValue := json.NewDecoder(sessionResponse.Body).Decode(&session); errorValue != nil {
		t.Fatal(errorValue)
	}
	if session.Authenticated || session.SignupURL != "/auth/verify/start?return=%2Ftasks%2F" {
		t.Fatalf("session = %+v", session)
	}
}

func TestWebLogoutSuppressesAuthenticationWithMarkerCookie(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/auth/session?return=/flow/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	request.AddCookie(webLogoutMarkerCookie(true))
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("session status = %d body = %s", response.Code, response.Body.String())
	}
	var session webSessionResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&session); errorValue != nil {
		t.Fatal(errorValue)
	}
	if session.Authenticated {
		t.Fatalf("session = %+v", session)
	}
}

func TestCloudflareAuthCallbackIssuesWebSession(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/auth/verify/callback?return=/calendar/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("callback status = %d body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != "/calendar/" {
		t.Fatalf("location = %q", response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	sessionCookie := responseCookieByNameForTest(t, cookies, webSessionCookieName)
	if !sessionCookie.HttpOnly {
		t.Fatalf("session cookie = %#v", sessionCookie)
	}
	logoutMarkerCookie := responseCookieByNameForTest(t, cookies, webLogoutMarkerCookieName)
	if logoutMarkerCookie.MaxAge != -1 {
		t.Fatalf("logout marker cookie = %#v", logoutMarkerCookie)
	}
}

func TestCloudflareAuthCallbackRejectsNonMember(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/auth/verify/callback?return=/calendar/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "outsider@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("callback status = %d body = %s", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatalf("cookies = %#v", response.Result().Cookies())
	}
}

func responseCookieByNameForTest(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found in %#v", name, cookies)
	return nil
}

func webSessionCookieForTest(t *testing.T, service *Service, email string) string {
	t.Helper()
	now := time.Now().UTC()
	policyVersion, errorValue := service.currentWebPolicyVersion(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	cookieValue, errorValue := service.signWebSessionPayload(webSessionPayload{
		Email:         strings.ToLower(strings.TrimSpace(email)),
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(webSessionDuration).Unix(),
		PolicyVersion: policyVersion,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return cookieValue
}

func expiredWebSessionCookieForTest(t *testing.T, service *Service, email string) string {
	t.Helper()
	now := time.Now().UTC()
	policyVersion, errorValue := service.currentWebPolicyVersion(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	cookieValue, errorValue := service.signWebSessionPayload(webSessionPayload{
		Email:         strings.ToLower(strings.TrimSpace(email)),
		IssuedAt:      now.Add(-2 * webSessionDuration).Unix(),
		ExpiresAt:     now.Add(-time.Minute).Unix(),
		PolicyVersion: policyVersion,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return cookieValue
}

func staleWebSessionCookieForTest(t *testing.T, service *Service, email string) string {
	t.Helper()
	now := time.Now().UTC()
	cookieValue, errorValue := service.signWebSessionPayload(webSessionPayload{
		Email:         strings.ToLower(strings.TrimSpace(email)),
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(webSessionDuration).Unix(),
		PolicyVersion: "stale",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return cookieValue
}

func webSessionFailureForTest(service *Service, cookieValue string) error {
	_, errorValue := service.verifyWebSessionPayload(context.Background(), cookieValue, time.Now().UTC())
	return errorValue
}

func TestQuickTaskOnTheLocalSocketRequiresARequesterActor(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	unnamed := httptest.NewRequest(http.MethodPost, quickTaskPath, strings.NewReader(`{"prompt":"보고서"}`))
	unnamed.RemoteAddr = "127.0.0.1:12345"
	unnamedResponse := httptest.NewRecorder()

	service.router().ServeHTTP(unnamedResponse, unnamed)

	if unnamedResponse.Code != http.StatusForbidden {
		t.Fatalf("a local call naming nobody = %d body = %s", unnamedResponse.Code, unnamedResponse.Body.String())
	}

	named := httptest.NewRequest(http.MethodPost, quickTaskPath, strings.NewReader(`{"prompt":"보고서"}`))
	named.Header.Set(requesterEmailHeader, "member@example.com")
	namedResponse := httptest.NewRecorder()
	service.router().ServeHTTP(namedResponse, arrivingOnTheRequesterSocket(named))
	if namedResponse.Code == http.StatusForbidden {
		t.Fatalf("a local call naming the requester was refused: %s", namedResponse.Body.String())
	}
}

func newTaskAuthorizationTestService(t *testing.T) *Service {
	t.Helper()
	service := NewService(Configuration{
		StateDirectory:        filepath.Join(t.TempDir(), "state"),
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
		TaskDatabasePath:      filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	seatPeopleInACompanyDirectoryForTest(t, service)
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	directory := companyDirectoryHolding(
		centralplane.Member{MemberID: "user-admin", Email: "admin@example.com", Name: "Admin", Role: "admin", Status: "active"},
		centralplane.Member{MemberID: "user-member", Email: "member@example.com", Name: "Member", Role: "member", Status: "active"},
		centralplane.Member{MemberID: "user-other", Email: "other@example.com", Name: "Other", Role: "member", Status: "active"},
	)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return directory.respond(t, request)
		}
		if isRecordRequestOutsideTheDirectory(request) {
			return jsonResponse(http.StatusBadGateway, `{"message":"the record is down"}`, nil), nil
		}
		if request.URL.Path == "/admin/api/policy" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
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

func TestCORSHeaderIsLimitedToInternKimPaths(t *testing.T) {
	service := NewService(Configuration{})
	handler := service.withCORS(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Access-Control-Allow-Origin", "https://messenger.example")
		responseWriter.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v4/system/ping", nil)
	request.Header.Set("Origin", "https://device.example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if origins := response.Result().Header.Values("Access-Control-Allow-Origin"); !reflect.DeepEqual(origins, []string{"https://messenger.example"}) {
		t.Fatalf("unexpected proxy CORS origins: %#v", origins)
	}
}

func TestCORSHeaderIsAddedForInternKimPaths(t *testing.T) {
	service := NewService(Configuration{APIBaseURL: "https://api.example.test"})
	handler := service.withCORS(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil)
	request.Header.Set("Origin", "https://device.example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Access-Control-Allow-Origin") != "https://device.example.test" {
		t.Fatalf("unexpected internkim CORS origin: %s", response.Header().Get("Access-Control-Allow-Origin"))
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

func isBlueclawPolicyReload(request *http.Request) bool {
	return request.Method == http.MethodPost && request.URL.String() == "http://127.0.0.1:8080/admin/api/policy/reload"
}

func isBlueclawPolicyGet(request *http.Request) bool {
	return request.Method == http.MethodGet && request.URL.String() == "http://127.0.0.1:8080/admin/api/policy"
}

func isBlueclawAdminPolicyDelivered(t *testing.T, request *http.Request, policyPath string, expectedEmail string) bool {
	t.Helper()
	if !isBlueclawPolicyReload(request) {
		return false
	}
	delivered, errorValue := os.ReadFile(policyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var policyDocument map[string]any
	if errorValue := json.Unmarshal(delivered, &policyDocument); errorValue != nil {
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
	return `{"people":[{"personID":"00000000-0000-0000-0000-000000000001","displayName":"Intern Kim Admin","emails":["admin@example.com"],"isAdmin":true}],"channels":[],"retention":{"rawEventDays":60}}`
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
