package admind

import (
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
