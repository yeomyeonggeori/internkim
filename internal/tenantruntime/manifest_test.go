package tenantruntime

import (
	"testing"
	"time"
)

func TestCloudSharedManifestUsesSystemdNspawnAndBackupMirror(t *testing.T) {
	manifest, errorValue := NewCloudSharedManifest("acme", "Acme", "mac-a", "https://acme.example.test", "mac-b")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := manifest.Validate(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if manifest.Profile != ProfileCloudShared {
		t.Fatalf("expected cloud-shared profile, got %q", manifest.Profile)
	}
	if manifest.ContainerRuntime != ContainerRuntimeSystemdNspawn {
		t.Fatalf("expected systemd-nspawn runtime, got %q", manifest.ContainerRuntime)
	}
	if manifest.ContainerName != "internkim-acme" {
		t.Fatalf("expected tenant container name, got %q", manifest.ContainerName)
	}
	if !manifest.BackupPolicy.IsEnabled || manifest.BackupPolicy.Interval != 6*time.Hour || manifest.BackupPolicy.MirrorHost != "mac-b" {
		t.Fatalf("unexpected backup policy: %+v", manifest.BackupPolicy)
	}
}

func TestEdgeApplianceManifestDoesNotRequireContainerRuntime(t *testing.T) {
	manifest, errorValue := NewEdgeApplianceManifest("dawn", "Dawn", "jetson-1", "https://dawn.example.test")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := manifest.Validate(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if manifest.Profile != ProfileEdgeAppliance {
		t.Fatalf("expected edge-appliance profile, got %q", manifest.Profile)
	}
	if manifest.ContainerRuntime != "" || manifest.ContainerName != "" {
		t.Fatalf("expected edge appliance to avoid tenant container runtime, got %+v", manifest)
	}
}

func TestTenantRuntimePathsDoNotOverlap(t *testing.T) {
	firstPaths, errorValue := BuildRuntimePaths("/srv/internkim/tenants", "acme")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondPaths, errorValue := BuildRuntimePaths("/srv/internkim/tenants", "dawn")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if RuntimePathsOverlap(firstPaths, secondPaths) {
		t.Fatalf("expected tenant runtime paths not to overlap: %+v %+v", firstPaths, secondPaths)
	}
	if firstPaths.InternKimSecretsPath == secondPaths.InternKimSecretsPath {
		t.Fatalf("expected tenant secret paths to be unique: %+v %+v", firstPaths, secondPaths)
	}
	if firstPaths.BlueclawWorkspacePath == secondPaths.BlueclawWorkspacePath {
		t.Fatalf("expected tenant workspace paths to be unique: %+v %+v", firstPaths, secondPaths)
	}
}

func TestTenantIDRejectsPathTraversal(t *testing.T) {
	if _, errorValue := BuildRuntimePaths("/srv/internkim/tenants", "../acme"); errorValue == nil {
		t.Fatal("expected path traversal tenant id to be rejected")
	}
	if _, errorValue := NewCloudSharedManifest("Acme", "Acme", "mac-a", "https://acme.example.test", "mac-b"); errorValue != nil {
		t.Fatalf("expected tenant id to normalize to lowercase, got %v", errorValue)
	}
}
