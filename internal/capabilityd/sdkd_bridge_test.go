package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKDBridgeInjectsHostCredentialAndPreservesResponse(t *testing.T) {
	temporaryDirectory := t.TempDir()
	socketPath := filepath.Join("/tmp", filepath.Base(temporaryDirectory)+"-sdkd.sock")
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	authKeyPath := filepath.Join(temporaryDirectory, "sdkd.key")
	if errorValue := os.WriteFile(authKeyPath, []byte("host-key\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/llm/structured" || request.Header.Get("Authorization") != "Bearer host-key" {
			t.Fatalf("unexpected SDKD request: %s %s", request.URL.Path, request.Header.Get("Authorization"))
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Header().Set("Retry-After", "3")
		responseWriter.Header().Set("X-Request-ID", "sdkd-request-1")
		responseWriter.WriteHeader(http.StatusTooManyRequests)
		_, _ = responseWriter.Write([]byte(`{"error":{"code":"rate_limited"}}`))
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
		_ = listener.Close()
	})

	service := Service{Configuration: Configuration{SDKDSocketPath: socketPath, SDKDAuthKeyPath: authKeyPath}.WithDefaults()}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/sdkd/v1/llm/structured", strings.NewReader(`{"model":"test"}`))
	request.Header.Set("Authorization", "Bearer guest-key")
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusTooManyRequests || responseRecorder.Body.String() != `{"error":{"code":"rate_limited"}}` {
		t.Fatalf("expected unchanged SDKD response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if responseRecorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected SDKD content type, got %+v", responseRecorder.Header())
	}
	if responseRecorder.Header().Get("Retry-After") != "3" || responseRecorder.Header().Get("X-Request-ID") != "sdkd-request-1" {
		t.Fatalf("expected safe SDKD response headers, got %+v", responseRecorder.Header())
	}
}

func TestSDKDBridgeRejectsOversizedRequestWithoutFallback(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(
		http.MethodPost,
		"/_internkim/sdkd/v1/llm/structured",
		strings.NewReader(strings.Repeat("x", sdkdMaximumBodyBytes+1)),
	)
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected request size rejection, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	assertSDKDErrorResponse(t, responseRecorder, "request_too_large", false)
}

func TestSDKDBridgeRejectsOversizedResponseWithoutFallback(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusCreated,
		Header: http.Header{
			"Content-Type": []string{"application/octet-stream"},
			"X-Request-ID": []string{"oversized-request"},
		},
		Body: io.NopCloser(strings.NewReader(strings.Repeat("x", sdkdMaximumBodyBytes+1))),
	}
	responseRecorder := httptest.NewRecorder()

	copySDKDResponse(responseRecorder, response)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected response size rejection, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if responseRecorder.Header().Get("Content-Type") != "application/json" || responseRecorder.Header().Get("X-Request-ID") != "" {
		t.Fatalf("expected deterministic bridge error headers, got %+v", responseRecorder.Header())
	}
	assertSDKDErrorResponse(t, responseRecorder, "sdkd_bridge_response_invalid", false)
}

func assertSDKDErrorResponse(t *testing.T, responseRecorder *httptest.ResponseRecorder, expectedCode string, expectedFallback bool) {
	t.Helper()
	var responseDocument struct {
		Error struct {
			Code                string `json:"code"`
			AllowLegacyFallback bool   `json:"allowLegacyFallback"`
		} `json:"error"`
	}
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &responseDocument); errorValue != nil {
		t.Fatalf("expected JSON SDKD error: %v", errorValue)
	}
	if responseDocument.Error.Code != expectedCode || responseDocument.Error.AllowLegacyFallback != expectedFallback {
		t.Fatalf("unexpected SDKD error: %+v", responseDocument.Error)
	}
}

func TestSDKDBridgeExposesOnlyStructuredRoute(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	for _, path := range []string{"/_internkim/sdkd/health", "/_internkim/sdkd/v1/llm/text", "/_internkim/sdkd/v1/tools/test/invoke"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		responseRecorder := httptest.NewRecorder()
		service.router().ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected %s to be hidden, got %d", path, responseRecorder.Code)
		}
	}
}
