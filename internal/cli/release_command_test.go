package cli

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

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
