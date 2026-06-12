package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

type testReleasePublisher struct {
	deletedObjectKeys []string
	objects           map[string][]byte
	publicBaseURL     string
}

func TestReleasePublisherFromEnvironmentUsesWrangler(t *testing.T) {
	repositoryRootPath := t.TempDir()
	wranglerPath := filepath.Join(repositoryRootPath, "web", "node_modules", ".bin")
	if errorValue := os.MkdirAll(wranglerPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	localWranglerPath := filepath.Join(wranglerPath, "wrangler")
	if errorValue := os.WriteFile(localWranglerPath, []byte("#!/bin/sh\n"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Setenv("INTERNKIM_RELEASE_R2_PUBLISHER", "wrangler")
	t.Setenv("INTERNKIM_RELEASE_R2_ACCOUNT_ID", "account-1")
	t.Setenv("INTERNKIM_RELEASE_R2_BUCKET", "internkim-releases")
	t.Setenv("INTERNKIM_RELEASE_PUBLIC_BASE_URL", "https://updates.intern.kim")

	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	wranglerPublisher, ok := publisher.(wranglerReleasePublisher)
	if !ok {
		t.Fatalf("publisher = %T", publisher)
	}
	if wranglerPublisher.commandPath != localWranglerPath {
		t.Fatalf("command path = %q", wranglerPublisher.commandPath)
	}
}

func TestWranglerObjectPutArgumentsUseRemoteR2(t *testing.T) {
	arguments := wranglerObjectPutArguments("internkim-releases", "channels/stable.json", "/tmp/stable.json", "application/json")
	expectedArguments := []string{
		"r2", "object", "put", "internkim-releases/channels/stable.json",
		"--file", "/tmp/stable.json",
		"--force",
		"--remote",
		"--content-type", "application/json",
	}
	if !equalStrings(arguments, expectedArguments) {
		t.Fatalf("arguments = %#v", arguments)
	}
}

func TestWranglerObjectDeleteArgumentsUseRemoteR2(t *testing.T) {
	arguments := wranglerObjectDeleteArguments("internkim-releases", "releases/release-1/manifest.json")
	expectedArguments := []string{
		"r2", "object", "delete", "internkim-releases/releases/release-1/manifest.json",
		"--remote",
	}
	if !equalStrings(arguments, expectedArguments) {
		t.Fatalf("arguments = %#v", arguments)
	}
}

func TestWranglerObjectPutEnvironmentUsesOAuthSession(t *testing.T) {
	environment := wranglerObjectPutEnvironment([]string{
		"CF_API_TOKEN=bad",
		"CLOUDFLARE_API_TOKEN=bad",
		"CF_ACCOUNT_ID=old",
		"PATH=/bin",
	}, "account-1")
	expectedEnvironment := []string{"PATH=/bin", "CLOUDFLARE_ACCOUNT_ID=account-1"}
	if !equalStrings(environment, expectedEnvironment) {
		t.Fatalf("environment = %#v", environment)
	}
}

func TestBuildReleaseChannelHistoryTrimsNewestFirst(t *testing.T) {
	updatedAt := time.Date(2026, 6, 12, 1, 2, 3, 0, time.UTC)
	existingHistory := releaseset.ChannelHistory{
		Channel: "stable",
		Entries: []releaseset.ChannelHistoryEntry{
			{ReleaseID: "release-2", ManifestURL: "https://updates.test/releases/release-2/manifest.json", CreatedAt: "2026-06-11T00:00:00Z"},
			{ReleaseID: "release-1", ManifestURL: "https://updates.test/releases/release-1/manifest.json", CreatedAt: "2026-06-10T00:00:00Z"},
		},
	}
	history, prunedEntries := buildReleaseChannelHistory(existingHistory, "stable", releaseset.ChannelHistoryEntry{
		ReleaseID:   "release-3",
		ManifestURL: "https://updates.test/releases/release-3/manifest.json",
		CreatedAt:   "2026-06-12T00:00:00Z",
	}, 2, updatedAt)

	if len(history.Entries) != 2 || history.Entries[0].ReleaseID != "release-3" || history.Entries[1].ReleaseID != "release-2" {
		t.Fatalf("history entries = %+v", history.Entries)
	}
	if len(prunedEntries) != 1 || prunedEntries[0].ReleaseID != "release-1" {
		t.Fatalf("pruned entries = %+v", prunedEntries)
	}
	if history.UpdatedAt != "2026-06-12T01:02:03Z" {
		t.Fatalf("updated at = %q", history.UpdatedAt)
	}
}

func TestPruneReleaseChannelHistoryKeepsSharedBlob(t *testing.T) {
	sharedBlobSHA256 := strings.Repeat("a", 64)
	prunedBlobSHA256 := strings.Repeat("b", 64)
	retainedBlobSHA256 := strings.Repeat("c", 64)
	currentManifest := testReleaseManifestWithBlobs("release-2", sharedBlobSHA256, retainedBlobSHA256)
	prunedManifest := testReleaseManifestWithBlobs("release-1", sharedBlobSHA256, prunedBlobSHA256)
	originalStatusHTTPClient := statusHTTPClient
	defer func() { statusHTTPClient = originalStatusHTTPClient }()
	statusHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/releases/release-1/manifest.json" {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		document, errorValue := json.Marshal(prunedManifest)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(document)), Header: make(http.Header)}, nil
	})}
	publisher := &testReleasePublisher{}

	warnings := pruneReleaseChannelHistory(
		publisher,
		[]releaseset.ChannelHistoryEntry{{ReleaseID: "release-2", ManifestURL: "https://updates.test/releases/release-2/manifest.json"}},
		[]releaseset.ChannelHistoryEntry{{ReleaseID: "release-1", ManifestURL: "https://updates.test/releases/release-1/manifest.json"}},
		&currentManifest,
	)

	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if !containsDeletedObjectKey(publisher.deletedObjectKeys, "releases/release-1/manifest.json") {
		t.Fatalf("deleted keys = %+v", publisher.deletedObjectKeys)
	}
	if !containsDeletedObjectKey(publisher.deletedObjectKeys, "blobs/sha256/"+prunedBlobSHA256) {
		t.Fatalf("deleted keys = %+v", publisher.deletedObjectKeys)
	}
	if containsDeletedObjectKey(publisher.deletedObjectKeys, "blobs/sha256/"+sharedBlobSHA256) {
		t.Fatalf("shared blob was deleted: %+v", publisher.deletedObjectKeys)
	}
}

func TestAddReleaseDownloadHeadersUsesEnvironmentToken(t *testing.T) {
	t.Setenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN", "download-token")
	request, errorValue := http.NewRequest(http.MethodGet, "https://updates.intern.kim/channels/stable.json", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	addReleaseDownloadHeaders(request)

	if request.Header.Get("X-InternKim-Release-Token") != "download-token" {
		t.Fatalf("release token header = %q", request.Header.Get("X-InternKim-Release-Token"))
	}
}

func testReleaseManifestWithBlobs(releaseID string, blobSHA256Values ...string) releaseset.Manifest {
	components := map[string]releaseset.Component{}
	for index, blobSHA256Value := range blobSHA256Values {
		componentName := "component-" + string(rune('a'+index))
		components[componentName] = releaseset.Component{
			Name:         componentName,
			Revision:     releaseID,
			SHA256:       blobSHA256Value,
			Size:         1,
			BlobPath:     "blobs/sha256/" + blobSHA256Value,
			RestartGroup: "admind",
			HealthCheck:  "binary",
		}
	}
	return releaseset.NewManifest(releaseID, "stable", components)
}

func (publisher *testReleasePublisher) PutObject(objectKey string, document []byte, _ string) error {
	if publisher.objects == nil {
		publisher.objects = map[string][]byte{}
	}
	publisher.objects[objectKey] = document
	return nil
}

func (publisher *testReleasePublisher) DeleteObject(objectKey string) error {
	publisher.deletedObjectKeys = append(publisher.deletedObjectKeys, objectKey)
	return nil
}

func (publisher *testReleasePublisher) PublicURL(objectKey string) string {
	return strings.TrimRight(publisher.publicBaseURL, "/") + "/" + strings.TrimLeft(objectKey, "/")
}

func containsDeletedObjectKey(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestWriteReleaseArchiveFollowsRootDirectorySymlink(t *testing.T) {
	rootPath := t.TempDir()
	sourcePath := filepath.Join(rootPath, "source")
	if errorValue := os.MkdirAll(sourcePath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(sourcePath, "manifest.json"), []byte("{}"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	symlinkPath := filepath.Join(rootPath, "payload")
	if errorValue := os.Symlink(sourcePath, symlinkPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	archivePath := filepath.Join(rootPath, "payload.tar.gz")

	if errorValue := writeReleaseArchive(symlinkPath, archivePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !releaseArchiveContains(t, archivePath, "source/manifest.json") {
		t.Fatal("expected archive to contain manifest from symlink target directory")
	}
}

func releaseArchiveContains(t *testing.T, archivePath string, targetName string) bool {
	t.Helper()
	file, errorValue := os.Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer file.Close()
	gzipReader, errorValue := gzip.NewReader(file)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, errorValue := tarReader.Next()
		if errorValue == io.EOF {
			return false
		}
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if header.Name == targetName {
			return true
		}
	}
}
