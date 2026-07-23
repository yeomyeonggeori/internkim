package cli

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func validGzipBytes(t *testing.T, payload string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	if _, errorValue := gzipWriter.Write([]byte(payload)); errorValue != nil {
		t.Fatalf("write gzip payload: %v", errorValue)
	}
	if errorValue := gzipWriter.Close(); errorValue != nil {
		t.Fatalf("close gzip writer: %v", errorValue)
	}
	return buffer.Bytes()
}

func TestResolveMattermostVersionDefaultsWhenEnvironmentVariableUnset(t *testing.T) {
	t.Setenv(mattermostVersionEnvironmentVariable, "")

	resolvedVersion := resolveMattermostVersion()

	if resolvedVersion != defaultMattermostVersion {
		t.Fatalf("expected default version %q, got %q", defaultMattermostVersion, resolvedVersion)
	}
}

func TestResolveMattermostVersionUsesEnvironmentOverride(t *testing.T) {
	t.Setenv(mattermostVersionEnvironmentVariable, "12.0.0")

	resolvedVersion := resolveMattermostVersion()

	if resolvedVersion != "12.0.0" {
		t.Fatalf("expected overridden version %q, got %q", "12.0.0", resolvedVersion)
	}
}

func TestResolveMattermostVersionTrimsWhitespaceInOverride(t *testing.T) {
	t.Setenv(mattermostVersionEnvironmentVariable, "  9.5.0  ")

	resolvedVersion := resolveMattermostVersion()

	if resolvedVersion != "9.5.0" {
		t.Fatalf("expected trimmed version %q, got %q", "9.5.0", resolvedVersion)
	}
}

func TestMattermostDownloadURLBuildsExpectedReleaseAddress(t *testing.T) {
	downloadURL := mattermostDownloadURL("11.9.0")

	expectedURL := "https://releases.mattermost.com/11.9.0/mattermost-11.9.0-linux-arm64.tar.gz"
	if downloadURL != expectedURL {
		t.Fatalf("expected %q, got %q", expectedURL, downloadURL)
	}
}

func TestMattermostTarballCachePathsUsesVersionedFileNames(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	tarballPath, checksumPath, errorValue := mattermostTarballCachePaths("11.9.0")
	if errorValue != nil {
		t.Fatalf("unexpected error: %v", errorValue)
	}

	expectedTarballPath := filepath.Join(homeDirectory, ".cache", "internkim", "mattermost", "mattermost-11.9.0-linux-arm64.tar.gz")
	if tarballPath != expectedTarballPath {
		t.Fatalf("expected tarball path %q, got %q", expectedTarballPath, tarballPath)
	}
	if checksumPath != expectedTarballPath+".sha256" {
		t.Fatalf("expected checksum path %q, got %q", expectedTarballPath+".sha256", checksumPath)
	}
}

func TestIsValidGzipTarballAcceptsWellFormedGzipData(t *testing.T) {
	tarballPath := filepath.Join(t.TempDir(), "mattermost.tar.gz")
	if errorValue := os.WriteFile(tarballPath, validGzipBytes(t, "mattermost release payload"), 0o644); errorValue != nil {
		t.Fatalf("write test tarball: %v", errorValue)
	}

	if !isValidGzipTarball(tarballPath) {
		t.Fatalf("expected valid gzip tarball to pass validation")
	}
}

func TestIsValidGzipTarballRejectsCorruptedData(t *testing.T) {
	tarballPath := filepath.Join(t.TempDir(), "mattermost.tar.gz")
	if errorValue := os.WriteFile(tarballPath, []byte("not a gzip file"), 0o644); errorValue != nil {
		t.Fatalf("write test tarball: %v", errorValue)
	}

	if isValidGzipTarball(tarballPath) {
		t.Fatalf("expected corrupted data to fail gzip validation")
	}
}

func TestIsValidGzipTarballRejectsMissingFile(t *testing.T) {
	if isValidGzipTarball(filepath.Join(t.TempDir(), "missing.tar.gz")) {
		t.Fatalf("expected missing file to fail gzip validation")
	}
}

func TestIsMattermostTarballCacheValidAcceptsMatchingChecksum(t *testing.T) {
	directory := t.TempDir()
	tarballPath := filepath.Join(directory, "mattermost.tar.gz")
	checksumPath := tarballPath + ".sha256"
	tarballData := validGzipBytes(t, "cached payload")
	if errorValue := os.WriteFile(tarballPath, tarballData, 0o644); errorValue != nil {
		t.Fatalf("write tarball: %v", errorValue)
	}
	digest := sha256.Sum256(tarballData)
	if errorValue := os.WriteFile(checksumPath, []byte(hex.EncodeToString(digest[:])), 0o644); errorValue != nil {
		t.Fatalf("write checksum: %v", errorValue)
	}

	if !isMattermostTarballCacheValid(tarballPath, checksumPath) {
		t.Fatalf("expected cache with matching checksum to be valid")
	}
}

func TestIsMattermostTarballCacheValidRejectsMismatchedChecksum(t *testing.T) {
	directory := t.TempDir()
	tarballPath := filepath.Join(directory, "mattermost.tar.gz")
	checksumPath := tarballPath + ".sha256"
	if errorValue := os.WriteFile(tarballPath, validGzipBytes(t, "cached payload"), 0o644); errorValue != nil {
		t.Fatalf("write tarball: %v", errorValue)
	}
	if errorValue := os.WriteFile(checksumPath, []byte("0000000000000000000000000000000000000000000000000000000000000000"), 0o644); errorValue != nil {
		t.Fatalf("write checksum: %v", errorValue)
	}

	if isMattermostTarballCacheValid(tarballPath, checksumPath) {
		t.Fatalf("expected cache with mismatched checksum to be invalid")
	}
}

func TestIsMattermostTarballCacheValidRejectsMissingChecksumFile(t *testing.T) {
	directory := t.TempDir()
	tarballPath := filepath.Join(directory, "mattermost.tar.gz")
	if errorValue := os.WriteFile(tarballPath, validGzipBytes(t, "cached payload"), 0o644); errorValue != nil {
		t.Fatalf("write tarball: %v", errorValue)
	}

	if isMattermostTarballCacheValid(tarballPath, tarballPath+".sha256") {
		t.Fatalf("expected cache with missing checksum file to be invalid")
	}
}

func TestEnsureMattermostTarballCachedReturnsExistingValidCacheWithoutDownloading(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	tarballPath, checksumPath, errorValue := mattermostTarballCachePaths("11.9.0")
	if errorValue != nil {
		t.Fatalf("resolve cache paths: %v", errorValue)
	}
	if errorValue := os.MkdirAll(filepath.Dir(tarballPath), 0o755); errorValue != nil {
		t.Fatalf("create cache directory: %v", errorValue)
	}
	tarballData := validGzipBytes(t, "already cached")
	if errorValue := os.WriteFile(tarballPath, tarballData, 0o644); errorValue != nil {
		t.Fatalf("write tarball: %v", errorValue)
	}
	digest := sha256.Sum256(tarballData)
	if errorValue := os.WriteFile(checksumPath, []byte(hex.EncodeToString(digest[:])), 0o644); errorValue != nil {
		t.Fatalf("write checksum: %v", errorValue)
	}

	downloadCallCount := 0
	fakeDownloader := func(downloadURL string, destinationPath string) error {
		downloadCallCount++
		return errors.New("download should not be called when cache is valid")
	}

	resolvedTarballPath, resolvedChecksumPath, errorValue := ensureMattermostTarballCached("11.9.0", fakeDownloader)
	if errorValue != nil {
		t.Fatalf("unexpected error: %v", errorValue)
	}
	if resolvedTarballPath != tarballPath || resolvedChecksumPath != checksumPath {
		t.Fatalf("expected cached paths %q/%q, got %q/%q", tarballPath, checksumPath, resolvedTarballPath, resolvedChecksumPath)
	}
	if downloadCallCount != 0 {
		t.Fatalf("expected no download attempts, got %d", downloadCallCount)
	}
}

func TestEnsureMattermostTarballCachedDownloadsWhenCacheMissing(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	downloadCallCount := 0
	fakeDownloader := func(downloadURL string, destinationPath string) error {
		downloadCallCount++
		return os.WriteFile(destinationPath, validGzipBytes(t, "freshly downloaded"), 0o644)
	}

	tarballPath, checksumPath, errorValue := ensureMattermostTarballCached("11.9.0", fakeDownloader)
	if errorValue != nil {
		t.Fatalf("unexpected error: %v", errorValue)
	}
	if downloadCallCount != 1 {
		t.Fatalf("expected exactly one download attempt, got %d", downloadCallCount)
	}
	if !isMattermostTarballCacheValid(tarballPath, checksumPath) {
		t.Fatalf("expected freshly downloaded tarball to be cached and valid")
	}
}

func TestEnsureMattermostTarballCachedRetriesOnCorruptDownloadThenSucceeds(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	downloadCallCount := 0
	fakeDownloader := func(downloadURL string, destinationPath string) error {
		downloadCallCount++
		if downloadCallCount < mattermostDownloadAttemptCount {
			return os.WriteFile(destinationPath, []byte("corrupted, not gzip"), 0o644)
		}
		return os.WriteFile(destinationPath, validGzipBytes(t, "succeeded on final attempt"), 0o644)
	}

	tarballPath, checksumPath, errorValue := ensureMattermostTarballCached("11.9.0", fakeDownloader)
	if errorValue != nil {
		t.Fatalf("unexpected error: %v", errorValue)
	}
	if downloadCallCount != mattermostDownloadAttemptCount {
		t.Fatalf("expected %d download attempts, got %d", mattermostDownloadAttemptCount, downloadCallCount)
	}
	if !isMattermostTarballCacheValid(tarballPath, checksumPath) {
		t.Fatalf("expected tarball from final successful attempt to be cached and valid")
	}
}

func TestEnsureMattermostTarballCachedFailsAfterExhaustingAttempts(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	downloadCallCount := 0
	fakeDownloader := func(downloadURL string, destinationPath string) error {
		downloadCallCount++
		return errors.New("network unreachable")
	}

	_, _, errorValue := ensureMattermostTarballCached("11.9.0", fakeDownloader)
	if errorValue == nil {
		t.Fatalf("expected error after exhausting all download attempts")
	}
	if downloadCallCount != mattermostDownloadAttemptCount {
		t.Fatalf("expected %d download attempts, got %d", mattermostDownloadAttemptCount, downloadCallCount)
	}
}

func TestEnsureMattermostTarballCachedDeletesCorruptedTarballBeforeRetrying(t *testing.T) {
	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)

	fakeDownloader := func(downloadURL string, destinationPath string) error {
		return os.WriteFile(destinationPath, []byte("always corrupted"), 0o644)
	}

	tarballPath, _, errorValue := mattermostTarballCachePaths("11.9.0")
	if errorValue != nil {
		t.Fatalf("resolve cache paths: %v", errorValue)
	}

	_, _, errorValue = ensureMattermostTarballCached("11.9.0", fakeDownloader)
	if errorValue == nil {
		t.Fatalf("expected error for persistently corrupted downloads")
	}
	if _, statError := os.Stat(tarballPath); !os.IsNotExist(statError) {
		t.Fatalf("expected corrupted tarball to be removed, stat error: %v", statError)
	}
}

func TestMattermostGuestFallbackInstallScriptEmbedsPinnedVersionWithoutGitHubLookup(t *testing.T) {
	script := mattermostGuestFallbackInstallScript("11.9.0")

	if !bytes.Contains([]byte(script), []byte(`MMVER="11.9.0"`)) {
		t.Fatalf("expected fallback script to embed pinned version, got: %s", script)
	}
	if bytes.Contains([]byte(script), []byte("api.github.com")) {
		t.Fatalf("expected fallback script to not query GitHub releases API, got: %s", script)
	}
}
