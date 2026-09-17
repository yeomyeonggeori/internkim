package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

func TestIdentityForSeedUsesDistinctCanonicalSubjects(t *testing.T) {
	seed := "company-seed"
	identity, errorValue := identityForSeed(seed)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	wantOwner, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	wantAgent := buzzidentity.Secret(seed, buzzidentity.AgentSubject)
	if identity.RelayOwnerPublicKey != wantOwner {
		t.Fatalf("relay owner public key = %q, want canonical bootstrap key %q", identity.RelayOwnerPublicKey, wantOwner)
	}
	if identity.AgentPrivateKey != wantAgent {
		t.Fatalf("agent private key = %q, want canonical agent key %q", identity.AgentPrivateKey, wantAgent)
	}
	if identity.AgentPrivateKey == buzzidentity.Secret(seed, buzzidentity.BootstrapSubject) {
		t.Fatal("agent identity must remain distinct from bootstrap identity")
	}
}

func TestWritePrivateIdentityRestrictsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if errorValue := os.WriteFile(path, []byte("old"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writePrivateIdentity(path, []byte(`{"agentPrivateKey":"agent"}`)); errorValue != nil {
		t.Fatal(errorValue)
	}
	fileInfo, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("identity mode = %o, want 600", mode)
	}
	var document map[string]string
	if errorValue := json.Unmarshal(mustReadFile(t, path), &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document["agentPrivateKey"] != "agent" {
		t.Fatalf("identity content = %v", document)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return content
}
