package admind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublicBlueclawUpdateMetadataHidesArchivePath(t *testing.T) {
	metadata := &blueclawUpdateArtifactMetadata{
		Component:        "blueclaw",
		Version:          "version-1",
		BlueclawRevision: "revision-1",
		ArchivePath:      "/root/private/payload.tar.gz",
	}

	publicMetadata := publicBlueclawUpdateMetadata(metadata)

	if publicMetadata.ArchivePath != "" {
		t.Fatalf("archive path was exposed: %q", publicMetadata.ArchivePath)
	}
	if metadata.ArchivePath == "" {
		t.Fatal("source metadata was mutated")
	}
}

func TestWriteLimitedRequestBodyRejectsOversizedInput(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "chunk")

	errorValue := writeLimitedRequestBody(targetPath, strings.NewReader("12345"), 4)

	if errorValue == nil {
		t.Fatal("expected oversized request body to fail")
	}
}

func TestValidateBlueclawUpdateUploadRequest(t *testing.T) {
	payload := blueclawUpdateUploadCreateRequest{
		fleetSignedRequest: fleetSignedRequest{
			Action:    "blueclaw-update-upload",
			DeviceID:  "device-1",
			Nonce:     "nonce-1",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		Version:  "revision-1",
		Filename: "payload.tar.gz",
		Size:     10,
		SHA256:   strings.Repeat("a", 64),
	}

	if errorValue := validateBlueclawUpdateUploadRequest(payload); errorValue != nil {
		t.Fatalf("valid upload request failed: %v", errorValue)
	}

	payload.SHA256 = "bad"
	if errorValue := validateBlueclawUpdateUploadRequest(payload); errorValue == nil {
		t.Fatal("invalid sha256 was accepted")
	}

	payload.SHA256 = strings.Repeat("a", 64)
	payload.Size = 0
	if errorValue := validateBlueclawUpdateUploadRequest(payload); errorValue == nil {
		t.Fatal("missing size was accepted")
	}
}

func TestPersistedBlueclawUpdateMetadataKeepsArchivePath(t *testing.T) {
	stateDirectory := t.TempDir()
	path := filepath.Join(stateDirectory, "metadata.json")
	metadata := &blueclawUpdateArtifactMetadata{
		Component:        "blueclaw",
		Version:          "version-1",
		BlueclawRevision: "revision-1",
		ArchivePath:      "/root/private/payload.tar.gz",
	}

	if errorValue := writeBlueclawUpdateMetadata(path, metadata); errorValue != nil {
		t.Fatal(errorValue)
	}

	readMetadata := readBlueclawUpdateMetadata(path)
	if readMetadata == nil || readMetadata.ArchivePath != metadata.ArchivePath {
		document, _ := os.ReadFile(path)
		t.Fatalf("archive path was not persisted: %s", string(document))
	}
}

func TestBlueclawPayloadTenantInstallTargetsReadRuntimeConfiguration(t *testing.T) {
	tenantBasePath := t.TempDir()
	runtimeConfigurationPath := filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "config", "runtime.json")
	if errorValue := os.MkdirAll(filepath.Dir(runtimeConfigurationPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	runtimeConfiguration := blueclawPayloadRuntimeConfiguration{}
	runtimeConfiguration.Firecracker.HostWorkspacePath = filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "workspace")
	runtimeConfiguration.Firecracker.WorkspaceImagePath = filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "firecracker", "workspace.ext4")
	document, errorValue := json.Marshal(runtimeConfiguration)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(runtimeConfigurationPath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	targets := blueclawPayloadTenantInstallTargets(tenantBasePath)

	if len(targets) != 1 {
		t.Fatalf("expected one tenant target, got %+v", targets)
	}
	target := targets[0]
	if target.Name != "pilot-01" || target.ServiceName != "internkim-tenant-blueclaw-pilot-01.service" {
		t.Fatalf("unexpected tenant target identity: %+v", target)
	}
	if target.HostWorkspacePath != runtimeConfiguration.Firecracker.HostWorkspacePath {
		t.Fatalf("unexpected host workspace path: %+v", target)
	}
	if target.WorkspaceImagePath != runtimeConfiguration.Firecracker.WorkspaceImagePath {
		t.Fatalf("unexpected workspace image path: %+v", target)
	}
	if target.PayloadManifestPath != filepath.Join(tenantBasePath, "pilot-01", "blueclaw", "payload-manifest.json") {
		t.Fatalf("unexpected payload manifest path: %+v", target)
	}
}

func TestHostWorkspacePayloadSyncCommandUsesTenantTarget(t *testing.T) {
	target := blueclawPayloadInstallTarget{
		HostWorkspacePath: "/srv/internkim/tenants/pilot-01/blueclaw/workspace",
	}

	command := hostWorkspacePayloadSyncCommandForTarget("/tmp/payload", target)

	for _, expectedText := range []string{
		"/tmp/payload/workspace/.blueclaw/runtime/",
		"/srv/internkim/tenants/pilot-01/blueclaw/workspace/.blueclaw/runtime/",
		"rsync -a --delete",
		"chown -R blueclaw:blueclaw",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected tenant sync command to include %q, got:\n%s", expectedText, command)
		}
	}
	if strings.Contains(command, "/root/.blueclaw/workspace") {
		t.Fatalf("tenant sync command must not use canonical workspace, got:\n%s", command)
	}
}
