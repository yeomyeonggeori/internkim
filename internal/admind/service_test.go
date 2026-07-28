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
	"reflect"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
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

func TestMattermostPostMaintenanceRepairsMessageRootAndTimestamps(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	queries := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		if name != "su" || len(arguments) != 4 || arguments[0] != "-" || arguments[1] != "postgres" || arguments[2] != "-c" {
			t.Fatalf("command = %s %v", name, arguments)
		}
		query := arguments[3]
		queries = append(queries, query)
		switch {
		case strings.Contains(query, "row_to_json") && strings.Contains(query, "post-1"):
			return []byte(`{"id":"post-1","channelid":"channel-1","rootid":"","message":"old","createat":1,"updateat":1,"editat":0,"deleteat":0}`), nil
		case strings.Contains(query, "row_to_json") && strings.Contains(query, "root-1"):
			return []byte(`{"id":"root-1","channelid":"channel-1","rootid":"","message":"root","createat":1,"updateat":1,"editat":0,"deleteat":0}`), nil
		case strings.Contains(query, "UPDATE posts SET"):
			return nil, nil
		default:
			t.Fatalf("query = %s", query)
			return nil, nil
		}
	}
	body := `{"message":"new","rootID":"root-1","createAt":"2026-06-09T23:00:58+09:00","updateAt":1777734058000}`
	request := httptest.NewRequest(http.MethodPost, "/admin/api/maintenance/mattermost-posts/post-1/repair", strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()

	service.handleAdmin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(queries) != 3 {
		t.Fatalf("queries = %+v", queries)
	}
	updateQuery := queries[2]
	for _, expectedText := range []string{"message", "new", "rootid", "root-1", "createat = 1781013658000", "updateat = 1777734058000"} {
		if !strings.Contains(updateQuery, expectedText) {
			t.Fatalf("update query %q missing %q", updateQuery, expectedText)
		}
	}
	var repairResponse mattermostPostRepairResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &repairResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if repairResponse.After.Message != "new" || repairResponse.After.RootID != "root-1" || len(repairResponse.Updates) != 4 {
		t.Fatalf("response = %+v", repairResponse)
	}
}

func TestMattermostPostMaintenanceRequiresClaimedAdminForRemoteRequests(t *testing.T) {
	service := NewService(Configuration{
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath: writeTestFile(t, "lee@example.com"),
	})
	request := httptest.NewRequest(http.MethodPost, "/admin/api/maintenance/mattermost-posts/post-1/repair", strings.NewReader(`{"message":"new"}`))
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set("X-Forwarded-Email", "admin@example.com")
	response := httptest.NewRecorder()

	service.handleAdmin(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestAdminRequestMetricsClassifiesAndRecordsSlowRequests(t *testing.T) {
	service := NewService(Configuration{})
	staticRequest := httptest.NewRequest(http.MethodGet, "/flow/", nil)
	apiRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/events", nil)
	writeRequest := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", nil)
	controlRequest := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", nil)
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
	if adminEndpointClassForRequest(controlRequest) != adminEndpointControl {
		t.Fatalf("expected control class")
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
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	database, errorValue := service.openFlowDatabase(context.Background())
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
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", response.Code)
	}
}

func TestGatewayRejectsFlowChannelPostCreation(t *testing.T) {
	testFlowOrCalendarChannelPostRejection(t, "flow-channel", (*Service).saveMattermostFlowChannelID)
}

func TestGatewayRejectsCalendarChannelPostCreation(t *testing.T) {
	testFlowOrCalendarChannelPostRejection(t, "calendar-channel", (*Service).saveMattermostCalendarChannelID)
}

func testFlowOrCalendarChannelPostRejection(t *testing.T, channelID string, saveChannelID func(*Service, string)) {
	t.Helper()
	service := NewService(Configuration{StateDirectory: t.TempDir()})
	saveChannelID(service, channelID)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatalf("managed post reached Mattermost: %s %s", request.Method, request.URL)
		return nil, nil
	})}
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"`+channelID+`","message":"blocked"}`))
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("managed channel post status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestGatewayDeletesAttendanceChannelPostCreation(t *testing.T) {
	deletedPostIDs := testManagedChannelPostCreation(t, "attendance-channel", (*Service).saveMattermostAttendanceChannelID)
	if len(deletedPostIDs) != 1 || deletedPostIDs[0] != "managed-post-1" {
		t.Fatalf("deleted post IDs = %+v", deletedPostIDs)
	}
}

func testManagedChannelPostCreation(t *testing.T, channelID string, saveChannelID func(*Service, string)) []string {
	t.Helper()
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		StateDirectory:              stateDirectory,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
	})
	saveChannelID(service, channelID)
	deletedPostIDs := []string{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/api/v4/users/login" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.Path == "/api/v4/posts" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"managed-post-1","channel_id":"`+channelID+`"}`, nil), nil
		case strings.HasPrefix(request.URL.Path, "/api/v4/posts/") && request.Method == http.MethodDelete:
			if request.Header.Get("Authorization") != "Bearer admin-token" {
				t.Fatalf("delete token = %q", request.Header.Get("Authorization"))
			}
			deletedPostIDs = append(deletedPostIDs, strings.TrimPrefix(request.URL.Path, "/api/v4/posts/"))
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"`+channelID+`","message":"hello"}`))
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("managed channel post status = %d body = %s", response.Code, response.Body.String())
	}
	return deletedPostIDs
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
	if settings.TimeZone != workspaceBusinessTimeZone {
		t.Fatalf("workspace time zone = %q", settings.TimeZone)
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

func TestWorkspaceSettingsUpdateSyncsKoreanMattermostDisplayNames(t *testing.T) {
	service, patchedDisplayNames, createdChannels := newWorkspaceSettingsMattermostTestService(t, false)

	request := httptest.NewRequest(http.MethodPut, "/admin/api/workspace-settings", strings.NewReader(`{"language":"ko"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("workspace settings status = %d body = %s", response.Code, response.Body.String())
	}
	if patchedDisplayNames[mattermostFlowChannelName] != "업무" {
		t.Fatalf("flow display name = %q", patchedDisplayNames[mattermostFlowChannelName])
	}
	if patchedDisplayNames[mattermostdefaults.TownSquareChannelName] != "광장" {
		t.Fatalf("town square display name = %q", patchedDisplayNames[mattermostdefaults.TownSquareChannelName])
	}
	if patchedDisplayNames[mattermostdefaults.OffTopicChannelName] != "잡담" {
		t.Fatalf("off-topic display name = %q", patchedDisplayNames[mattermostdefaults.OffTopicChannelName])
	}
	if patchedDisplayNames[mattermostCalendarChannelName] != "일정" {
		t.Fatalf("calendar display name = %q", patchedDisplayNames[mattermostCalendarChannelName])
	}
	if patchedDisplayNames[attendanceChannelName] != "근태" {
		t.Fatalf("attendance display name = %q", patchedDisplayNames[attendanceChannelName])
	}
	if patchedDisplayNames[calendarAnnouncementsChannelName] != "공지사항" {
		t.Fatalf("announcements display name = %q", patchedDisplayNames[calendarAnnouncementsChannelName])
	}
	if len(createdChannels) != 0 {
		t.Fatalf("unexpected created channels = %#v", createdChannels)
	}
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if settings.Language != workspaceLanguageKorean {
		t.Fatalf("saved workspace language = %q", settings.Language)
	}
}

func TestWorkspaceSettingsUpdateSyncsEnglishMattermostDisplayNames(t *testing.T) {
	service, patchedDisplayNames, createdChannels := newWorkspaceSettingsMattermostTestService(t, true)

	request := httptest.NewRequest(http.MethodPut, "/admin/api/workspace-settings", strings.NewReader(`{"language":"en"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("workspace settings status = %d body = %s", response.Code, response.Body.String())
	}
	if patchedDisplayNames[mattermostFlowChannelName] != "Flow" {
		t.Fatalf("flow display name = %q", patchedDisplayNames[mattermostFlowChannelName])
	}
	if patchedDisplayNames[mattermostdefaults.TownSquareChannelName] != "Town Square" {
		t.Fatalf("town square display name = %q", patchedDisplayNames[mattermostdefaults.TownSquareChannelName])
	}
	if patchedDisplayNames[mattermostdefaults.OffTopicChannelName] != "Off-Topic" {
		t.Fatalf("off-topic display name = %q", patchedDisplayNames[mattermostdefaults.OffTopicChannelName])
	}
	if patchedDisplayNames[mattermostCalendarChannelName] != "Calendar" {
		t.Fatalf("calendar display name = %q", patchedDisplayNames[mattermostCalendarChannelName])
	}
	if patchedDisplayNames[attendanceChannelName] != "Attendance" {
		t.Fatalf("attendance display name = %q", patchedDisplayNames[attendanceChannelName])
	}
	if createdChannels[calendarAnnouncementsChannelName] != "Announcements" {
		t.Fatalf("announcements created display name = %q", createdChannels[calendarAnnouncementsChannelName])
	}
	if _, found := patchedDisplayNames[calendarAnnouncementsChannelName]; found {
		t.Fatalf("missing announcements channel should not be patched: %#v", patchedDisplayNames)
	}
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if settings.Language != workspaceLanguageEnglish {
		t.Fatalf("saved workspace language = %q", settings.Language)
	}
}

func TestAdminHealthDoesNotClaimFirstAuthenticatedCaller(t *testing.T) {
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminEmailPath := filepath.Join(deviceDirectory, "admin-email")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminEmailPath, "setup@example.com")

	var roleWrites []adminUserMutation
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        adminEmailPath,
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		StateDirectory:        t.TempDir(),
		CompanionJobPath:      filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:           t.TempDir(),
		MattermostBaseURL:     "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
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
	if response.Code != http.StatusOK {
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")
	adminUIPath := t.TempDir()
	writeFile(t, filepath.Join(adminUIPath, "index.html"), "admin ui")

	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:       claimedAdminEmailPath,
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 adminUIPath,
	})
	blueclawInvited := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")

	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		MattermostBaseURL:     "http://mattermost.local",
		AdminEmailPath:        filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath: claimedAdminEmailPath,
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		StateDirectory:        t.TempDir(),
		CompanionJobPath:      filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:           t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	createdMattermostPassword := ""
	hasUserRecord := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:       claimedAdminEmailPath,
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			if hasUserRecord {
				return jsonResponse(http.StatusOK, `{"records":[{"email":"lee@example.com","role":"admin"}]}`, nil), nil
			}
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			var payload adminUserMutation
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.Email != "lee@example.com" || payload.Role != "admin" {
				t.Fatalf("unexpected user role payload: %+v", payload)
			}
			hasUserRecord = true
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	passwordReset := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              filepath.Join(deviceDirectory, "admin-email"),
		ClaimedAdminEmailPath:       claimedAdminEmailPath,
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	claimedAdminEmailPath := filepath.Join(deviceDirectory, "claimed-admin-email")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	stateDirectory := t.TempDir()
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
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
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
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
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
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
	fleetIDPath := writeTestFile(t, "dc719d8e")
	fleetSecretPath := writeTestFile(t, "secret-value")
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		StateDirectory:        t.TempDir(),
		CompanionJobPath:      filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:           t.TempDir(),
		MattermostBaseURL:     "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://api.example.test/api/users?fleet_id=dc719d8e" {
			t.Fatalf("proxy url = %s", request.URL.String())
		}
		if request.Header.Get("X-InternKim-Fleet-ID") != "dc719d8e" {
			t.Fatalf("fleet id header = %q", request.Header.Get("X-InternKim-Fleet-ID"))
		}
		if request.Header.Get("X-InternKim-Fleet-Secret") != "secret-value" {
			t.Fatalf("fleet secret header = %q", request.Header.Get("X-InternKim-Fleet-Secret"))
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
	fleetIDPath := writeTestFile(t, "dc719d8e")
	fleetSecretPath := writeTestFile(t, "secret-value")
	adminPasswordPath := writeTestFile(t, "admin-pass")
	directChannelCreated := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath:       writeTestFile(t, "admin@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	var pagesPayload map[string]any
	blueclawInvited := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
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
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			if request.Header.Get("X-InternKim-Fleet-Secret") != "secret-value" {
				t.Fatalf("fleet secret header = %q", request.Header.Get("X-InternKim-Fleet-Secret"))
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

func TestAdminUserPasswordResetDeletesDMHistoryBeforeReturningPassword(t *testing.T) {
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	deletedPostIDs := []string{}
	passwordReset := false
	blueclawReset := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		BlueclawBaseURL:             "http://blueclaw.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath:       writeTestFile(t, "admin@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://blueclaw.local/admin/api/conversation/reset" && request.Method == http.MethodPost:
			requestBody, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(requestBody), `"channelID":"dm-1"`) {
				t.Fatalf("blueclaw reset body = %s", requestBody)
			}
			if passwordReset {
				t.Fatal("Blueclaw DM history was reset after password reset")
			}
			blueclawReset = true
			return jsonResponse(http.StatusOK, `{"deletedRows":2}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[{"email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/dm-1/posts?page=0&per_page=200":
			return jsonResponse(http.StatusOK, `{"order":["post-1","post-2"],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/dm-1/posts?page=1&per_page=200":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case strings.HasPrefix(request.URL.String(), "http://mattermost.local/api/v4/posts/") && request.Method == http.MethodDelete:
			if passwordReset {
				t.Fatal("DM post was deleted after password reset")
			}
			deletedPostIDs = append(deletedPostIDs, strings.TrimPrefix(request.URL.Path, "/api/v4/posts/"))
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/password" && request.Method == http.MethodPut:
			if !blueclawReset {
				t.Fatal("password reset happened before Blueclaw DM history reset")
			}
			passwordReset = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users/member%40example.com/password-reset", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("password reset status = %d body = %s", response.Code, response.Body.String())
	}
	var document map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document["temporaryPassword"] == "" || document["temporaryPasswordEmail"] != "member@example.com" {
		t.Fatalf("password reset response = %#v", document)
	}
	if !reflect.DeepEqual(deletedPostIDs, []string{"post-1", "post-2"}) {
		t.Fatalf("deleted posts = %#v", deletedPostIDs)
	}
	if !passwordReset {
		t.Fatal("Mattermost password was not reset")
	}
}

func TestAdminInvitePreservesCurrentAdminRole(t *testing.T) {
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	var pagesPayload map[string]any
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath:       writeTestFile(t, "admin@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	deactivatedUserID := ""
	systemPostsDeleted := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
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
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"admin@example.com","role":"admin"},{"email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
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
		case request.URL.String() == "https://api.example.test/api/users/member@example.com?fleet_id=dc719d8e" && request.Method == http.MethodDelete:
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
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	adminPasswordPath := filepath.Join(deviceDirectory, "mm-admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, adminPasswordPath, "admin-pass")

	pagesDeleteCalled := false
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		AdminEmailPath:              writeTestFile(t, "owner@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"email":"owner@example.com","role":"admin"},{"email":"admin@example.com","role":"admin","mattermostUserID":"admin-id","mattermostUsername":"admin"}]}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
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
		case request.URL.String() == "https://api.example.test/api/users/admin@example.com?fleet_id=dc719d8e" && request.Method == http.MethodDelete:
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
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/bot-1/image":
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

func TestBotProfileUpdateUploadsMattermostProfileImage(t *testing.T) {
	workspacePath := t.TempDir()
	profilePath := filepath.Join(t.TempDir(), "bot-profile.yaml")
	profileImagePath := writeTestFile(t, "profile-image")
	uploadedImage := false
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-password"),
		BotProfilePath:              profilePath,
		BotProfileImagePath:         profileImagePath,
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
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/bot-1/image":
			assertMattermostBearerToken(t, request, "admin-token")
			if !strings.Contains(request.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("profile image content type = %q", request.Header.Get("Content-Type"))
			}
			file, fileHeader, errorValue := request.FormFile("image")
			if errorValue != nil {
				t.Fatalf("expected multipart image: %v", errorValue)
			}
			defer file.Close()
			document, errorValue := io.ReadAll(file)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if fileHeader.Filename != filepath.Base(profileImagePath) || string(document) != "profile-image" {
				t.Fatalf("unexpected profile image upload: %s %q", fileHeader.Filename, string(document))
			}
			uploadedImage = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodPut, "/admin/api/bot-profile", strings.NewReader(`{
		"displayName":"김인턴",
		"englishDisplayName":"Intern Kim",
		"aliases":["인턴킴"],
		"publicDescription":"",
		"identityExtension":"Always use the display name."
	}`))
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("bot profile status = %d: %s", response.Code, response.Body.String())
	}
	if !uploadedImage {
		t.Fatal("expected profile image upload")
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
		httptest.NewRequest(http.MethodGet, "/flow/api/state", nil),
		httptest.NewRequest(http.MethodPost, "/flow/api/tasks", strings.NewReader(`{}`)),
		httptest.NewRequest(http.MethodDelete, "/flow/api/tasks/task-1", nil),
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

	stateRequest := httptest.NewRequest(http.MethodGet, "/flow/api/state", nil)
	stateRequest.RemoteAddr = "198.51.100.10:443"
	stateRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	stateResponse := httptest.NewRecorder()
	handler.ServeHTTP(stateResponse, stateRequest)
	if stateResponse.Code != http.StatusOK {
		t.Fatalf("state status = %d body = %s", stateResponse.Code, stateResponse.Body.String())
	}
	var state flowStateResponse
	if errorValue := json.NewDecoder(stateResponse.Body).Decode(&state); errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.CurrentUserEmail != "staff@example.com" || state.IsAdmin {
		t.Fatalf("state current user email=%q isAdmin=%v", state.CurrentUserEmail, state.IsAdmin)
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

func TestFlowAPIDeletesOwnTask(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
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
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/flow/api/tasks/"+task.ID, nil)
	deleteRequest.RemoteAddr = "198.51.100.10:443"
	deleteRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status = %d body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	_, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatal("expected task to be deleted")
	}
}

func TestFlowAPIRejectsDeletingOtherUserTask(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	taskRequest := newFlowTaskRequest("other@example.com", "other@example.com")
	taskResponse := httptest.NewRecorder()
	handler.ServeHTTP(taskResponse, taskRequest)
	if taskResponse.Code != http.StatusOK {
		t.Fatalf("task status = %d body = %s", taskResponse.Code, taskResponse.Body.String())
	}
	var task flowTask
	if errorValue := json.NewDecoder(taskResponse.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/flow/api/tasks/"+task.ID, nil)
	deleteRequest.RemoteAddr = "198.51.100.10:443"
	deleteRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusForbidden {
		t.Fatalf("delete status = %d body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	_, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected task to remain")
	}
}


func TestFlowAPIAllowsSignedWebSessionStaffSummary(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	cookieValue := webSessionCookieForTest(t, service, "staff@example.com")
	request := httptest.NewRequest(http.MethodGet, "/flow/api/state", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.AddCookie(&http.Cookie{Name: webSessionCookieName, Value: cookieValue})
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("state status = %d body = %s", response.Code, response.Body.String())
	}
	var state flowStateResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&state); errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.CurrentUserEmail != "staff@example.com" || state.IsAdmin {
		t.Fatalf("state current user email=%q isAdmin=%v", state.CurrentUserEmail, state.IsAdmin)
	}
}

func TestFlowSummaryIncludesDistanceReport(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	staffID := stableFlowID("staff@example.com")
	otherID := stableFlowID("other@example.com")
	tasks := []flowTask{
		flowReportTestTask("current", "26W18", []string{staffID, otherID}, []string{"Staff", "Other"}, "M", "완료", "2026-04-27", "2026-04-29"),
		flowReportTestTask("previous-week", "26W17", []string{otherID}, []string{"Other"}, "S", "완료", "2026-04-20", "2026-04-21"),
		flowReportTestTask("previous-month", "26W10", []string{staffID}, []string{"Staff"}, "XS", "완료", "2026-03-03", "2026-03-03"),
		flowReportTestTask("future-week", "26W19", []string{staffID}, []string{"Staff"}, "XS", "예정", "2026-05-04", ""),
	}
	for _, task := range tasks {
		if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary?week=26W18", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("summary status = %d body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	var summary flowSummaryResponse
	if errorValue := json.NewDecoder(bytes.NewReader(body)).Decode(&summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	if summary.Report.WeeklyDistanceTrend.CurrentTotal != 3 || summary.Report.WeeklyDistanceTrend.PreviousTotal != 2 {
		t.Fatalf("weekly trend = %+v", summary.Report.WeeklyDistanceTrend)
	}
	if len(summary.Report.WeeklyDistanceTrend.CurrentValues) != 7 || summary.Report.WeeklyDistanceTrend.CurrentValues[2] != 3 {
		t.Fatalf("weekly values = %+v", summary.Report.WeeklyDistanceTrend.CurrentValues)
	}
	if summary.Report.MonthlyDistanceTrend.CurrentTotal != 5 || summary.Report.MonthlyDistanceTrend.PreviousTotal != 1 {
		t.Fatalf("monthly trend = %+v", summary.Report.MonthlyDistanceTrend)
	}
	if len(summary.Report.MonthlyDistanceTrend.CurrentValues) != 30 || summary.Report.MonthlyDistanceTrend.CurrentValues[28] != 5 {
		t.Fatalf("monthly values = %+v", summary.Report.MonthlyDistanceTrend.CurrentValues)
	}
	if len(summary.WeeklyTasks) != 1 || summary.WeeklyTasks[0].ID != "current" {
		t.Fatalf("weekly tasks = %+v", summary.WeeklyTasks)
	}
	if summary.CurrentWeek.Code == "" {
		t.Fatalf("current week = %+v selected week = %+v", summary.CurrentWeek, summary.Week)
	}
	if len(summary.Metrics.MemberScores) != 0 || len(summary.Metrics.MemberScoreDetails) != 0 || summary.Metrics.TotalScore != 0 {
		t.Fatalf("summary score metrics = %+v", summary.Metrics)
	}
	assertJSONFieldsAbsent(t, body, "members", "tasks", "definitions", "statusOptions", "currentUserEmail", "currentUserName", "isAdmin")
}

func TestFlowStateIncludesCurrentGlobalData(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	staffID := stableFlowID("staff@example.com")
	otherID := stableFlowID("other@example.com")
	now := time.Now()
	currentWeekCode := weekCodeForDate(now)
	currentWeekStart := weekStartForCode(currentWeekCode, now)
	tasks := []flowTask{
		flowReportTestTask("current-score", currentWeekCode, []string{staffID}, []string{"Staff"}, "M", "완료", currentWeekStart.Format("2006-01-02"), currentWeekStart.Format("2006-01-02")),
		flowReportTestTask("older", weekCodeForDate(currentWeekStart.AddDate(0, 0, -7*2)), []string{otherID}, []string{"Other"}, "XS", "진행", currentWeekStart.AddDate(0, 0, -14).Format("2006-01-02"), ""),
	}
	for _, task := range tasks {
		if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/flow/api/state", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("state status = %d body = %s", response.Code, response.Body.String())
	}
	body := response.Body.Bytes()
	var state flowStateResponse
	if errorValue := json.NewDecoder(bytes.NewReader(body)).Decode(&state); errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.CurrentWeek.Code != currentWeekCode {
		t.Fatalf("current week = %+v, want %s", state.CurrentWeek, currentWeekCode)
	}
	if len(state.Tasks) != 2 {
		t.Fatalf("state tasks = %+v", state.Tasks)
	}
	if len(state.Members) != 3 {
		t.Fatalf("state members = %+v", state.Members)
	}
	if state.Metrics.MemberScores[staffID] == 0 || state.Metrics.MemberScoreDetails[staffID].CurrentScore == 0 {
		t.Fatalf("state member scores = %+v details = %+v", state.Metrics.MemberScores, state.Metrics.MemberScoreDetails)
	}
	assertJSONFieldsAbsent(t, body, "week", "weeklyTasks", "report")
}

func assertJSONFieldsAbsent(t *testing.T, document []byte, fields ...string) {
	t.Helper()
	values := map[string]json.RawMessage{}
	if errorValue := json.Unmarshal(document, &values); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, field := range fields {
		if _, found := values[field]; found {
			t.Fatalf("unexpected JSON field %q in %s", field, string(document))
		}
	}
}

func TestFlowSummaryMemberScoresUseCurrentWeek(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	staffID := stableFlowID("staff@example.com")
	now := time.Now()
	currentWeekCode := weekCodeForDate(now)
	currentWeekStart := weekStartForCode(currentWeekCode, now)
	previousCurrentWeekStart := currentWeekStart.AddDate(0, 0, -7)
	previousCurrentWeekCode := weekCodeForDate(previousCurrentWeekStart)
	selectedWeekStart := currentWeekStart.AddDate(0, 0, -7*30)
	selectedWeekCode := weekCodeForDate(selectedWeekStart)
	tasks := []flowTask{
		flowReportTestTask("selected-score", selectedWeekCode, []string{staffID}, []string{"Staff"}, "M", "완료", selectedWeekStart.Format("2006-01-02"), selectedWeekStart.Format("2006-01-02")),
		flowReportTestTask("current-score", currentWeekCode, []string{staffID}, []string{"Staff"}, "M", "완료", currentWeekStart.Format("2006-01-02"), currentWeekStart.Format("2006-01-02")),
		flowReportTestTask("previous-current-score", previousCurrentWeekCode, []string{staffID}, []string{"Staff"}, "M", "완료", previousCurrentWeekStart.Format("2006-01-02"), previousCurrentWeekStart.Format("2006-01-02")),
	}
	for _, task := range tasks {
		if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/flow/api/state", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("state status = %d body = %s", response.Code, response.Body.String())
	}
	var state flowStateResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&state); errorValue != nil {
		t.Fatal(errorValue)
	}
	members := service.flowMembers(request)
	currentScoreStart, currentScoreEnd := flowScoreDateRange(currentWeekStart)
	currentScoreTasks, errorValue := service.readFlowTasksBetweenDates(context.Background(), currentScoreStart.Format("2006-01-02"), currentScoreEnd.Format("2006-01-02"), members)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedCurrentDetails := buildFlowMemberScoreDetails(currentScoreTasks, members, state.Definitions, currentWeekStart)
	selectedScoreStart, selectedScoreEnd := flowScoreDateRange(selectedWeekStart)
	selectedScoreTasks, errorValue := service.readFlowTasksBetweenDates(context.Background(), selectedScoreStart.Format("2006-01-02"), selectedScoreEnd.Format("2006-01-02"), members)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedWeekDetails := buildFlowMemberScoreDetails(selectedScoreTasks, members, state.Definitions, selectedWeekStart)
	if expectedCurrentDetails[staffID].CurrentScore == selectedWeekDetails[staffID].CurrentScore {
		t.Fatalf("test setup did not distinguish current and selected score details: current=%+v selected=%+v", expectedCurrentDetails[staffID], selectedWeekDetails[staffID])
	}
	if state.Metrics.MemberScoreDetails[staffID] != expectedCurrentDetails[staffID] {
		t.Fatalf("staff score detail = %+v, want current-week detail %+v", state.Metrics.MemberScoreDetails[staffID], expectedCurrentDetails[staffID])
	}
	if state.Metrics.MemberScores[staffID] != expectedCurrentDetails[staffID].CurrentScore {
		t.Fatalf("staff score = %d, want %d", state.Metrics.MemberScores[staffID], expectedCurrentDetails[staffID].CurrentScore)
	}
}

func TestWebSessionRejectsExpiredTamperedAndStaleCookies(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	expiredCookie := expiredWebSessionCookieForTest(t, service, "staff@example.com")
	if errorValue := webSessionFailureForTest(service, expiredCookie); errorValue == nil || errorValue.Error() != "expired" {
		t.Fatalf("expired cookie error = %v", errorValue)
	}
	validCookie := webSessionCookieForTest(t, service, "staff@example.com")
	encodedPayload, _, _ := strings.Cut(validCookie, ".")
	tamperedCookie := encodedPayload + ".AAAA"
	if errorValue := webSessionFailureForTest(service, tamperedCookie); errorValue == nil || errorValue.Error() != "bad_signature" {
		t.Fatalf("tampered cookie error = %v", errorValue)
	}
	staleCookie := staleWebSessionCookieForTest(t, service, "staff@example.com")
	if errorValue := webSessionFailureForTest(service, staleCookie); errorValue == nil || errorValue.Error() != "policy_changed" {
		t.Fatalf("stale cookie error = %v", errorValue)
	}
}

func TestWebSessionDoesNotAuthorizeAdminAPI(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	cookieValue := webSessionCookieForTest(t, service, "staff@example.com")
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
	service := newFlowAuthorizationTestService(t)
	logoutRequest := httptest.NewRequest(http.MethodPost, "/auth/logout?return=/tasks/", nil)
	logoutRequest.RemoteAddr = "198.51.100.10:443"
	logoutRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
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
	sessionRequest.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
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

func TestWebLogoutSuppressesMattermostSessionCookie(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/auth/session?return=/flow/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.AddCookie(&http.Cookie{Name: "MMAUTHTOKEN", Value: "session-token"})
	request.AddCookie(webLogoutMarkerCookie())
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

func TestTasksPageRefreshServesApplicationShell(t *testing.T) {
	adminUIPath := t.TempDir()
	writeFile(t, filepath.Join(adminUIPath, "index.html"), "application shell")
	service := NewService(Configuration{
		AdminUIPath:       adminUIPath,
		MattermostBaseURL: "http://mattermost.local",
	})
	request := httptest.NewRequest(http.MethodGet, "/tasks/run-1", nil)
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != "application shell" {
		t.Fatalf("tasks page status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestTasksPageRedirectsBarePath(t *testing.T) {
	service := NewService(Configuration{AdminUIPath: t.TempDir()})
	request := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusFound || response.Header().Get("Location") != "/tasks/" {
		t.Fatalf("tasks redirect status = %d location = %q", response.Code, response.Header().Get("Location"))
	}
}






func TestCloudflareAuthCallbackIssuesWebSession(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/auth/verify/callback?return=/calendar/", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
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

func TestCloudflareAuthCallbackRejectsNonStaff(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
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

func TestFlowAPIForcesStaffTaskForOtherMemberToRequestWithoutRequesterParticipant(t *testing.T) {
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
	if containsString(task.ParticipantIDs, stableFlowID("staff@example.com")) {
		t.Fatalf("requester should not be a participant by default, got %+v", task.ParticipantIDs)
	}
}

func TestAdminLocalePersistsMattermostSystemTextLanguage(t *testing.T) {
	service := NewService(Configuration{
		StateDirectory: filepath.Join(t.TempDir(), "state"),
		AdminEmailPath: writeTestFile(t, "admin@example.com"),
	})
	request := httptest.NewRequest(http.MethodPut, "/admin/api/locale", strings.NewReader(`{"locale":"en"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("locale status = %d body = %s", response.Code, response.Body.String())
	}
	if service.mattermostFlowLink("") != "[업무 열기](/flow/)" {
		t.Fatalf("flow link = %q", service.mattermostFlowLink(""))
	}
	if service.mattermostCalendarLink("") != "[일정 열기](/calendar/)" {
		t.Fatalf("calendar link = %q", service.mattermostCalendarLink(""))
	}
	if service.mattermostAttendanceLink() != "[근태 열기](/attendance/)" {
		t.Fatalf("attendance link = %q", service.mattermostAttendanceLink())
	}
}

func TestFlowAPIRespectsExplicitRequesterParticipantForOtherMemberTask(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	payload := flowTaskWriteRequest{
		OwnerID:        stableFlowID("other@example.com"),
		ParticipantIDs: []string{stableFlowID("other@example.com"), stableFlowID("staff@example.com")},
		Type:           "회의",
		Content:        "같이 10분 회의",
		Size:           "XS",
		Status:         "진행",
		WeekCode:       "26W18",
	}
	document, _ := json.Marshal(payload)
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", bytes.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("task status = %d body = %s", response.Code, response.Body.String())
	}
	var task flowTask
	if errorValue := json.NewDecoder(response.Body).Decode(&task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !containsString(task.ParticipantIDs, stableFlowID("staff@example.com")) {
		t.Fatalf("expected requester participant for joint work, got %+v", task.ParticipantIDs)
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

	request = httptest.NewRequest(http.MethodGet, "/flow/api/state", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set(flowRequesterEmailHeader, "staff@example.com")
	response = httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("local requester state status = %d body = %s", response.Code, response.Body.String())
	}
	var state flowStateResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&state); errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.CurrentUserEmail != "staff@example.com" || state.IsAdmin {
		t.Fatalf("local requester state current user email=%q isAdmin=%v", state.CurrentUserEmail, state.IsAdmin)
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
	if requests.createdMessages[0] == "" || !strings.Contains(requests.createdMessages[0], "| 요청 | [10분 회의](") || !strings.Contains(requests.createdMessages[0], "task=task-1") || strings.Contains(requests.createdMessages[0], "업무 열기") {
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
	if len(requests.updatedMessages) != 1 || !strings.Contains(requests.updatedMessages[0], "| 완료 | [회의 완료](") || !strings.Contains(requests.updatedMessages[0], "task=task-1") || strings.Contains(requests.updatedMessages[0], "업무 열기") {
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

func TestFlowMattermostProjectionOutboxRetriesFailedCreate(t *testing.T) {
	createAttempts := 0
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		MattermostBotTokenPath:      writeTestFile(t, "bot-token"),
		FlowDatabasePath:            filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users?per_page=200" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `[]`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			createAttempts++
			if createAttempts == 1 {
				return jsonResponse(http.StatusServiceUnavailable, `{}`, nil), nil
			}
			return jsonResponse(http.StatusCreated, `{"id":"flow-post-1"}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostExistingFlowSetupResponse(t, request), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	task := flowNotificationTestTask("요청")
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	task = service.applyFlowMattermostProjection(context.Background(), task)
	if task.MattermostPostID != "" {
		t.Fatalf("post id after failed projection = %q", task.MattermostPostID)
	}
	assertFlowProjectionOutboxCount(t, service, 1)

	service.drainFlowMattermostProjectionOutbox(context.Background())

	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded task: found=%v error=%v", found, errorValue)
	}
	if reloadedTask.MattermostPostID != "flow-post-1" {
		t.Fatalf("post id after reconcile = %q", reloadedTask.MattermostPostID)
	}
	if createAttempts != 2 {
		t.Fatalf("create attempts = %d", createAttempts)
	}
	assertFlowProjectionOutboxCount(t, service, 0)
}

func TestFlowMattermostNotificationReplacesAdminPostWithBotPost(t *testing.T) {
	deletedPostIDs := []string{}
	createdPostToken := ""
	service := NewService(Configuration{
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		MattermostBotTokenPath:      writeTestFile(t, "bot-token"),
		FlowDatabasePath:            filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users?per_page=200" && request.Method == http.MethodGet:
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `[]`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/admin-post-1" && request.Method == http.MethodGet:
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"admin-post-1","user_id":"admin"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/admin-post-1/patch":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusForbidden, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/admin-post-1" && request.Method == http.MethodDelete:
			assertMattermostBearerToken(t, request, "admin-token")
			deletedPostIDs = append(deletedPostIDs, "admin-post-1")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			createdPostToken = request.Header.Get("Authorization")
			return jsonResponse(http.StatusCreated, `{"id":"bot-post-1"}`, nil), nil
		case strings.HasSuffix(request.URL.String(), "/members/bot-1/schemeRoles") && request.Method == http.MethodPut:
			assertMattermostChannelSchemeRoles(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostExistingFlowSetupResponse(t, request), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	task := flowNotificationTestTask("요청")
	task.MattermostPostID = "admin-post-1"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	task = service.syncFlowMattermostNotification(context.Background(), task)
	if task.MattermostPostID != "bot-post-1" {
		t.Fatalf("post id = %q", task.MattermostPostID)
	}
	if strings.Join(deletedPostIDs, "|") != "admin-post-1" {
		t.Fatalf("deleted post ids = %+v", deletedPostIDs)
	}
	if createdPostToken != "Bearer bot-token" {
		t.Fatalf("created post authorization = %q", createdPostToken)
	}
	reloadedTask, found, errorValue := service.readFlowTaskByID(context.Background(), task.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded task: found=%v error=%v", found, errorValue)
	}
	if reloadedTask.MattermostPostID != "bot-post-1" {
		t.Fatalf("reloaded post id = %q", reloadedTask.MattermostPostID)
	}
}

func assertFlowProjectionOutboxCount(t *testing.T, service *Service, expectedCount int) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM flow_channel_outbox").Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count != expectedCount {
		t.Fatalf("flow projection outbox count = %d, want %d", count, expectedCount)
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
	request.Header.Set(flowResolvedActorHeader, "me@example.com")
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
		MattermostBotTokenPath:      writeTestFile(t, "bot-token"),
		FlowDatabasePath:            filepath.Join(t.TempDir(), "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members" && request.Method == http.MethodPost:
			assertMattermostChannelMember(t, request, "bot-1")
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			assertMattermostChannelSchemeRoles(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			assertMattermostChannelSchemeRoles(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users?per_page=200" && request.Method == http.MethodGet:
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `[{"id":"user-iam","username":"iam","first_name":"","last_name":"","nickname":"김민수","email":"iam@example.com"}]`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "bot-token")
			requests.createdMessages = append(requests.createdMessages, mattermostPostMessage(t, request))
			return jsonResponse(http.StatusCreated, `{"id":"flow-post-1"}`, nil), nil
		case isMattermostFlowSetupRequest(request):
			return mattermostExistingFlowSetupResponse(t, request), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-post-1" && request.Method == http.MethodGet:
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"flow-post-1","user_id":"bot-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-post-1/patch" && request.Method == http.MethodPut:
			assertMattermostBearerToken(t, request, "bot-token")
			requests.updatedMessages = append(requests.updatedMessages, mattermostPostMessage(t, request))
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-post-1" && request.Method == http.MethodDelete:
			assertMattermostBearerToken(t, request, "admin-token")
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
		assertMattermostFlowChannelPatch(t, request, "[업무 열기](/flow/)", "[업무 열기](https://dc719d8e.example.test/flow/)")
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/moderations/patch" && request.Method == http.MethodPut:
		assertMattermostFlowChannelModerationPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
		return jsonResponse(http.StatusOK, `{"order":["flow-entry"],"posts":{"flow-entry":{"id":"flow-entry","user_id":"bot-1","message":"Flow에서 이번 주 업무를 보고, 요청하고, 정리합니다.","props":{"internkim_flow_entry":true}}}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-entry" && request.Method == http.MethodDelete:
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=100":
		return jsonResponse(http.StatusOK, `{"order":["system-add"],"posts":{"system-add":{"id":"system-add","type":"system_add_to_channel","message":"lee added to the channel by admin."}}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/posts/system-add" && request.Method == http.MethodDelete:
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/calendar":
		return jsonResponse(http.StatusOK, `{"id":"calendar-channel"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/patch" && request.Method == http.MethodPut:
		assertMattermostCalendarChannelPatch(t, request, "[일정 열기](/calendar/)", "[일정 열기](https://dc719d8e.example.test/calendar/)")
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/posts?per_page=100":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/moderations/patch" && request.Method == http.MethodPut:
		assertMattermostFlowChannelModerationPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members" && request.Method == http.MethodPost:
		return jsonResponse(http.StatusCreated, `{}`, nil)
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

func flowReportTestTask(id string, weekCode string, participantIDs []string, participantNames []string, size string, status string, startDate string, endDate string) flowTask {
	return flowTask{
		ID:               id,
		OwnerID:          participantIDs[0],
		OwnerName:        participantNames[0],
		ParticipantIDs:   participantIDs,
		ParticipantNames: participantNames,
		Business:         "개발",
		Type:             "회의",
		Content:          id,
		Goal:             "거리 리포트 검증",
		Size:             size,
		Status:           status,
		StartDate:        startDate,
		EndDate:          endDate,
		WeekCode:         weekCode,
	}
}

func newFlowAuthorizationTestService(t *testing.T) *Service {
	t.Helper()
	fleetIDPath := writeTestFile(t, "device-1")
	fleetSecretPath := writeTestFile(t, "secret-1")
	service := NewService(Configuration{
		StateDirectory:        filepath.Join(t.TempDir(), "state"),
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
		FleetIDPath:           fleetIDPath,
		FleetSecretPath:       fleetSecretPath,
		FlowDatabasePath:      filepath.Join(t.TempDir(), "flow.sqlite"),
		MattermostBaseURL:     "http://mattermost.local",
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "https://api.example.test/api/users?fleet_id=device-1" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-admin","email":"admin@example.com","name":"Admin","role":"admin","status":"active"},{"userID":"user-staff","email":"staff@example.com","name":"Staff","role":"member","status":"active"},{"userID":"user-other","email":"other@example.com","name":"Other","role":"member","status":"active"}]}`, nil), nil
		}
		if request.URL.String() == "http://mattermost.local/api/v4/users/me" && request.Method == http.MethodGet && strings.Contains(request.Header.Get("Cookie"), "MMAUTHTOKEN=session-token") {
			return jsonResponse(http.StatusOK, `{"id":"staff-mm","email":"staff@example.com","username":"staff"}`, nil), nil
		}
		if request.URL.String() == "http://mattermost.local/api/v4/users/me" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusUnauthorized, `{}`, nil), nil
		}
		if request.URL.Path == "/admin/api/policy" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
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

func assertMattermostBearerToken(t *testing.T, request *http.Request, expectedToken string) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer "+expectedToken {
		t.Fatalf("authorization = %q, expected Bearer %s", request.Header.Get("Authorization"), expectedToken)
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
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/posts?per_page=100":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/off-topic":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/posts?per_page=100":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/members":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/moderations/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/posts/flow-entry" && request.Method == http.MethodDelete:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=100":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/posts/system-add" && request.Method == http.MethodDelete:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/calendar":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/posts?per_page=100":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/moderations/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/members":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/attendance":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/patch" && request.Method == http.MethodPut:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=100":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/posts/attendance-entry" && request.Method == http.MethodDelete:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/posts/attendance-entry/pin" && request.Method == http.MethodPost:
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members":
		return true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
		return true
	default:
		return false
	}
}

func mattermostFlowSetupResponse(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	switch {
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1":
		return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/patch" && request.Method == http.MethodPut:
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/posts?per_page=100":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
		return jsonResponse(http.StatusOK, `{"id":"flow-channel"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/off-topic":
		return jsonResponse(http.StatusOK, `{"id":"off-topic-channel"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/patch" && request.Method == http.MethodPut:
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/posts?per_page=100":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
		assertMattermostFlowChannelPatch(t, request, "[업무 열기](/flow/)", "[업무 열기](https://dc719d8e.example.test/flow/)")
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/moderations/patch" && request.Method == http.MethodPut:
		assertMattermostFlowChannelModerationPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=100":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/calendar":
		return jsonResponse(http.StatusOK, `{"id":"calendar-channel"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/patch" && request.Method == http.MethodPut:
		assertMattermostCalendarChannelPatch(t, request, "[일정 열기](/calendar/)", "[일정 열기](https://dc719d8e.example.test/calendar/)")
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/posts?per_page=100":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/moderations/patch" && request.Method == http.MethodPut:
		assertMattermostFlowChannelModerationPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/attendance":
		return jsonResponse(http.StatusOK, `{"id":"attendance-channel"}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/patch" && request.Method == http.MethodPut:
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=100":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
		return jsonResponse(http.StatusOK, `{"order":["attendance-entry"],"posts":{"attendance-entry":{"id":"attendance-entry","props":{"internkim_attendance_entry":true}}}}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/posts/attendance-entry" && request.Method == http.MethodDelete:
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatal(errorValue)
		}
		if payload["channel_id"] == "attendance-channel" {
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusCreated, `{"id":"attendance-entry"}`, nil)
		}
		if payload["channel_id"] == "flow-channel" {
			t.Fatalf("unexpected flow entry post payload: %+v", payload)
		}
		return jsonResponse(http.StatusCreated, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/posts/attendance-entry/pin" && request.Method == http.MethodPost:
		assertMattermostBearerToken(t, request, "bot-token")
		return jsonResponse(http.StatusOK, `{}`, nil)
	case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members":
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
		trigger := assertMattermostConnectCommandPayload(t, request)
		return jsonResponse(http.StatusCreated, `{"id":"`+trigger+`-command","token":"`+trigger+`-token","team_id":"team-1","trigger":"`+trigger+`"}`, nil)
	default:
		t.Fatalf("unexpected Mattermost command setup request %s %s", request.Method, request.URL.String())
		return nil
	}
}

func newWorkspaceSettingsMattermostTestService(t *testing.T, isAnnouncementsMissing bool) (*Service, map[string]string, map[string]string) {
	t.Helper()
	adminPasswordPath := filepath.Join(t.TempDir(), "admin-pass")
	writeFile(t, adminPasswordPath, "admin-pass")
	service := NewService(Configuration{
		StateDirectory:              t.TempDir(),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
	})
	patchedDisplayNames := map[string]string{}
	createdChannels := map[string]string{}
	channelNamesByID := map[string]string{
		"town-square-channel":   mattermostdefaults.TownSquareChannelName,
		"off-topic-channel":     mattermostdefaults.OffTopicChannelName,
		"flow-channel":          mattermostFlowChannelName,
		"calendar-channel":      mattermostCalendarChannelName,
		"attendance-channel":    attendanceChannelName,
		"announcements-channel": calendarAnnouncementsChannelName,
	}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
			return jsonResponse(http.StatusOK, `{"id":"flow-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"town-square-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/off-topic":
			return jsonResponse(http.StatusOK, `{"id":"off-topic-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/calendar":
			return jsonResponse(http.StatusOK, `{"id":"calendar-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/attendance":
			return jsonResponse(http.StatusOK, `{"id":"attendance-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/announcements" && isAnnouncementsMissing:
			return jsonResponse(http.StatusNotFound, `{"message":"not found"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/announcements":
			return jsonResponse(http.StatusOK, `{"id":"announcements-channel"}`, nil), nil
		case strings.HasPrefix(request.URL.String(), "http://mattermost.local/api/v4/channels/") && strings.HasSuffix(request.URL.String(), "/patch"):
			channelID := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/v4/channels/"), "/patch")
			channelName := channelNamesByID[channelID]
			patchedDisplayNames[channelName] = assertMattermostDisplayNamePatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case strings.HasPrefix(request.URL.String(), "http://mattermost.local/api/v4/channels/") && strings.HasSuffix(request.URL.String(), "/posts?per_page=100"):
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels" && request.Method == http.MethodPost:
			channelName, displayName := assertMattermostPublicChannelCreate(t, request)
			createdChannels[channelName] = displayName
			return jsonResponse(http.StatusCreated, `{"id":"created-channel"}`, nil), nil
		default:
			t.Fatalf("unexpected workspace settings Mattermost request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service, patchedDisplayNames, createdChannels
}

func assertMattermostDisplayNamePatch(t *testing.T, request *http.Request) string {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found := payload["name"]; found {
		t.Fatalf("display name patch must not change channel name: %#v", payload)
	}
	displayName := strings.TrimSpace(payload["display_name"])
	if displayName == "" {
		t.Fatalf("display name patch = %#v", payload)
	}
	return displayName
}

func assertMattermostPublicChannelCreate(t *testing.T, request *http.Request) (string, string) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["team_id"] != "team-1" || payload["type"] != "O" {
		t.Fatalf("public channel create = %#v", payload)
	}
	if payload["name"] == "" || payload["display_name"] == "" {
		t.Fatalf("public channel create = %#v", payload)
	}
	return payload["name"], payload["display_name"]
}

func assertMattermostConnectCommandPayload(t *testing.T, request *http.Request) string {
	t.Helper()
	var payload mattermostCommandRecord
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload.TeamID != "team-1" || !containsString(mattermostManagedCommandTriggers(), payload.Trigger) || payload.Method != "P" {
		t.Fatalf("connect command payload = %+v", payload)
	}
	if payload.URL != "http://127.0.0.1:18080/_internkim/mattermost/commands" || !payload.Autocomplete {
		t.Fatalf("connect command url/autocomplete = %+v", payload)
	}
	return payload.Trigger
}

func assertMattermostFlowChannelPatch(t *testing.T, request *http.Request, expectedFlowLinks ...string) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["display_name"] != flowChannelDisplayName(workspaceLanguageKorean) || !containsString(expectedFlowLinks, payload["header"]) || payload["purpose"] != "" {
		t.Fatalf("flow channel patch = %#v", payload)
	}
}

func assertMattermostCalendarChannelPatch(t *testing.T, request *http.Request, expectedCalendarLinks ...string) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["display_name"] != mattermostdefaults.CalendarChannelDisplayName || !containsString(expectedCalendarLinks, payload["header"]) || payload["purpose"] != "" {
		t.Fatalf("calendar channel patch = %#v", payload)
	}
}

func assertMattermostChannelSchemeRoles(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]bool
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !payload["scheme_admin"] || !payload["scheme_user"] {
		t.Fatalf("channel scheme roles = %#v", payload)
	}
}

func assertMattermostFlowChannelModerationPatch(t *testing.T, request *http.Request) {
	t.Helper()
	var payload []struct {
		Name  string          `json:"name"`
		Roles map[string]bool `json:"roles"`
	}
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedRolesByName := map[string]map[string]bool{
		"create_post":          {"members": true, "guests": false},
		"create_reactions":     {"members": false, "guests": false},
		"manage_members":       {"members": false},
		"use_channel_mentions": {"members": false, "guests": false},
	}
	if len(payload) != len(expectedRolesByName) {
		t.Fatalf("flow moderation patch = %#v", payload)
	}
	for _, patch := range payload {
		expectedRoles, found := expectedRolesByName[patch.Name]
		if !found || !reflect.DeepEqual(patch.Roles, expectedRoles) {
			t.Fatalf("flow moderation patch = %#v", payload)
		}
	}
}

func TestMattermostManagedChannelSystemPostCleanupDeletesMetadataPostsOnly(t *testing.T) {
	deletedPostIDs := []string{}
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":["header","display","purpose","user","entry"],"posts":{"header":{"id":"header","type":"system_header_change"},"display":{"id":"display","type":"system_displayname_change"},"purpose":{"id":"purpose","type":"system_purpose_change"},"user":{"id":"user","type":""},"entry":{"id":"entry","type":"","props":{"internkim_flow_entry":true}}}}`, nil), nil
		case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/api/v4/posts/"):
			deletedPostIDs = append(deletedPostIDs, strings.TrimPrefix(request.URL.Path, "/api/v4/posts/"))
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected cleanup request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.cleanupMattermostManagedChannelSystemPosts(context.Background(), "admin-token", "channel-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedPostIDs := []string{"header", "display", "purpose"}
	if strings.Join(deletedPostIDs, ",") != strings.Join(expectedPostIDs, ",") {
		t.Fatalf("deleted post IDs = %#v", deletedPostIDs)
	}
}

func assertMattermostFlowChannelCreate(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["team_id"] != "team-1" || payload["name"] != "flow" || payload["display_name"] != flowChannelDisplayName(workspaceLanguageKorean) || payload["type"] != "O" {
		t.Fatalf("flow channel create = %#v", payload)
	}
}

func mattermostChannelCreateName(t *testing.T, request *http.Request) string {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	switch payload["name"] {
	case "flow":
		if payload["team_id"] != "team-1" || payload["display_name"] != mattermostdefaults.FlowChannelDisplayName || payload["type"] != "O" {
			t.Fatalf("flow channel create = %#v", payload)
		}
	case "calendar":
		if payload["team_id"] != "team-1" || payload["display_name"] != mattermostdefaults.CalendarChannelDisplayName || payload["type"] != "O" {
			t.Fatalf("calendar channel create = %#v", payload)
		}
	default:
		t.Fatalf("channel create = %#v", payload)
	}
	return payload["name"]
}

func assertMattermostCalendarChannelCreate(t *testing.T, request *http.Request) {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["team_id"] != "team-1" || payload["name"] != "calendar" || payload["display_name"] != mattermostdefaults.CalendarChannelDisplayName || payload["type"] != "O" {
		t.Fatalf("calendar channel create = %#v", payload)
	}
}

func mattermostChannelMemberID(t *testing.T, request *http.Request) string {
	t.Helper()
	return mattermostChannelMemberUserID(t, request)
}

func assertMattermostChannelMember(t *testing.T, request *http.Request, expectedUserID string) {
	t.Helper()
	userID := mattermostChannelMemberUserID(t, request)
	if userID != expectedUserID {
		t.Fatalf("channel member = %q", userID)
	}
}

func mattermostChannelMemberUserID(t *testing.T, request *http.Request) string {
	t.Helper()
	var payload map[string]string
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	return payload["user_id"]
}

func assertMattermostFlowEntryPost(t *testing.T, request *http.Request, expectedMessages ...string) {
	t.Helper()
	assertMattermostBearerToken(t, request, "bot-token")
	var payload map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	props, _ := payload["props"].(map[string]any)
	message, _ := payload["message"].(string)
	if payload["channel_id"] != "flow-channel" || !containsString(expectedMessages, message) || props["internkim_flow_entry"] != true {
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
	fleetIDPath := filepath.Join(stateDirectory, "fleet-id")
	adminPasswordPath := filepath.Join(stateDirectory, "admin-pass")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, adminPasswordPath, "admin-pass")
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		FleetIDPath:                 fleetIDPath,
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

func TestMattermostStopCommandCallsBlueclawTaskCancel(t *testing.T) {
	stateDirectory := t.TempDir()
	adminPasswordPath := filepath.Join(stateDirectory, "admin-pass")
	writeFile(t, adminPasswordPath, "admin-pass")
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: adminPasswordPath,
		BlueclawBaseURL:             "http://blueclaw.local",
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	writeFile(t, service.mattermostConnectCommandTokenPath(), "stop-token")
	blueclawStopCalled := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"Alice@Example.com","username":"alice","nickname":"Alice"}`, nil), nil
		case request.URL.String() == "http://blueclaw.local/admin/api/task/cancel":
			blueclawStopCalled = true
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["mode"] != "stop" || payload["requesterEmail"] != "alice@example.com" {
				t.Fatalf("unexpected stop payload = %#v", payload)
			}
			return jsonResponse(http.StatusOK, `{"cancelledTaskRunCount":1,"scheduleTouched":false}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	handler := service.router()

	form := url.Values{
		"command":    []string{"/stop"},
		"token":      []string{"stop-token"},
		"user_id":    []string{"user-1"},
		"user_name":  []string{"alice"},
		"channel_id": []string{"channel-1"},
	}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/commands", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("stop command status = %d body = %s", response.Code, response.Body.String())
	}
	if !blueclawStopCalled {
		t.Fatal("Blueclaw stop endpoint was not called")
	}
	var slashResponse mattermostSlashCommandResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&slashResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if slashResponse.ResponseType != "ephemeral" || !strings.Contains(slashResponse.Text, "1개를 중단") {
		t.Fatalf("slash response = %+v", slashResponse)
	}
}

func TestCORSHeaderIsLimitedToInternKimPaths(t *testing.T) {
	service := NewService(Configuration{})
	handler := service.withCORS(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Access-Control-Allow-Origin", "https://mattermost.example")
		responseWriter.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v4/system/ping", nil)
	request.Header.Set("Origin", "https://device.example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if origins := response.Result().Header.Values("Access-Control-Allow-Origin"); !reflect.DeepEqual(origins, []string{"https://mattermost.example"}) {
		t.Fatalf("unexpected proxy CORS origins: %#v", origins)
	}
}

func TestCORSHeaderIsAddedForInternKimPaths(t *testing.T) {
	service := NewService(Configuration{})
	handler := service.withCORS(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil)
	request.Header.Set("Origin", "https://device.example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Access-Control-Allow-Origin") != "https://device.example.test" {
		t.Fatalf("unexpected InternKim CORS origin: %s", response.Header().Get("Access-Control-Allow-Origin"))
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
	expectedTokens := []string{"connect-token", "stop-token", "stop-all-token"}
	if !stringFieldsMatch(readTrimmedFile(service.mattermostConnectCommandTokenPath()), expectedTokens) {
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
	fleetIDPath := filepath.Join(envDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(stateDirectory, "fleet-secret")
	writeFile(t, adminPasswordPath, "admin-pass")
	writeFile(t, fleetIDPath, "device-1")
	writeFile(t, fleetSecretPath, "fleet-secret")
	flowChannelCreated := false
	flowChannelPatched := false
	adminJoinedFlowChannel := false
	calendarChannelCreated := false
	calendarChannelPatched := false
	adminJoinedCalendarChannel := false
	adminJoinedAttendanceChannel := false
	staffJoinedDefaultChannels := map[string]bool{}
	botTokenPath := filepath.Join(stateDirectory, "bot-token")
	writeFile(t, botTokenPath, "bot-token")
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		MattermostBaseURL:           "http://mattermost.local",
		APIBaseURL:                  "https://api.intern.test",
		MattermostAdminPasswordPath: adminPasswordPath,
		MattermostBotTokenPath:      botTokenPath,
		MattermostOAuthClientPath:   filepath.Join(stateDirectory, "mattermost-oauth-client.json"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "https://api.intern.test/api/users?fleet_id=device-1":
			return jsonResponse(http.StatusOK, `{"records":[{"email":"staff@example.com","role":"member","mattermostUserID":"staff-1","mattermostUsername":"staff"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
			return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/staff-1":
			return jsonResponse(http.StatusOK, `{"id":"staff-1","email":"staff@example.com","username":"staff"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
			assertMattermostRuntimeSettingsPatch(t, request, "https://device-1.example.test")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"town-square-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel":
			return jsonResponse(http.StatusOK, `{"id":"town-square-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/off-topic":
			return jsonResponse(http.StatusOK, `{"id":"off-topic-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/attendance":
			return jsonResponse(http.StatusOK, `{"id":"attendance-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels" && request.Method == http.MethodPost:
			channelName := mattermostChannelCreateName(t, request)
			if channelName == "flow" {
				flowChannelCreated = true
				return jsonResponse(http.StatusCreated, `{"id":"flow-channel"}`, nil), nil
			}
			if channelName == "calendar" {
				calendarChannelCreated = true
				return jsonResponse(http.StatusCreated, `{"id":"calendar-channel"}`, nil), nil
			}
			t.Fatalf("unexpected created channel name %q", channelName)
			return nil, nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
			flowChannelPatched = true
			assertMattermostFlowChannelPatch(t, request, "[업무 열기](https://device-1.example.test/flow/)")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/moderations/patch" && request.Method == http.MethodPut:
			assertMattermostFlowChannelModerationPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			assertMattermostChannelSchemeRoles(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/calendar":
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/patch" && request.Method == http.MethodPut:
			calendarChannelPatched = true
			assertMattermostCalendarChannelPatch(t, request, "[일정 열기](https://device-1.example.test/calendar/)")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/moderations/patch" && request.Method == http.MethodPut:
			assertMattermostFlowChannelModerationPatch(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload["channel_id"] == "flow-channel" {
				t.Fatalf("unexpected flow entry post payload: %+v", payload)
			}
			if payload["channel_id"] == "attendance-channel" {
				assertMattermostBearerToken(t, request, "bot-token")
				return jsonResponse(http.StatusCreated, `{"id":"attendance-entry"}`, nil), nil
			}
			t.Fatalf("unexpected post payload: %+v", payload)
			return nil, nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/attendance-entry/pin" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel/members" && request.Method == http.MethodPost:
			if mattermostChannelMemberUserID(t, request) == "staff-1" {
				staffJoinedDefaultChannels["town-square-channel"] = true
			}
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/members" && request.Method == http.MethodPost:
			if mattermostChannelMemberUserID(t, request) == "staff-1" {
				staffJoinedDefaultChannels["off-topic-channel"] = true
			}
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members" && request.Method == http.MethodPost:
			userID := mattermostChannelMemberUserID(t, request)
			if userID == "admin" {
				adminJoinedFlowChannel = true
			}
			if userID == "staff-1" {
				staffJoinedDefaultChannels["flow-channel"] = true
			}
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/members" && request.Method == http.MethodPost:
			userID := mattermostChannelMemberUserID(t, request)
			if userID == "admin" {
				adminJoinedCalendarChannel = true
			}
			if userID == "staff-1" {
				staffJoinedDefaultChannels["calendar-channel"] = true
			}
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members" && request.Method == http.MethodPost:
			userID := mattermostChannelMemberUserID(t, request)
			if userID == "admin" {
				adminJoinedAttendanceChannel = true
			}
			if userID == "staff-1" {
				staffJoinedDefaultChannels["attendance-channel"] = true
			}
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			assertMattermostChannelSchemeRoles(t, request)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
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
	if !flowChannelCreated || !flowChannelPatched || !adminJoinedFlowChannel || !calendarChannelCreated || !calendarChannelPatched || !adminJoinedCalendarChannel || !adminJoinedAttendanceChannel {
		t.Fatalf("default channel setup flags flow=%v/%v/%v calendar=%v/%v/%v attendance=%v", flowChannelCreated, flowChannelPatched, adminJoinedFlowChannel, calendarChannelCreated, calendarChannelPatched, adminJoinedCalendarChannel, adminJoinedAttendanceChannel)
	}
	for _, channelID := range []string{"town-square-channel", "off-topic-channel", "flow-channel", "calendar-channel", "attendance-channel"} {
		if !staffJoinedDefaultChannels[channelID] {
			t.Fatalf("staff was not joined to default channel %s: %#v", channelID, staffJoinedDefaultChannels)
		}
	}
}

func TestMattermostDefaultChannelProvisioningAttemptsAllChannels(t *testing.T) {
	stateDirectory := t.TempDir()
	envDirectory := filepath.Join(stateDirectory, "env")
	if errorValue := os.MkdirAll(envDirectory, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	attendanceChannelPatched := false
	attendanceEntryCreated := false
	botTokenPath := filepath.Join(stateDirectory, "bot-token")
	writeFile(t, botTokenPath, "bot-token")
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		FleetIDPath:            filepath.Join(envDirectory, "fleet-id"),
		AdminEmailPath:         writeTestFile(t, "admin@example.com"),
		MattermostBaseURL:      "http://mattermost.local",
		MattermostBotTokenPath: botTokenPath,
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
			return jsonResponse(http.StatusOK, `{"id":"town-square-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/town-square-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/off-topic":
			return jsonResponse(http.StatusOK, `{"id":"off-topic-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/off-topic-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/flow":
			return jsonResponse(http.StatusOK, `{"id":"flow-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/moderations/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/calendar":
			return jsonResponse(http.StatusInternalServerError, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/attendance":
			return jsonResponse(http.StatusOK, `{"id":"attendance-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/patch" && request.Method == http.MethodPut:
			attendanceChannelPatched = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "bot-token")
			attendanceEntryCreated = true
			return jsonResponse(http.StatusCreated, `{"id":"attendance-entry"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/attendance-entry/pin" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected default channel request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	channelIDs, errorValue := service.ensureMattermostDefaultChannelIDs(context.Background(), "admin-token", "team-1")

	if errorValue == nil || !strings.Contains(errorValue.Error(), "calendar channel") {
		t.Fatalf("default channel error = %v", errorValue)
	}
	for _, channelID := range []string{"town-square-channel", "off-topic-channel", "flow-channel", "attendance-channel"} {
		if !containsString(channelIDs, channelID) {
			t.Fatalf("default channel ids = %#v", channelIDs)
		}
	}
	if !attendanceChannelPatched || !attendanceEntryCreated {
		t.Fatalf("attendance channel provisioning patch=%v entry=%v", attendanceChannelPatched, attendanceEntryCreated)
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
			trigger := assertMattermostConnectCommandPayload(t, request)
			return jsonResponse(http.StatusCreated, `{"id":"`+trigger+`-command","token":"new-`+trigger+`-token","team_id":"team-1","trigger":"`+trigger+`"}`, nil), nil
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
	expectedTokens := []string{"new-connect-token", "new-stop-token", "new-stop-all-token"}
	if !stringFieldsMatch(readTrimmedFile(service.mattermostConnectCommandTokenPath()), expectedTokens) {
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

	claimedBobJob, errorValue := service.claimNextCompanionJob(bob)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if claimedBobJob != nil {
		t.Fatalf("expected Bob companion not to claim Alice job, got %+v", claimedBobJob)
	}
	claimedJob, errorValue := service.claimNextCompanionJob(alice)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
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

	claimedBobJob, errorValue := service.claimNextCompanionJob(bobCompanion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if claimedBobJob != nil {
		t.Fatalf("expected Bob companion not to claim Alice job, got %+v", claimedBobJob)
	}
	claimedJob, errorValue := service.claimNextCompanionJob(aliceCompanion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
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

	claimedJob, errorValue := service.claimNextCompanionJob(companion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
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

	claimedJob, errorValue := service.claimNextCompanionJob(activeCompanion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

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

	claimedJob, errorValue := service.claimNextCompanionJob(companion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

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

	claimedBobJob, errorValue := service.claimNextCompanionJob(bobCompanion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if claimedBobJob != nil {
		t.Fatalf("expected Bob not to claim Alice attention watch, got %+v", claimedBobJob)
	}
	claimedJob, errorValue := service.claimNextCompanionJob(aliceCompanion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
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

	claimedJob, errorValue := service.claimNextCompanionJob(companion)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if claimedJob != nil {
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

func TestRuntimeRemoteModelReadAndUpdate(t *testing.T) {
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
	readRequest := httptest.NewRequest(http.MethodGet, "/_internkim/runtime/remote-model", nil)
	setCompanionHeaders(t, readRequest, pairResult.companionPairResponse, pairResult.privateKey)
	handler.ServeHTTP(readResponse, readRequest)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read status = %d: %s", readResponse.Code, readResponse.Body.String())
	}
	var readResult runtimeRemoteModelResponse
	if errorValue := json.NewDecoder(readResponse.Body).Decode(&readResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	if readResult.Model != "google/old-model" {
		t.Fatalf("expected old model, got %+v", readResult)
	}

	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/_internkim/runtime/remote-model", strings.NewReader(`{"model":"google/new-model"}`))
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

func TestRemoteModelIsNotCompanionBrokerEndpoint(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	request := httptest.NewRequest(http.MethodGet, "/_internkim/companion/remote-model", nil)
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected companion remote model endpoint to be absent, got %d", response.Code)
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
	assertMattermostManagedResourcePathPatch(t, payload)
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
	assertMattermostManagedResourcePathPatch(t, payload)
	teamSettings := payload["TeamSettings"]
	if teamSettings["TeammateNameDisplay"] != "nickname_full_name" {
		t.Fatalf("teammate name display = %#v", teamSettings["TeammateNameDisplay"])
	}
}

func assertMattermostManagedResourcePathPatch(t *testing.T, payload map[string]map[string]any) {
	t.Helper()
	serviceSettings := payload["ServiceSettings"]
	if serviceSettings["ManagedResourcePaths"] != mattermostdefaults.ManagedResourcePathSetting() {
		t.Fatalf("managed resource paths = %#v", serviceSettings["ManagedResourcePaths"])
	}
	if serviceSettings["AllowedUntrustedInternalConnections"] != "127.0.0.1 localhost" {
		t.Fatalf("allowed internal connections = %#v", serviceSettings["AllowedUntrustedInternalConnections"])
	}
	if serviceSettings["EnableUserAccessTokens"] != true {
		t.Fatalf("user access tokens = %#v", serviceSettings["EnableUserAccessTokens"])
	}
	if serviceSettings["EnableBotAccountCreation"] != true {
		t.Fatalf("bot account creation = %#v", serviceSettings["EnableBotAccountCreation"])
	}
}

func assertMattermostRuntimeSettingsPatch(t *testing.T, request *http.Request, siteURL string) {
	t.Helper()
	var payload map[string]map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertMattermostManagedResourcePathPatch(t, payload)
	serviceSettings := payload["ServiceSettings"]
	if serviceSettings["SiteURL"] != siteURL {
		t.Fatalf("site url = %#v", serviceSettings["SiteURL"])
	}
	if serviceSettings["AllowCorsFrom"] != siteURL {
		t.Fatalf("allow cors from = %#v", serviceSettings["AllowCorsFrom"])
	}
	if serviceSettings["CorsAllowCredentials"] != true {
		t.Fatalf("cors allow credentials = %#v", serviceSettings["CorsAllowCredentials"])
	}
	if serviceSettings["EnableOAuthServiceProvider"] != true {
		t.Fatalf("oauth service provider = %#v", serviceSettings["EnableOAuthServiceProvider"])
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
	return `{"people":[{"personID":"00000000-0000-0000-0000-000000000001","displayName":"Intern Kim Admin","emails":["eastriver0720@gmail.com"],"securityLevelName":"admin","securityLevelRank":100,"grantedClasses":["internal","executive"],"isAdmin":true},{"personID":"member-1","displayName":"lee","emails":["lee@example.com"],"securityLevelName":"member","securityLevelRank":10,"grantedClasses":["internal"],"isAdmin":false}],"channels":[],"retention":{"rawEventDays":60}}`
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

func stringFieldsMatch(document string, expectedFields []string) bool {
	actualFields := strings.Fields(document)
	if len(actualFields) != len(expectedFields) {
		return false
	}
	expectedFieldSet := map[string]bool{}
	for _, expectedField := range expectedFields {
		expectedFieldSet[expectedField] = true
	}
	for _, actualField := range actualFields {
		if !expectedFieldSet[actualField] {
			return false
		}
	}
	return true
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
