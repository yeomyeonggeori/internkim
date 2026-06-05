package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

func TestSyncCloudflareAccessRegistersCurrentFleetState(t *testing.T) {
	stateDirectory := t.TempDir()
	saveState(stateDirectory, "fleet_id", "fleet-1")
	saveState(stateDirectory, "fleet_secret", "secret-1")
	saveState(stateDirectory, "node_id", "node-1")
	saveState(stateDirectory, "node_key", "node-key-1")
	saveState(stateDirectory, "google_email", "admin@example.com")

	originalClient := registerHTTPClient
	defer func() { registerHTTPClient = originalClient }()
	registerHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/register" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer register-secret" {
			t.Fatalf("unexpected authorization header: %s", request.Header.Get("Authorization"))
		}
		var body map[string]string
		if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
			t.Fatal(errorValue)
		}
		expectedBody := map[string]string{
			"fleet_id":     "fleet-1",
			"fleet_secret": "secret-1",
			"node_id":      "node-1",
			"node_key":     "node-key-1",
			"admin_email":  "admin@example.com",
		}
		for key, expectedValue := range expectedBody {
			if body[key] != expectedValue {
				t.Fatalf("request body %s = %q, expected %q", key, body[key], expectedValue)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(bytes.NewBufferString(`{
			"fleet_id":"fleet-1",
			"node_id":"node-1",
			"tunnel_token":"tunnel-token-2",
			"node_tunnel_token":"node-token-2",
			"mattermost_url":"https://fleet-1.example",
			"ssh_hostname":"node-1.ssh.fleet-1.example",
			"tls_certificate_status":"active",
			"fleet_role":"active",
			"fleet_active_count":1,
			"fleet_pending_count":0,
			"fleet_quorum_size":1
		}`)),
		}, nil
	})}

	state := &setupFlowState{
		messenger:     newMsg("en"),
		configuration: config{APIBaseURL: "https://api.example", RegisterSecret: "register-secret"},
		stateDir:      stateDirectory,
	}

	errorValue := state.syncCloudflareAccess(&setup.Context{
		Callbacks: setup.Callbacks{
			SaveState: func(key string, value string) { saveState(stateDirectory, key, value) },
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	expectedState := map[string]string{
		"tunnel_token":           "tunnel-token-2",
		"node_tunnel_token":      "node-token-2",
		"device_url":             "https://fleet-1.example",
		"ssh_hostname":           "node-1.ssh.fleet-1.example",
		"tls_certificate_status": "active",
		"tunnel_origin":          setup.MattermostTunnelOrigin,
		"tunnel_revision":        setup.TunnelConfigurationRevision,
	}
	for key, expectedValue := range expectedState {
		if value := loadState(stateDirectory, key); value != expectedValue {
			t.Fatalf("state %s = %q, expected %q", key, value, expectedValue)
		}
	}
}

func TestSyncCloudflareAccessRequiresExistingLocalIdentity(t *testing.T) {
	state := &setupFlowState{
		messenger:     newMsg("en"),
		configuration: config{APIBaseURL: "https://api.example", RegisterSecret: "register-secret"},
		stateDir:      t.TempDir(),
	}

	errorValue := state.syncCloudflareAccess(&setup.Context{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "fleet_id, fleet_secret, node_id, and node_key") {
		t.Fatalf("expected local identity error, got %v", errorValue)
	}
}

func TestAdminWebDeployTargetsProductionPagesBranch(t *testing.T) {
	document, errorValue := os.ReadFile("setup_flow.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedCommand := `"bunx", "wrangler", "pages", "deploy", ".svelte-kit/cloudflare", "--project-name", "internkim", "--branch", "main"`
	if !strings.Contains(string(document), expectedCommand) {
		t.Fatalf("admin web deploy must target the production Pages branch")
	}
}

func TestRegisterAPIEnsuresMaintenanceBypassApplication(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "web", "src", "routes", "api", "register", "+server.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{
		"ensureMaintenanceBypassApplication",
		"ensurePublicBypassApplications",
	} {
		if !strings.Contains(string(document), expectedText) {
			t.Fatalf("register API must include %q", expectedText)
		}
	}
}

func TestCloudflareMaintenanceBypassApplicationCoversHealthAndRecovery(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "cloudflare.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{
		"intern kim maintenance",
		"/admin/api/health",
		"/admin/api/recovery/ssh-tunnel/restart",
		"maintenance-health-and-recovery",
	} {
		if !strings.Contains(string(document), expectedText) {
			t.Fatalf("Cloudflare maintenance bypass must include %q", expectedText)
		}
	}
}

func TestCloudflareWebSessionAccessApplicationCoversOnlyAuthCallback(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "cloudflare.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{
		"intern kim web session",
		"/auth/cloudflare/*",
		"Cloudflare web session access",
	} {
		if !strings.Contains(string(document), expectedText) {
			t.Fatalf("Cloudflare web session access must include %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{
		"${hostname}/flow*",
		"${hostname}/calendar*",
		"${hostname}/attendance*",
		"${hostname}/mail*",
		"${hostname}/memory*",
	} {
		if strings.Contains(string(document), forbiddenText) {
			t.Fatalf("Cloudflare web session access must not protect app pages with %q", forbiddenText)
		}
	}
}

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
