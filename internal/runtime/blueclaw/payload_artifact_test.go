package blueclaw

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePayloadArtifactDirectoryChecksBinaryAndMigrations(t *testing.T) {
	artifactDirectoryPath := t.TempDir()
	binaryPath := filepath.Join(artifactDirectoryPath, "workspace", ".blueclaw", "runtime", "current", "bin", "blueclaw")
	migrationPath := filepath.Join(artifactDirectoryPath, "workspace", ".blueclaw", "runtime", "current", "migrations", "001.sql")
	writePayloadFile(t, binaryPath, "blueclaw")
	writePayloadFile(t, migrationPath, "select 1;")

	binarySHA256, errorValue := calculateFileSHA256(binaryPath)
	if errorValue != nil {
		t.Fatalf("expected binary hash: %v", errorValue)
	}
	migrationsSHA256, errorValue := calculateDirectorySHA256(filepath.Dir(migrationPath))
	if errorValue != nil {
		t.Fatalf("expected migrations hash: %v", errorValue)
	}
	manifestDocument := `{
  "runtimeName": "internkim-blueclaw-payload",
  "platform": "linux-arm64",
  "blueclawRevision": "test",
  "blueclawSHA256": "` + binarySHA256 + `",
  "migrationsSHA256": "` + migrationsSHA256 + `"
}
`
	writePayloadFile(t, filepath.Join(artifactDirectoryPath, "manifest.json"), manifestDocument)

	manifest, errorValue := ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		t.Fatalf("expected payload to validate: %v", errorValue)
	}
	if manifest.BlueclawRevision != "test" {
		t.Fatalf("expected revision to match, got %q", manifest.BlueclawRevision)
	}
}

func TestValidatePayloadArtifactDirectoryRejectsBinaryMismatch(t *testing.T) {
	artifactDirectoryPath := t.TempDir()
	writePayloadFile(t, filepath.Join(artifactDirectoryPath, "workspace", ".blueclaw", "runtime", "current", "bin", "blueclaw"), "blueclaw")
	writePayloadFile(t, filepath.Join(artifactDirectoryPath, "workspace", ".blueclaw", "runtime", "current", "migrations", "001.sql"), "select 1;")
	writePayloadFile(t, filepath.Join(artifactDirectoryPath, "manifest.json"), `{
  "runtimeName": "internkim-blueclaw-payload",
  "platform": "linux-arm64",
  "blueclawRevision": "test",
  "blueclawSHA256": "bad",
  "migrationsSHA256": "bad"
}
`)

	_, errorValue := ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue == nil {
		t.Fatal("expected payload mismatch to fail")
	}
}

func writePayloadFile(t *testing.T, filePath string, content string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(filePath), 0o755); errorValue != nil {
		t.Fatalf("expected parent directory: %v", errorValue)
	}
	if errorValue := os.WriteFile(filePath, []byte(content), 0o644); errorValue != nil {
		t.Fatalf("expected payload file: %v", errorValue)
	}
}
