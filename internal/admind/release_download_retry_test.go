package admind

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/releaseset"
)

func releaseBlobTestService(t *testing.T, registryURL string) *Service {
	t.Helper()
	previousDelay := releaseDownloadRetryDelay
	releaseDownloadRetryDelay = time.Millisecond
	t.Cleanup(func() { releaseDownloadRetryDelay = previousDelay })
	return NewService(Configuration{ReleaseRegistryURL: registryURL})
}

func TestAResetStreamDoesNotCostTheWholeRelease(t *testing.T) {
	blob := []byte("the admind binary, as far as this test is concerned")
	digest := sha256.Sum256(blob)
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts < 3 {
			responseWriter.Header().Set("Content-Length", "4096")
			responseWriter.Write(blob[:4])
			return
		}
		responseWriter.Write(blob)
	}))
	defer server.Close()

	service := releaseBlobTestService(t, server.URL)
	archivePath := filepath.Join(t.TempDir(), "component.tar.gz")

	errorValue := service.fetchReleaseBlob(context.Background(), "admind",
		releaseset.Component{Name: "admind", BlobPath: "/blobs/admind.tar.gz", SHA256: hex.EncodeToString(digest[:])}, archivePath)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if attempts != 3 {
		t.Fatalf("the uplink dropped twice and the download should have been asked for three times, not %d", attempts)
	}
}

func TestABlobThatNeverArrivesStillFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Error(responseWriter, "no such blob", http.StatusNotFound)
	}))
	defer server.Close()

	service := releaseBlobTestService(t, server.URL)
	archivePath := filepath.Join(t.TempDir(), "component.tar.gz")

	errorValue := service.fetchReleaseBlob(context.Background(), "admind",
		releaseset.Component{Name: "admind", BlobPath: "/blobs/admind.tar.gz", SHA256: "unused"}, archivePath)

	if errorValue == nil {
		t.Fatal("a release that cannot be downloaded must not report success")
	}
}
