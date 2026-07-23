package blueclaw

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePayloadArtifactSourceRejectsStaleRevision(t *testing.T) {
	repositoryRootPath := t.TempDir()
	submodulePath := filepath.Join(repositoryRootPath, BlueclawSubmodulePath)
	writePayloadFile(t, filepath.Join(submodulePath, "migrations", "001.sql"), "select 1;")
	initGitRepositoryWithCommit(t, submodulePath)

	expectedManifest, errorValue := ExpectedPayloadArtifactSource(repositoryRootPath)
	if errorValue != nil {
		t.Fatalf("expected source manifest: %v", errorValue)
	}

	staleError := ValidatePayloadArtifactSource(repositoryRootPath, PayloadArtifactManifest{
		BlueclawRevision: "stale-revision",
		MigrationsSHA256: expectedManifest.MigrationsSHA256,
	})
	if staleError == nil || !strings.Contains(staleError.Error(), "prepare-blueclaw-payload") {
		t.Fatalf("expected stale revision to be rejected with rebuild guidance, got %v", staleError)
	}

	if freshError := ValidatePayloadArtifactSource(repositoryRootPath, expectedManifest); freshError != nil {
		t.Fatalf("expected matching revision to validate, got %v", freshError)
	}
}

func initGitRepositoryWithCommit(t *testing.T, repositoryPath string) {
	t.Helper()
	for _, arguments := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = repositoryPath
		if output, errorValue := command.CombinedOutput(); errorValue != nil {
			t.Fatalf("git %v: %v %s", arguments, errorValue, output)
		}
	}
}

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

func TestPayloadMigrationsHashIgnoresAppleMetadataFiles(t *testing.T) {
	directoryPath := t.TempDir()
	writePayloadFile(t, filepath.Join(directoryPath, "001.sql"), "select 1;")

	beforeSHA256, errorValue := calculateDirectorySHA256(directoryPath)
	if errorValue != nil {
		t.Fatalf("expected migrations hash: %v", errorValue)
	}

	writePayloadFile(t, filepath.Join(directoryPath, "._001.sql"), "appledouble")
	writePayloadFile(t, filepath.Join(directoryPath, ".DS_Store"), "metadata")

	afterSHA256, errorValue := calculateDirectorySHA256(directoryPath)
	if errorValue != nil {
		t.Fatalf("expected migrations hash: %v", errorValue)
	}
	if afterSHA256 != beforeSHA256 {
		t.Fatalf("expected Apple metadata files to be ignored")
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
