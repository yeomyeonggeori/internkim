package deployops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloudflareAccessServiceTokenReadsTheHeldFile(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token.json")
	if errorValue := os.WriteFile(tokenPath, []byte(`{"clientID":" id.access ","clientSecret":" secret "}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	clientID, clientSecret := cloudflareAccessServiceToken(tokenPath)
	if clientID != "id.access" || clientSecret != "secret" {
		t.Fatalf("expected the held credentials trimmed, got %q %q", clientID, clientSecret)
	}
}

func TestCloudflareAccessServiceTokenAbsentFileYieldsNothing(t *testing.T) {
	clientID, clientSecret := cloudflareAccessServiceToken(filepath.Join(t.TempDir(), "missing.json"))
	if clientID != "" || clientSecret != "" {
		t.Fatal("a missing file must yield no credential, so the login-token path serves instead")
	}
}
