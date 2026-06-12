package tenantruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncCloudflareTenantTunnelRoutesAppPathsToTenantAdmind(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "cloudflare-token")
	if errorValue := os.WriteFile(tokenPath, []byte("cloudflare-token\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var updateRequest cloudflareTunnelConfigurationResource
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer cloudflare-token" {
			t.Fatalf("unexpected authorization header")
		}
		if request.URL.Path != "/accounts/account-1/cfd_tunnel/tunnel-1/configurations" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		switch request.Method {
		case http.MethodGet:
			writeCloudflareTunnelTestJSON(t, responseWriter, cloudflareTunnelConfigurationResponse{
				Success: true,
				Result: cloudflareTunnelConfigurationResource{Configuration: cloudflareTunnelConfiguration{Ingress: []cloudflareTunnelIngress{
					{Hostname: "pilot-01.example.test", Service: "http://127.0.0.1:18065"},
					{Hostname: "pilot-02.example.test", Service: "http://127.0.0.1:18066"},
					{Hostname: "pilot-01.mattermost.example.test", Path: "^/flow", Service: "http://127.0.0.1:18180"},
					{Service: "http_status:404"},
				}}},
			})
		case http.MethodPut:
			if errorValue := json.NewDecoder(request.Body).Decode(&updateRequest); errorValue != nil {
				t.Fatal(errorValue)
			}
			writeCloudflareTunnelTestJSON(t, responseWriter, cloudflareTunnelConfigurationResponse{Success: true})
		default:
			t.Fatalf("unexpected method: %s", request.Method)
		}
	}))
	defer server.Close()
	service := Service{BasePath: t.TempDir()}
	manifest := newTestCloudSharedManifest(t)
	manifest.TenantID = "pilot-01"
	manifest.PublicURL = "https://pilot-01.mattermost.example.test"
	manifest.MattermostInstance = MattermostInstance{PublicURL: "https://pilot-01.mattermost.example.test", InternalURL: "http://127.0.0.1:18065", Port: 18065}
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	status, errorValue := service.SyncCloudflareTenantTunnel(context.Background(), CloudflareTunnelSyncOptions{
		AccountID:              "account-1",
		TunnelID:               "tunnel-1",
		APITokenPath:           tokenPath,
		APIBaseURL:             server.URL,
		PublicHostnameTemplate: "{tenant}.example.test",
		TenantIDs:              []string{"pilot-01"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if status.IngressCount != 6 {
		t.Fatalf("unexpected status: %+v", status)
	}
	ingress := updateRequest.Configuration.Ingress
	if len(ingress) != 6 {
		t.Fatalf("unexpected ingress: %+v", ingress)
	}
	assertCloudflareIngress(t, ingress[0], "pilot-02.example.test", "", "http://127.0.0.1:18066")
	assertCloudflareIngress(t, ingress[1], "pilot-01.example.test", "/(admin|flow|memory|calendar|mail|attendance)(/.*)?", "http://127.0.0.1:18180")
	assertCloudflareIngress(t, ingress[2], "pilot-01.example.test", "/(auth|_app|_internkim)(/.*)?", "http://127.0.0.1:18180")
	assertCloudflareIngress(t, ingress[3], "pilot-01.example.test", "/(logo\\.svg|\\.well-known/caldav)", "http://127.0.0.1:18180")
	assertCloudflareIngress(t, ingress[4], "pilot-01.example.test", "", "http://127.0.0.1:18065")
	assertCloudflareIngress(t, ingress[5], "", "", "http_status:404")
}

func TestSyncCloudflareTenantTunnelRequiresToken(t *testing.T) {
	_, errorValue := (Service{BasePath: t.TempDir()}).SyncCloudflareTenantTunnel(context.Background(), CloudflareTunnelSyncOptions{
		AccountID: "account-1",
		TunnelID:  "tunnel-1",
		TenantIDs: []string{"pilot-01"},
	})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "token") {
		t.Fatalf("expected token validation error, got %v", errorValue)
	}
}

func TestRemovedCloudflareTenantIngressDeletesTenantHostnames(t *testing.T) {
	manifest := newTestCloudSharedManifest(t)
	manifest.TenantID = "pilot-01"
	manifest.PublicURL = "https://pilot-01.mattermost.example.test"

	ingress, removedCount := removedCloudflareTenantIngress([]cloudflareTunnelIngress{
		{Hostname: "pilot-01.example.test", Path: "/flow", Service: "http://127.0.0.1:18180"},
		{Hostname: "pilot-01.mattermost.example.test", Service: "http://127.0.0.1:18065"},
		{Hostname: "pilot-02.example.test", Service: "http://127.0.0.1:18066"},
		{Service: "http_status:404"},
	}, []Manifest{manifest}, "{tenant}.example.test")

	if removedCount != 2 || len(ingress) != 2 {
		t.Fatalf("unexpected ingress removal: count=%d ingress=%+v", removedCount, ingress)
	}
	assertCloudflareIngress(t, ingress[0], "pilot-02.example.test", "", "http://127.0.0.1:18066")
	assertCloudflareIngress(t, ingress[1], "", "", "http_status:404")
}

func assertCloudflareIngress(t *testing.T, ingress cloudflareTunnelIngress, hostname string, path string, service string) {
	t.Helper()
	if ingress.Hostname != hostname || ingress.Path != path || ingress.Service != service {
		t.Fatalf("unexpected ingress: %+v", ingress)
	}
}

func writeCloudflareTunnelTestJSON(t *testing.T, responseWriter http.ResponseWriter, value any) {
	t.Helper()
	responseWriter.Header().Set("Content-Type", "application/json")
	if errorValue := json.NewEncoder(responseWriter).Encode(value); errorValue != nil {
		t.Fatal(errorValue)
	}
}
