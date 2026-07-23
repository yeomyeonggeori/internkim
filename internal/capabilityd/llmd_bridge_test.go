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
	"time"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestLLMDBridgeInjectsHostCredentialAndPreservesResponse(t *testing.T) {
	temporaryDirectory := t.TempDir()
	socketPath := filepath.Join("/tmp", filepath.Base(temporaryDirectory)+"-llmd.sock")
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	authKeyPath := filepath.Join(temporaryDirectory, "llmd.key")
	if errorValue := os.WriteFile(authKeyPath, []byte("host-key\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/llm/structured" || request.Header.Get("Authorization") != "Bearer host-key" {
			t.Fatalf("unexpected LLMD request: %s %s", request.URL.Path, request.Header.Get("Authorization"))
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Header().Set("Retry-After", "3")
		responseWriter.Header().Set("X-Request-ID", "llmd-request-1")
		responseWriter.WriteHeader(http.StatusTooManyRequests)
		_, _ = responseWriter.Write([]byte(`{"error":{"code":"rate_limited"}}`))
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
		_ = listener.Close()
	})

	service := Service{Configuration: Configuration{LLMDSocketPath: socketPath, LLMDAuthKeyPath: authKeyPath}.WithDefaults()}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/llmd/v1/llm/structured", strings.NewReader(`{"model":"test"}`))
	request.Header.Set("Authorization", "Bearer guest-key")
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusTooManyRequests || responseRecorder.Body.String() != `{"error":{"code":"rate_limited"}}` {
		t.Fatalf("expected unchanged LLMD response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if responseRecorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected LLMD content type, got %+v", responseRecorder.Header())
	}
	if responseRecorder.Header().Get("Retry-After") != "3" || responseRecorder.Header().Get("X-Request-ID") != "llmd-request-1" {
		t.Fatalf("expected safe LLMD response headers, got %+v", responseRecorder.Header())
	}
}

func TestLLMDBridgeInjectsHostCredentialAndPreservesChatResponse(t *testing.T) {
	temporaryDirectory := t.TempDir()
	socketPath := filepath.Join("/tmp", filepath.Base(temporaryDirectory)+"-llmd.sock")
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	authKeyPath := filepath.Join(temporaryDirectory, "llmd.key")
	if errorValue := os.WriteFile(authKeyPath, []byte("host-chat-key\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/llm/chat" || request.Header.Get("Authorization") != "Bearer host-chat-key" {
			t.Fatalf("unexpected LLMD chat request: %s %s", request.URL.Path, request.Header.Get("Authorization"))
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Header().Set("X-Request-ID", "llmd-chat-request-1")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"provider":"llmd","model":"test","selectedBackend":"device","finishReason":"stop","message":{"role":"assistant","content":"done"}}`))
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Shutdown(context.Background())
		_ = listener.Close()
	})

	service := Service{Configuration: Configuration{LLMDSocketPath: socketPath, LLMDAuthKeyPath: authKeyPath}.WithDefaults()}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/llmd/v1/llm/chat", strings.NewReader(`{"model":"test"}`))
	request.Header.Set("Authorization", "Bearer guest-key")
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK || responseRecorder.Body.String() != `{"provider":"llmd","model":"test","selectedBackend":"device","finishReason":"stop","message":{"role":"assistant","content":"done"}}` {
		t.Fatalf("expected unchanged LLMD chat response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if responseRecorder.Header().Get("Content-Type") != "application/json" || responseRecorder.Header().Get("X-Request-ID") != "llmd-chat-request-1" {
		t.Fatalf("expected safe LLMD chat response headers, got %+v", responseRecorder.Header())
	}
}

func TestLLMDHealthBridgeUsesExactReadOnlyRoute(t *testing.T) {
	identity := capabilityprotocol.GeneratedProtocolIdentity()
	service := Service{
		Configuration: Configuration{LLMDSocketPath: "/tmp/llmd-health-test.sock"},
		LLMDHTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.Path != "/health" || request.Header.Get("Authorization") != "" {
				t.Fatalf("unexpected LLMD health request: %s %s authorization=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
			}
			return testJSONResponse(http.StatusOK, map[string]string{
				"status":                "ok",
				"protocolVersion":       identity.ProtocolVersion,
				"aggregateProtocolHash": identity.AggregateProtocolHash,
			}), nil
		})},
	}
	request := httptest.NewRequest(http.MethodGet, "/_internkim/llmd/health", nil)
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected LLMD health bridge success, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response llmdHealthResponse
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected typed LLMD health response: %v", errorValue)
	}
	if response.ProtocolIdentity != identity || response.Status != "ok" {
		t.Fatalf("unexpected LLMD health response: %+v", response)
	}
}

func TestLLMDHealthBridgeRejectsMalformedResponse(t *testing.T) {
	identity := capabilityprotocol.GeneratedProtocolIdentity()
	unknownFieldDocument := `{"status":"ok","protocolVersion":"` + identity.ProtocolVersion + `","aggregateProtocolHash":"` + identity.AggregateProtocolHash + `","unexpected":true}`
	for _, document := range []string{`{"status":"ok"}`, `not-json`, unknownFieldDocument} {
		service := Service{
			Configuration: Configuration{LLMDSocketPath: "/tmp/llmd-health-test.sock"},
			LLMDHTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(document)),
				}, nil
			})},
		}
		request := httptest.NewRequest(http.MethodGet, "/_internkim/llmd/health", nil)
		responseRecorder := httptest.NewRecorder()

		service.router().ServeHTTP(responseRecorder, request)

		if responseRecorder.Code != http.StatusBadGateway {
			t.Fatalf("expected malformed LLMD health rejection, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
		}
		assertLLMDErrorResponse(t, responseRecorder, "llmd_bridge_response_invalid", false)
	}
}

func TestLLMDBridgeRejectsOversizedRequestWithoutFallback(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(
		http.MethodPost,
		"/_internkim/llmd/v1/llm/structured",
		strings.NewReader(strings.Repeat("x", llmdMaximumBodyBytes+1)),
	)
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected request size rejection, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	assertLLMDErrorResponse(t, responseRecorder, "request_too_large", false)
}

func TestLLMDBridgeRejectsOversizedResponseWithoutFallback(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusCreated,
		Header: http.Header{
			"Content-Type": []string{"application/octet-stream"},
			"X-Request-ID": []string{"oversized-request"},
		},
		Body: io.NopCloser(strings.NewReader(strings.Repeat("x", llmdMaximumBodyBytes+1))),
	}
	responseRecorder := httptest.NewRecorder()

	copyLLMDResponse(responseRecorder, response)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected response size rejection, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if responseRecorder.Header().Get("Content-Type") != "application/json" || responseRecorder.Header().Get("X-Request-ID") != "" {
		t.Fatalf("expected deterministic bridge error headers, got %+v", responseRecorder.Header())
	}
	assertLLMDErrorResponse(t, responseRecorder, "llmd_bridge_response_invalid", false)
}

func assertLLMDErrorResponse(t *testing.T, responseRecorder *httptest.ResponseRecorder, expectedCode string, expectedFallback bool) {
	t.Helper()
	var responseDocument struct {
		Error struct {
			Code                string `json:"code"`
			AllowLegacyFallback bool   `json:"allowLegacyFallback"`
		} `json:"error"`
	}
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &responseDocument); errorValue != nil {
		t.Fatalf("expected JSON LLMD error: %v", errorValue)
	}
	if responseDocument.Error.Code != expectedCode || responseDocument.Error.AllowLegacyFallback != expectedFallback {
		t.Fatalf("unexpected LLMD error: %+v", responseDocument.Error)
	}
}

func TestLLMDBridgeExposesOnlyAllowedRoutes(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	for _, path := range []string{"/_internkim/llmd/v1/llm/text", "/_internkim/llmd/v1/tools/test/invoke", "/_internkim/llmd/v1/llm/unknown"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		responseRecorder := httptest.NewRecorder()
		service.router().ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected %s to be hidden, got %d", path, responseRecorder.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/llmd/health", nil)
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected LLMD health to allow only GET, got %d", responseRecorder.Code)
	}
}

func TestLLMDBridgePreservesCancellation(t *testing.T) {
	temporaryDirectory := t.TempDir()
	socketPath := filepath.Join("/tmp", filepath.Base(temporaryDirectory)+"-llmd.sock")
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	requestStarted := make(chan struct{})
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		close(requestStarted)
		<-request.Context().Done()
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	service := Service{
		Configuration: Configuration{LLMDSocketPath: socketPath}.WithDefaults(),
		LLMDAuthKey:   "host-key",
	}
	requestContext, cancelRequest := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/_internkim/llmd/v1/llm/chat", strings.NewReader(`{"model":"test"}`)).WithContext(requestContext)
	responseRecorder := httptest.NewRecorder()
	requestFinished := make(chan struct{})
	go func() {
		service.router().ServeHTTP(responseRecorder, request)
		close(requestFinished)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("LLMD request did not start")
	}
	cancelRequest()
	select {
	case <-requestFinished:
	case <-time.After(time.Second):
		t.Fatal("LLMD request did not finish after cancellation")
	}

	if responseRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected cancellation fallback status, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	assertLLMDErrorResponse(t, responseRecorder, "llmd_bridge_unavailable", true)
}

func TestLLMDHTTPClientHasNoIndependentRequestTimeout(t *testing.T) {
	client := (Service{Configuration: DefaultConfiguration()}).newLLMDHTTPClient()
	if client.Timeout != 0 {
		t.Fatalf("expected request context to own LLMD cancellation, got client timeout %s", client.Timeout)
	}
	transport, isTransport := client.Transport.(*http.Transport)
	if !isTransport {
		t.Fatalf("expected LLMD HTTP transport, got %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout != 0 {
		t.Fatalf("expected no independent LLMD response header timeout, got %s", transport.ResponseHeaderTimeout)
	}
}
