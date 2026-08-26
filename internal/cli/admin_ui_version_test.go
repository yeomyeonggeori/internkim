package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheWebRevisionIsAVersionRatherThanTheDocumentCarryingIt(t *testing.T) {
	boardUIPath := t.TempDir()
	if errorValue := os.MkdirAll(filepath.Join(boardUIPath, "_app"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	stamp := []byte("{\n  \"version\": \"1787716413000\"\n}\n")
	if errorValue := os.WriteFile(filepath.Join(boardUIPath, "_app", "version.json"), stamp, 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	version, errorValue := readAdminUIVersion(boardUIPath)
	if errorValue != nil {
		t.Fatalf("a stamp svelte wrote must read: %v", errorValue)
	}
	if version != "1787716413000" {
		t.Fatalf("a revision truncated to twelve characters would read as %q, got %q", "{\"version\":\"", version)
	}
}

func TestAVersionStampNamingNoVersionIsRefused(t *testing.T) {
	if _, errorValue := adminUIVersionOf([]byte(`{"built":"today"}`)); errorValue == nil {
		t.Fatal("a stamp with no version was accepted, so the release would carry an empty revision")
	}
	if _, errorValue := adminUIVersionOf([]byte("not json")); errorValue == nil {
		t.Fatal("a document that is not a version stamp was accepted")
	}
}
