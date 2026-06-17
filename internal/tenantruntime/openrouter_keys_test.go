package tenantruntime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPOpenRouterKeyProvisionerCreatesKey(t *testing.T) {
	managementKeyPath := filepath.Join(t.TempDir(), "management-key")
	if errorValue := os.WriteFile(managementKeyPath, []byte("management-secret"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var authorization string
	var payload map[string]any
	httpClient := &http.Client{Transport: openRouterKeyTestTransport(func(request *http.Request) (*http.Response, error) {
		authorization = request.Header.Get("Authorization")
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatal(errorValue)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"key":"sk-or-v1-created","data":{"hash":"hash-created","label":"label-created"}}`)),
		}, nil
	})}

	provisioner := HTTPOpenRouterKeyProvisioner{
		ManagementKeyPath: managementKeyPath,
		APIKeysURL:        "https://openrouter.test/api/v1/keys",
		HTTPClient:        httpClient,
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

type openRouterKeyTestTransport func(request *http.Request) (*http.Response, error)

func (transport openRouterKeyTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}
