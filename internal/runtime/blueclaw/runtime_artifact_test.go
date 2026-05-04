package blueclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateRuntimeArtifactDirectoryRequiresManifestAndChecksums(t *testing.T) {
	artifactDirectoryPath := t.TempDir()
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "firecracker", "firecracker")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "jailer", "jailer")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "vmlinux.bin", "kernel")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "rootfs.ext4", "rootfs")

	manifestDocument := `{
  "runtimeName": "internkim-blueclaw-runtime",
  "platform": "linux-arm64",
  "version": "test",
  "files": [
    {"name": "firecracker", "path": "firecracker", "sha256": "` + runtimeArtifactTestSHA256("firecracker") + `", "mode": "0755"},
    {"name": "jailer", "path": "jailer", "sha256": "` + runtimeArtifactTestSHA256("jailer") + `", "mode": "0755"},
    {"name": "vmlinux.bin", "path": "vmlinux.bin", "sha256": "` + runtimeArtifactTestSHA256("kernel") + `", "mode": "0644"},
    {"name": "rootfs.ext4", "path": "rootfs.ext4", "sha256": "` + runtimeArtifactTestSHA256("rootfs") + `", "mode": "0644"}
  ]
}
`
	if errorValue := os.WriteFile(filepath.Join(artifactDirectoryPath, "manifest.json"), []byte(manifestDocument), 0o644); errorValue != nil {
		t.Fatalf("expected manifest to be written: %v", errorValue)
	}

	manifest, errorValue := ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		t.Fatalf("expected artifact directory to validate: %v", errorValue)
	}
	if manifest.Platform != "linux-arm64" {
		t.Fatalf("expected platform to match, got %q", manifest.Platform)
	}
}

func TestValidateRuntimeArtifactDirectoryRejectsChecksumMismatch(t *testing.T) {
	artifactDirectoryPath := t.TempDir()
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "firecracker", "firecracker")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "jailer", "jailer")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "vmlinux.bin", "kernel")
	writeRuntimeArtifactFile(t, artifactDirectoryPath, "rootfs.ext4", "rootfs")

	manifestDocument := `{
  "runtimeName": "internkim-blueclaw-runtime",
  "platform": "linux-arm64",
  "version": "test",
  "files": [
    {"name": "firecracker", "path": "firecracker", "sha256": "bad", "mode": "0755"},
    {"name": "jailer", "path": "jailer", "sha256": "` + runtimeArtifactTestSHA256("jailer") + `", "mode": "0755"},
    {"name": "vmlinux.bin", "path": "vmlinux.bin", "sha256": "` + runtimeArtifactTestSHA256("kernel") + `", "mode": "0644"},
    {"name": "rootfs.ext4", "path": "rootfs.ext4", "sha256": "` + runtimeArtifactTestSHA256("rootfs") + `", "mode": "0644"}
  ]
}
`
	if errorValue := os.WriteFile(filepath.Join(artifactDirectoryPath, "manifest.json"), []byte(manifestDocument), 0o644); errorValue != nil {
		t.Fatalf("expected manifest to be written: %v", errorValue)
	}

	_, errorValue := ValidateRuntimeArtifactDirectory(artifactDirectoryPath)
	if errorValue == nil {
		t.Fatal("expected checksum mismatch to fail")
	}
}

func TestValidateRuntimeArtifactSourceAcceptsBaseMetadataWithoutBlueclawRevision(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	expectedManifest, errorValue := ExpectedRuntimeArtifactSource(repositoryRootPath)
	if errorValue != nil {
		t.Fatalf("expected source metadata: %v", errorValue)
	}
	manifest := RuntimeArtifactManifest{
		RuntimeName:         "internkim-blueclaw-runtime",
		Platform:            "linux-arm64",
		Version:             "test",
		GuestInitSHA256:     expectedManifest.GuestInitSHA256,
		PrepareScriptSHA256: expectedManifest.PrepareScriptSHA256,
		BaseSourceSHA256:    expectedManifest.BaseSourceSHA256,
	}
	if errorValue := ValidateRuntimeArtifactSource(repositoryRootPath, manifest); errorValue != nil {
		t.Fatalf("expected source metadata to validate: %v", errorValue)
	}
}

func writeRuntimeArtifactFile(t *testing.T, artifactDirectoryPath string, fileName string, content string) {
	t.Helper()
	if errorValue := os.WriteFile(filepath.Join(artifactDirectoryPath, fileName), []byte(content), 0o644); errorValue != nil {
		t.Fatalf("expected artifact file to be written: %v", errorValue)
	}
}

func runtimeArtifactTestSHA256(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func runtimeArtifactRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filePath, _, isOK := runtime.Caller(0)
	if !isOK {
		t.Fatal("expected caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filePath), "..", "..", ".."))
}
