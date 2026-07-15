package tenantruntime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		CredentialsPath:     credentialsPath,
		BaseURLTemplate:     server.URL,
		BlueclawURLTemplate: server.URL + "/blueclaw/{tenant}/{blueclawPort}",
		PublicURLTemplate:   "https://{tenant}.intern.kim",
		PortStart:           18065,
		Language:            "ko",
		TokenOutputRoot:     tokenOutputRoot,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(statuses) != 1 || statuses[0].TenantID != "pilot-01" || statuses[0].BotUserID != "bot-1" {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
	if statuses[0].AdminEmail != "admin@pilot-01.local" || !statuses[0].BlueclawInvited {
		t.Fatalf("expected admin email and Blueclaw invite in bootstrap status: %+v", statuses[0])
	}
	for _, channelName := range []string{"flow", "calendar", "attendance"} {
		if statuses[0].Channels[channelName] == "" {
			t.Fatalf("expected channel %q in bootstrap status: %+v", channelName, statuses[0].Channels)
		}
	}
	if server.adminEmail != "admin@pilot-01.local" || server.adminPassword != "password" {
		t.Fatalf("expected Mattermost admin profile patch, got email=%q password=%q", server.adminEmail, server.adminPassword)
	}
	if server.blueclawInvitedEmail != "admin@pilot-01.local" || server.blueclawInvitePath != "/blueclaw/pilot-01/18100/admin/api/people/invite" {
		t.Fatalf("expected Blueclaw invite, got email=%q path=%q", server.blueclawInvitedEmail, server.blueclawInvitePath)
	}
	assertFileContains(t, filepath.Join(tokenOutputRoot, "pilot-01", "internkim", "secrets", "mattermost-bot-token"), "bot-token-1")
	if server.configuration["ServiceSettings"].(map[string]any)["ManagedResourcePaths"] != "admin,attendance,calendar,flow,mail,memory" {
		t.Fatalf("unexpected configuration patch: %+v", server.configuration)
	}
	if server.channels["flow"].DisplayName != "업무" || server.channels["attendance"].DisplayName != "근태" {
		t.Fatalf("expected localized channels, got %+v", server.channels)
	}
	if !server.teamMembers["admin-1"] || !server.teamMembers["bot-1"] {
		t.Fatalf("expected admin and bot team members, got %+v", server.teamMembers)
	}
	if !server.channelMembers["channel-flow:admin-1"] || !server.channelMembers["channel-calendar:admin-1"] || !server.channelMembers["channel-attendance:admin-1"] {
		t.Fatalf("expected admin default channel memberships, got %+v", server.channelMembers)
	}
	if !server.schemeRoles["channel-flow:bot-1"] || !server.schemeRoles["channel-calendar:bot-1"] || !server.schemeRoles["channel-attendance:bot-1"] {
		t.Fatalf("expected bot channel scheme roles, got %+v", server.schemeRoles)
	}
	if !server.deletedWelcomePost {
		t.Fatal("expected Mattermost bot welcome DM cleanup")
	}
}

func TestMattermostBootstrapRejectsExistingHumanInternKimAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v4/bots":
			writeMattermostBootstrapJSON(responseWriter, http.StatusBadRequest, map[string]string{"message": "username exists"})
		case "/api/v4/users/username/internkim":
			writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]any{"id": "human-1", "username": "internkim", "is_bot": false})
		case "/api/v4/users/human-1":
			writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]any{"id": "human-1", "username": "internkim", "is_bot": false})
		default:
			writeMattermostBootstrapJSON(responseWriter, http.StatusNotFound, map[string]string{"path": request.URL.Path})
		}
	}))
	defer server.Close()

	client := mattermostBootstrapClient{baseURL: server.URL, httpClient: server.Client()}
	_, _, errorValue := client.ensureBot("admin-token", "team-1")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "is not a bot") {
		t.Fatalf("expected human account rejection, got %v", errorValue)
	}
}

func TestBootstrapMattermostTenantResourcesProvisionMembersIdempotently(t *testing.T) {
	server := newMattermostBootstrapTestServer(t)
	server.usersByEmail["existing@example.com"] = mattermostBootstrapTestUser{ID: "user-existing", Username: "existing"}

	status, errorValue := BootstrapMattermostTenantResources("pilot-01", "admin", "password", MattermostFleetBootstrapOptions{
		BaseURLTemplate:     server.URL,
		BlueclawURLTemplate: server.URL + "/blueclaw/{tenant}/{blueclawPort}",
		PublicURLTemplate:   "https://{tenant}.intern.kim",
		PortStart:           18065,
		Language:            "ko",
		Members: []MattermostBootstrapMember{
			{Email: "new.member@example.com", Name: "New Member", Password: "member-password"},
			{Email: "existing@example.com", Name: "Existing Member", Password: "member-password"},
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	assertMattermostBootstrapMemberOutcome(t, status.Members, "new.member@example.com", "created")
	assertMattermostBootstrapMemberOutcome(t, status.Members, "existing@example.com", "already existed")
	if !server.teamMembers["user-new-member"] || !server.teamMembers["user-existing"] {
		t.Fatalf("expected member team memberships, got %+v", server.teamMembers)
	}
	if !server.channelMembers["channel-flow:user-new-member"] || !server.channelMembers["channel-flow:user-existing"] {
		t.Fatalf("expected member channel memberships, got %+v", server.channelMembers)
	}
	if server.blueclawInvites["new.member@example.com"] != "tenant-pilot-01-new.member" {
		t.Fatalf("expected new member Blueclaw invite, got %+v", server.blueclawInvites)
	}

	secondStatus, errorValue := BootstrapMattermostTenantResources("pilot-01", "admin", "password", MattermostFleetBootstrapOptions{
		BaseURLTemplate:     server.URL,
		BlueclawURLTemplate: "",
		PublicURLTemplate:   "https://{tenant}.intern.kim",
		PortStart:           18065,
		Language:            "ko",
		Members: []MattermostBootstrapMember{
			{Email: "new.member@example.com", Name: "New Member", Password: "member-password"},
			{Email: "existing@example.com", Name: "Existing Member", Password: "member-password"},
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertMattermostBootstrapMemberOutcome(t, secondStatus.Members, "new.member@example.com", "already existed")
	assertMattermostBootstrapMemberOutcome(t, secondStatus.Members, "existing@example.com", "already existed")
}

func assertMattermostBootstrapMemberOutcome(t *testing.T, results []MattermostBootstrapMemberResult, email string, outcome string) {
	t.Helper()
	for _, result := range results {
		if result.Email == email && result.Outcome == outcome && result.UserID != "" {
			return
		}
	}
	t.Fatalf("expected member %s outcome %s, got %+v", email, outcome, results)
}

func writeMattermostBootstrapTestCredentials(t *testing.T, path string) {
	t.Helper()
	document := `[{"tenantID":"pilot-01","adminUsername":"admin","adminPassword":"password"}]`
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

type mattermostBootstrapTestServer struct {
	adminEmail           string
	adminPassword        string
	blueclawInvitedEmail string
	blueclawInvitePath   string
	blueclawInvites      map[string]string
	configuration        map[string]any
	channels             map[string]mattermostBootstrapTestChannel
	channelMembers       map[string]bool
	teamMembers          map[string]bool
	schemeRoles          map[string]bool
	deletedWelcomePost   bool
	usersByEmail         map[string]mattermostBootstrapTestUser
	URL                  string
}

type mattermostBootstrapTestUser struct {
	ID       string
	Username string
}

type mattermostBootstrapTestChannel struct {
	ID          string
	DisplayName string
	Header      string
}

func newMattermostBootstrapTestServer(t *testing.T) *mattermostBootstrapTestServer {
	t.Helper()
	server := &mattermostBootstrapTestServer{
		configuration:   map[string]any{},
		channels:        map[string]mattermostBootstrapTestChannel{},
		channelMembers:  map[string]bool{},
		teamMembers:     map[string]bool{},
		schemeRoles:     map[string]bool{},
		blueclawInvites: map[string]string{},
		usersByEmail:    map[string]mattermostBootstrapTestUser{},
		URL:             "https://mattermost-bootstrap.test",
	}
	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mattermostBootstrapTestTransport(func(request *http.Request) (*http.Response, error) {
		response := httptest.NewRecorder()
		server.handle(response, request)
		return response.Result(), nil
	})
	t.Cleanup(func() {
		http.DefaultClient.Transport = originalTransport
	})
	return server
}

type mattermostBootstrapTestTransport func(request *http.Request) (*http.Response, error)

func (transport mattermostBootstrapTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
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
	if request.URL.Path == "/api/v4/users/admin-1/patch" && request.Method == http.MethodPut {
		body := readMattermostBootstrapTestBody(request)
		server.adminEmail, _ = body["email"].(string)
		server.adminPassword, _ = body["password"].(string)
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if request.URL.Path == "/api/v4/users/admin-1/roles" && request.Method == http.MethodPut {
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
	if request.URL.Path == "/api/v4/users/bot-1" && request.Method == http.MethodGet {
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]any{"id": "bot-1", "username": "internkim", "is_bot": true})
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
	if strings.HasPrefix(request.URL.Path, "/api/v4/users/email/") && request.Method == http.MethodGet {
		email, _ := url.PathUnescape(strings.TrimPrefix(request.URL.Path, "/api/v4/users/email/"))
		user, isFound := server.usersByEmail[email]
		if !isFound {
			writeMattermostBootstrapJSON(responseWriter, http.StatusNotFound, map[string]string{"id": ""})
			return
		}
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]string{"id": user.ID, "username": user.Username})
		return
	}
	if request.URL.Path == "/api/v4/users" && request.Method == http.MethodPost {
		body := readMattermostBootstrapTestBody(request)
		email, _ := body["email"].(string)
		username, _ := body["username"].(string)
		if _, isFound := server.usersByEmail[email]; isFound {
			writeMattermostBootstrapJSON(responseWriter, http.StatusBadRequest, map[string]string{"message": "email exists"})
			return
		}
		userID := "user-" + strings.ReplaceAll(username, ".", "-")
		server.usersByEmail[email] = mattermostBootstrapTestUser{ID: userID, Username: username}
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]string{"id": userID})
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
		parts := strings.Split(request.URL.Path, "/")
		body := readMattermostBootstrapTestBody(request)
		userID, _ := body["user_id"].(string)
		if len(parts) >= 5 && strings.TrimSpace(userID) != "" {
			server.channelMembers[parts[4]+":"+userID] = true
		}
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]bool{"ok": true})
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v4/channels/channel-") && strings.HasSuffix(request.URL.Path, "/schemeRoles") && request.Method == http.MethodPut {
		parts := strings.Split(request.URL.Path, "/")
		if len(parts) >= 8 {
			server.schemeRoles[parts[4]+":"+parts[6]] = true
		}
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if request.URL.Path == "/api/v4/channels/direct" && request.Method == http.MethodPost {
		writeMattermostBootstrapJSON(responseWriter, http.StatusCreated, map[string]string{"id": "direct-1"})
		return
	}
	if request.URL.Path == "/api/v4/channels/direct-1/posts" && request.Method == http.MethodGet {
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]any{
			"order": []string{"welcome-1"},
			"posts": map[string]any{
				"welcome-1": map[string]string{
					"message": defaultMattermostBotWelcomeMessage,
				},
			},
		})
		return
	}
	if request.URL.Path == "/api/v4/posts/welcome-1" && request.Method == http.MethodDelete {
		server.deletedWelcomePost = true
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if strings.HasSuffix(request.URL.Path, "/admin/api/people/invite") && request.Method == http.MethodPost {
		body := readMattermostBootstrapTestBody(request)
		server.blueclawInvitedEmail, _ = body["email"].(string)
		server.blueclawInvitePath = request.URL.Path
		personID, _ := body["personID"].(string)
		server.blueclawInvites[server.blueclawInvitedEmail] = personID
		writeMattermostBootstrapJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
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
