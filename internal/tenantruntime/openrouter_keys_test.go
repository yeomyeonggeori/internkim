package tenantruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPOpenRouterKeyProvisionerCreatesKey(t *testing.T) {
	managementKeyPath := filepath.Join(t.TempDir(), "management-key")
	if errorValue := os.WriteFile(managementKeyPath, []byte("management-secret"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var authorization string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatal(errorValue)
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"key":"sk-or-v1-created","data":{"hash":"hash-created","label":"label-created"}}`))
	}))
	defer server.Close()

	provisioner := HTTPOpenRouterKeyProvisioner{
		ManagementKeyPath: managementKeyPath,
		APIKeysURL:        server.URL,
	}
	key, errorValue := provisioner.CreateAPIKey(context.Background(), OpenRouterAPIKeyCreateRequest{
		Name:       "internkim-pilot-01",
		LimitUSD:   20,
		LimitReset: "monthly",
		ExpiresAt:  "2026-12-31T00:00:00Z",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if authorization != "Bearer management-secret" {
		t.Fatalf("expected management key authorization, got %q", authorization)
	}
	if payload["name"] != "internkim-pilot-01" || payload["limit"] != 20.0 || payload["limit_reset"] != "monthly" || payload["expires_at"] != "2026-12-31T00:00:00Z" {
		t.Fatalf("unexpected OpenRouter key create payload: %+v", payload)
	}
	if key.APIKey != "sk-or-v1-created" || key.Hash != "hash-created" || key.Label != "label-created" {
		t.Fatalf("unexpected provisioned OpenRouter key: %+v", key)
	}
}
