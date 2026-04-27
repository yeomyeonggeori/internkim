package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

func TestGatewayRoutesAdminAndMattermost(t *testing.T) {
	service := NewService(Configuration{
		MattermostBaseURL: "http://mattermost.local",
		AdminEmailPath:    writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(bytes.NewBufferString("mattermost")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    request,
		}, nil
	})}
	handler := service.router()

	adminRequest := httptest.NewRequest(http.MethodGet, "/_internkim/admin/health", nil)
	adminRequest.RemoteAddr = "127.0.0.1:12345"
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin health status = %d", adminResponse.Code)
	}

	mattermostRequest := httptest.NewRequest(http.MethodGet, "/team/channels/town-square", nil)
	mattermostResponse := httptest.NewRecorder()
	handler.ServeHTTP(mattermostResponse, mattermostRequest)
	if mattermostResponse.Code != http.StatusAccepted {
		t.Fatalf("mattermost proxy status = %d", mattermostResponse.Code)
	}
	if mattermostResponse.Body.String() != "mattermost" {
		t.Fatalf("mattermost proxy body = %q", mattermostResponse.Body.String())
	}
}

func TestAdminRejectsUnauthorizedRemoteCaller(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()

	request := httptest.NewRequest(http.MethodGet, "/_internkim/admin/health", nil)
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unauthorized status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/_internkim/admin/health", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authorized status = %d", response.Code)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	directoryPath := t.TempDir()
	plainPath := filepath.Join(directoryPath, "plain.tar.gz")
	encryptedPath := filepath.Join(directoryPath, "backup.ikbak")
	decryptedPath := filepath.Join(directoryPath, "decrypted.tar.gz")
	document := []byte("backup document")
	if errorValue := os.WriteFile(plainPath, document, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := encryptFile(plainPath, encryptedPath, "passphrase"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := decryptFile(encryptedPath, decryptedPath, "passphrase"); errorValue != nil {
		t.Fatal(errorValue)
	}
	decryptedDocument, errorValue := os.ReadFile(decryptedPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(decryptedDocument) != string(document) {
		t.Fatalf("decrypted document = %q", string(decryptedDocument))
	}
}

func TestRestoreUploadAssembly(t *testing.T) {
	directoryPath := t.TempDir()
	chunksPath := filepath.Join(directoryPath, "chunks")
	if errorValue := os.MkdirAll(chunksPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(chunksPath, "0"), []byte("hello "), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(chunksPath, "1"), []byte("world"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	service := NewService(Configuration{})
	bundlePath := filepath.Join(directoryPath, "bundle.ikbak")
	errorValue := service.assembleRestoreUpload(&RestoreUpload{DirectoryPath: directoryPath}, 2, bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := os.ReadFile(bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(document) != "hello world" {
		t.Fatalf("assembled document = %q", string(document))
	}
}

func TestRestoreUploadAssemblyRequiresEveryChunk(t *testing.T) {
	service := NewService(Configuration{})
	errorValue := service.assembleRestoreUpload(&RestoreUpload{DirectoryPath: t.TempDir()}, 1, filepath.Join(t.TempDir(), "bundle.ikbak"))
	if errorValue == nil {
		t.Fatal("expected missing chunk error")
	}
}

func TestExtractBundleRejectsUnsafePath(t *testing.T) {
	bundlePath := filepath.Join(t.TempDir(), "backup.tar.gz")
	bundleFile, errorValue := os.Create(bundlePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	gzipWriter := gzip.NewWriter(bundleFile)
	tarWriter := tar.NewWriter(gzipWriter)
	if errorValue := tarWriter.WriteHeader(&tar.Header{Name: "../evil", Mode: 0o600, Size: 4}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := tarWriter.Write([]byte("evil")); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := tarWriter.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := gzipWriter.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := bundleFile.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue = extractBundle(bundlePath, t.TempDir())
	if errorValue == nil {
		t.Fatal("expected unsafe path error")
	}
}

func TestCompanionPairHeartbeatAndJobLifecycle(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	handler := service.router()

	pairingResponse := httptest.NewRecorder()
	pairingRequest := httptest.NewRequest(http.MethodPost, "/_internkim/admin/companion/pairing-codes", nil)
	pairingRequest.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(pairingResponse, pairingRequest)
	if pairingResponse.Code != http.StatusOK {
		t.Fatalf("pairing code status = %d", pairingResponse.Code)
	}
	var pairingCode companionPairingCodeResponse
	if errorValue := json.NewDecoder(pairingResponse.Body).Decode(&pairingCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	if pairingCode.Code == "" || time.Until(pairingCode.ExpiresAt) <= 0 {
		t.Fatalf("unexpected pairing code response: %+v", pairingCode)
	}

	pairResponse := httptest.NewRecorder()
	pairRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{
		"code":"`+pairingCode.Code+`",
		"displayName":"test companion",
		"publicKey":"pk-test",
		"localOnly":true,
		"capabilities":[{"name":"user.confirm","version":"1","privacyClass":"user_input","estimatedLatency":"interactive","requiresUserPresence":true,"worksOffline":true}]
	}`))
	handler.ServeHTTP(pairResponse, pairRequest)
	if pairResponse.Code != http.StatusOK {
		t.Fatalf("pair status = %d: %s", pairResponse.Code, pairResponse.Body.String())
	}
	var pairResult companionPairResponse
	if errorValue := json.NewDecoder(pairResponse.Body).Decode(&pairResult); errorValue != nil {
		t.Fatal(errorValue)
	}
	if pairResult.CompanionID == "" || pairResult.Token == "" {
		t.Fatalf("unexpected pair result: %+v", pairResult)
	}

	reuseResponse := httptest.NewRecorder()
	reuseRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/pair", strings.NewReader(`{"code":"`+pairingCode.Code+`"}`))
	handler.ServeHTTP(reuseResponse, reuseRequest)
	if reuseResponse.Code != http.StatusForbidden {
		t.Fatalf("expected reused pairing code to fail, got %d", reuseResponse.Code)
	}

	statusResponse := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/_internkim/admin/companion/status", nil)
	statusRequest.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status code = %d", statusResponse.Code)
	}
	var status companionStatusResponse
	if errorValue := json.NewDecoder(statusResponse.Body).Decode(&status); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(status.Companions) != 1 || !status.Companions[0].IsOnline {
		t.Fatalf("unexpected companion status: %+v", status)
	}

	resultChannel := make(chan capabilities.ToolInvokeResponse, 1)
	errorChannel := make(chan error, 1)
	go func() {
		response, errorValue := service.invokeCompanionJob(context.Background(), capabilities.ToolInvokeRequest{
			ToolName:      "user.confirm",
			Input:         json.RawMessage(`{"message":"continue?"}`),
			TimeoutSecond: 2,
		})
		if errorValue != nil {
			errorChannel <- errorValue
			return
		}
		resultChannel <- response
	}()

	nextResponse := httptest.NewRecorder()
	nextRequest := httptest.NewRequest(http.MethodGet, "/_internkim/companion/jobs/next", nil)
	setCompanionHeaders(nextRequest, pairResult)
	handler.ServeHTTP(nextResponse, nextRequest)
	if nextResponse.Code != http.StatusOK {
		t.Fatalf("next job status = %d: %s", nextResponse.Code, nextResponse.Body.String())
	}
	var companionJob CompanionJob
	if errorValue := json.NewDecoder(nextResponse.Body).Decode(&companionJob); errorValue != nil {
		t.Fatal(errorValue)
	}
	if companionJob.JobID == "" || companionJob.Request.ToolName != "user.confirm" {
		t.Fatalf("unexpected companion job: %+v", companionJob)
	}

	completeResponse := httptest.NewRecorder()
	completeRequest := httptest.NewRequest(http.MethodPost, "/_internkim/companion/jobs/"+companionJob.JobID+"/complete", strings.NewReader(`{
		"provider":"companion",
		"selectedBackend":"companion_local",
		"toolName":"user.confirm",
		"result":{"confirmed":true}
	}`))
	setCompanionHeaders(completeRequest, pairResult)
	handler.ServeHTTP(completeResponse, completeRequest)
	if completeResponse.Code != http.StatusOK {
		t.Fatalf("complete status = %d: %s", completeResponse.Code, completeResponse.Body.String())
	}

	select {
	case response := <-resultChannel:
		if response.ToolName != "user.confirm" {
			t.Fatalf("unexpected invoke response: %+v", response)
		}
	case errorValue := <-errorChannel:
		t.Fatal(errorValue)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for companion job result")
	}
}

func setCompanionHeaders(request *http.Request, pairResult companionPairResponse) {
	request.Header.Set("X-InternKim-Companion-ID", pairResult.CompanionID)
	request.Header.Set("X-InternKim-Companion-Token", pairResult.Token)
}

func writeTestFile(t *testing.T, document string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
