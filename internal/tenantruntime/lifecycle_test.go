package tenantruntime

import (
	"os"
	"testing"
)

func TestCreateTenantWritesManifestAndRuntimeDirectories(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	manifest, errorValue := NewCloudSharedManifest("acme", "Acme", "mac-a", "https://acme.example.test", "mac-b")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	status, errorValue := service.CreateTenant(manifest)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if status.Manifest.TenantID != "acme" || status.Manifest.ContainerRuntime != ContainerRuntimeSystemdNspawn {
		t.Fatalf("unexpected tenant status manifest: %+v", status.Manifest)
	}
	if !status.ContainerRootReady || !status.SecretsReady || !status.WorkspaceReady || !status.BackupReady {
		t.Fatalf("expected runtime paths to be ready, got %+v", status)
	}
	assertDirectoryMode(t, status.ManifestPath, 0o600)
	paths, errorValue := BuildRuntimePaths(service.BasePath, "acme")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertDirectoryMode(t, paths.InternKimSecretsPath, 0o700)
	assertDirectoryMode(t, paths.BackupPath, 0o700)
	assertDirectoryMode(t, paths.BlueclawWorkspacePath, 0o750)
}

func TestReadManifestValidatesStoredTenant(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	manifest, errorValue := NewCloudSharedManifest("acme", "Acme", "mac-a", "https://acme.example.test", "mac-b")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.CreateTenant(manifest); errorValue != nil {
		t.Fatal(errorValue)
	}

	storedManifest, errorValue := service.ReadManifest("acme")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if storedManifest.TenantID != manifest.TenantID || storedManifest.BackupPolicy.MirrorHost != "mac-b" {
		t.Fatalf("unexpected stored manifest: %+v", storedManifest)
	}
}

func TestCreateTenantRejectsInvalidManifest(t *testing.T) {
	service := Service{BasePath: t.TempDir()}
	_, errorValue := service.CreateTenant(Manifest{TenantID: "../bad", Profile: ProfileCloudShared})
	if errorValue == nil {
		t.Fatal("expected invalid tenant manifest to be rejected")
	}
}

func assertDirectoryMode(t *testing.T, path string, expectedMode os.FileMode) {
	t.Helper()
	info, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if info.Mode().Perm() != expectedMode {
		t.Fatalf("expected %s mode %o, got %o", path, expectedMode, info.Mode().Perm())
	}
}
