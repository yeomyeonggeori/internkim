package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvironmentFileSetsUnsetKeys(t *testing.T) {
	directoryPath := t.TempDir()
	filePath := filepath.Join(directoryPath, ".env")
	document := "# comment\nINTERNKIM_POC_SSH_PASSWORD=4321\nexport OTHER_KEY=\"quoted value\"\nBLANK=\n"
	if errorValue := os.WriteFile(filePath, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Setenv("INTERNKIM_POC_SSH_PASSWORD", "")
	t.Setenv("OTHER_KEY", "")

	loadEnvironmentFile(filePath)

	if got := os.Getenv("INTERNKIM_POC_SSH_PASSWORD"); got != "4321" {
		t.Fatalf("password = %q, want 4321", got)
	}
	if got := os.Getenv("OTHER_KEY"); got != "quoted value" {
		t.Fatalf("other key = %q, want quoted value", got)
	}
}

func TestLoadEnvironmentFileDoesNotOverrideExisting(t *testing.T) {
	directoryPath := t.TempDir()
	filePath := filepath.Join(directoryPath, ".env")
	if errorValue := os.WriteFile(filePath, []byte("INTERNKIM_POC_SSH_PASSWORD=fromfile\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Setenv("INTERNKIM_POC_SSH_PASSWORD", "fromenv")

	loadEnvironmentFile(filePath)

	if got := os.Getenv("INTERNKIM_POC_SSH_PASSWORD"); got != "fromenv" {
		t.Fatalf("password = %q, want fromenv (env must win)", got)
	}
}
