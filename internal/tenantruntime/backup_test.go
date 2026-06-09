package tenantruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedBackupRestoresTenantOnDifferentBasePath(t *testing.T) {
	sourceService := Service{BasePath: filepath.Join(t.TempDir(), "source")}
	targetService := Service{BasePath: filepath.Join(t.TempDir(), "target")}
	manifest, errorValue := NewCloudSharedManifest("acme", "Acme", "mac-a", "https://acme.intern.kim", "mac-b")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := sourceService.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	sourcePaths, errorValue := BuildRuntimePaths(sourceService.BasePath, "acme")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sourceDocumentPath := filepath.Join(sourcePaths.BlueclawWorkspacePath, "artifact.txt")
	if errorValue := os.WriteFile(sourceDocumentPath, []byte("tenant artifact"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	bundlePath := filepath.Join(t.TempDir(), "acme.iktbak")

	if errorValue := sourceService.CreateEncryptedBackup("acme", bundlePath, "passphrase"); errorValue != nil {
		t.Fatal(errorValue)
	}
	status, errorValue := targetService.RestoreEncryptedBackup(bundlePath, "passphrase")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if status.Manifest.TenantID != "acme" || !status.WorkspaceReady || !status.SecretsReady || !status.BackupReady {
		t.Fatalf("unexpected restored tenant status: %+v", status)
	}
	targetPaths, errorValue := BuildRuntimePaths(targetService.BasePath, "acme")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := os.ReadFile(filepath.Join(targetPaths.BlueclawWorkspacePath, "artifact.txt"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != "tenant artifact" {
		t.Fatalf("unexpected restored artifact: %q", string(document))
	}
}

func TestEncryptedBackupRejectsWrongPassphrase(t *testing.T) {
	sourceService := Service{BasePath: filepath.Join(t.TempDir(), "source")}
	manifest, errorValue := NewCloudSharedManifest("acme", "Acme", "mac-a", "https://acme.intern.kim", "mac-b")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := sourceService.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}
	bundlePath := filepath.Join(t.TempDir(), "acme.iktbak")
	if errorValue := sourceService.CreateEncryptedBackup("acme", bundlePath, "passphrase"); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue = (Service{BasePath: filepath.Join(t.TempDir(), "target")}).RestoreEncryptedBackup(bundlePath, "wrong")

	if errorValue == nil {
		t.Fatal("expected wrong passphrase to fail")
	}
}
