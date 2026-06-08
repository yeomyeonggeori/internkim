package tenantruntime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapMattermostFleetResourcesEnsuresBotChannelsAndToken(t *testing.T) {
	server := newMattermostBootstrapTestServer(t)
	credentialsPath := filepath.Join(t.TempDir(), "credentials.json")
	tokenOutputRoot := t.TempDir()
	writeMattermostBootstrapTestCredentials(t, credentialsPath)

	statuses, errorValue := BootstrapMattermostFleetResources(MattermostFleetBootstrapOptions{
		CredentialsPath:   credentialsPath,
		BaseURLTemplate:   server.URL,
		PublicURLTemplate: "https://{tenant}.example.test",
		PortStart:         18065,
		Language:          "ko",
		TokenOutputRoot:   tokenOutputRoot,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(statuses) != 1 || statuses[0].TenantID != "pilot-01" || statuses[0].BotUserID != "bot-1" {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
	for _, channelName := range []string{"flow", "calendar", "attendance"} {
		if statuses[0].Channels[channelName] == "" {
			t.Fatalf("expected channel %q in bootstrap status: %+v", channelName, statuses[0].Channels)
		}
	}
	assertFileContains(t, filepath.Join(tokenOutputRoot, "pilot-01", "internkim", "secrets", "mattermost-bot-token"), "bot-token-1")
	if server.configuration["ServiceSettings"].(map[string]any)["ManagedResourcePaths"] != "admin,attendance,calendar,flow,mail,memory" {
		t.Fatalf("unexpected configuration patch: %+v", server.configuration)
	}
	if server.channels["flow"].DisplayName != "업무" || server.channels["attendance"].DisplayName != "출결" {
		t.Fatalf("expected localized channels, got %+v", server.channels)
	}
	if !server.teamMembers["admin-1"] || !server.teamMembers["bot-1"] {
		t.Fatalf("expected admin and bot team members, got %+v", server.teamMembers)
	}
}

func writeMattermostBootstrapTestCredentials(t *testing.T, path string) {
	t.Helper()
	document := `[{"tenantID":"pilot-01","adminUsername":"admin","adminPassword":"password"}]`
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

type mattermostBootstrapTestServer struct {
	*httptest.Server
	configuration map[string]any
	channels      map[string]mattermostBootstrapTestChannel
	teamMembers   map[string]bool
}

type mattermostBootstrapTestChannel struct {
	ID          string
	DisplayName string
	Header      string
}

func newMattermostBootstrapTestServer(t *testing.T) *mattermostBootstrapTestServer {
	t.Helper()
	server := &mattermostBootstrapTestServer{
		configuration: map[string]any{},
		channels:      map[string]mattermostBootstrapTestChannel{},
		teamMembers:   map[string]bool{},
	}
	server.Server = httptest.NewServer(http.HandlerFunc(server.handle))
	t.Cleanup(server.Close)
	return server
}

func (server *mattermostBootstrapTestServer) handle(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/api/v4/users/login" && request.Method == http.MethodPost {
		responseWriter.Header().Set("Token", "admin-token")
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]string{"id": "admin-1"})
		return
	}
	if request.URL.Path == "/api/v4/config/patch" && request.Method == http.MethodPut {
		server.configuration = readMattermostBootstrapTestBody(request)
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if request.URL.Path == "/api/v4/teams/name/internkim" && request.Method == http.MethodGet {
		writeMattermostBootstrapJSON(responseWriter, http.StatusNotFound, map[string]string{"id": ""})
		return
	}
	if request.URL.Path == "/api/v4/teams" && request.Method == http.MethodPost {
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]string{"id": "team-1"})
		return
	}
	if request.URL.Path == "/api/v4/bots" && request.Method == http.MethodPost {
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]string{"user_id": "bot-1"})
		return
	}
	if request.URL.Path == "/api/v4/users/bot-1/patch" && request.Method == http.MethodPut {
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if request.URL.Path == "/api/v4/teams/team-1/members" && request.Method == http.MethodPost {
		body := readMattermostBootstrapTestBody(request)
		userID, _ := body["user_id"].(string)
		if strings.TrimSpace(userID) != "" {
			server.teamMembers[userID] = true
		}
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]bool{"ok": true})
		return
	}
	if request.URL.Path == "/api/v4/users/bot-1/tokens" && request.Method == http.MethodPost {
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]string{"token": "bot-token-1"})
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v4/teams/team-1/channels/name/") && request.Method == http.MethodGet {
		channelName := strings.TrimPrefix(request.URL.Path, "/api/v4/teams/team-1/channels/name/")
		channel, isFound := server.channels[channelName]
		if !isFound {
			writeMattermostBootstrapJSON(responseWriter, http.StatusNotFound, map[string]string{"id": ""})
			return
		}
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]string{"id": channel.ID})
		return
	}
	if request.URL.Path == "/api/v4/channels" && request.Method == http.MethodPost {
		body := readMattermostBootstrapTestBody(request)
		channelName := body["name"].(string)
		channelID := "channel-" + channelName
		server.channels[channelName] = mattermostBootstrapTestChannel{ID: channelID, DisplayName: body["display_name"].(string)}
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]string{"id": channelID})
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v4/channels/channel-") && strings.HasSuffix(request.URL.Path, "/patch") && request.Method == http.MethodPut {
		channelName := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/v4/channels/channel-"), "/patch")
		body := readMattermostBootstrapTestBody(request)
		server.channels[channelName] = mattermostBootstrapTestChannel{
			ID:          "channel-" + channelName,
			DisplayName: body["display_name"].(string),
			Header:      body["header"].(string),
		}
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v4/channels/channel-") && strings.HasSuffix(request.URL.Path, "/members") && request.Method == http.MethodPost {
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]bool{"ok": true})
		return
	}
	writeMattermostBootstrapJSON(responseWriter, http.StatusNotFound, map[string]string{"path": request.URL.Path})
}

func readMattermostBootstrapTestBody(request *http.Request) map[string]any {
	document := map[string]any{}
	_ = json.NewDecoder(request.Body).Decode(&document)
	return document
}

func writeMattermostBootstrapJSON(responseWriter http.ResponseWriter, statusCode int, document any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(document)
}
