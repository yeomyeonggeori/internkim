package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

const llmdMaximumBodyBytes = 8 * 1024 * 1024

var errLLMDRequestTooLarge = errors.New("LLMD request exceeds 8 MiB")
var errLLMDHealthInvalid = errors.New("LLMD health response is invalid")

type llmdHealthResponse struct {
	capabilityprotocol.ProtocolIdentity
	Status string `json:"status"`
}

func (service Service) handleLLMDStructured(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleLLMDBridge(responseWriter, request)
}

func (service Service) handleLLMDChat(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleLLMDBridge(responseWriter, request)
}

func (service Service) handleLLMDHealth(responseWriter http.ResponseWriter, request *http.Request) {
	health, errorValue := service.fetchLLMDHealth(request.Context())
	if errorValue != nil {
		if errors.Is(errorValue, errLLMDHealthInvalid) {
			writeLLMDBridgeInvalidResponse(responseWriter)
			return
		}
		writeLLMDBridgeUnavailable(responseWriter)
		return
	}
	service.writeJSON(responseWriter, health)
}

func (service Service) llmdHealth(ctx context.Context) map[string]any {
	health, errorValue := service.fetchLLMDHealth(ctx)
	if errorValue != nil {
		reason := "unavailable"
		if errors.Is(errorValue, errLLMDHealthInvalid) {
			reason = "malformed"
		}
		return map[string]any{
			"ok":     false,
			"status": "unhealthy",
			"reason": reason,
		}
	}
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	if health.ProtocolIdentity != expectedIdentity {
		return map[string]any{
			"ok":                    false,
			"status":                "unhealthy",
			"reason":                "protocol identity mismatch",
			"protocolVersion":       health.ProtocolVersion,
			"aggregateProtocolHash": health.AggregateProtocolHash,
		}
	}
	return map[string]any{
		"ok":                    true,
		"status":                health.Status,
		"protocolVersion":       health.ProtocolVersion,
		"aggregateProtocolHash": health.AggregateProtocolHash,
	}
}

func (service Service) fetchLLMDHealth(ctx context.Context) (llmdHealthResponse, error) {
	request, errorValue := service.newLLMDHealthRequest(ctx)
	if errorValue != nil {
		return llmdHealthResponse{}, errorValue
	}
	response, errorValue := service.llmdHTTPClient().Do(request)
	if errorValue != nil {
		return llmdHealthResponse{}, errorValue
	}
	if response.Body == nil {
		return llmdHealthResponse{}, errLLMDHealthInvalid
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return llmdHealthResponse{}, fmt.Errorf("LLMD health returned status %d", response.StatusCode)
	}
	document, errorValue := io.ReadAll(io.LimitReader(response.Body, llmdMaximumBodyBytes+1))
	if errorValue != nil {
		return llmdHealthResponse{}, errorValue
	}
	if len(document) > llmdMaximumBodyBytes {
		return llmdHealthResponse{}, errLLMDHealthInvalid
	}
	return decodeLLMDHealthResponse(document)
}

func (service Service) newLLMDHealthRequest(ctx context.Context) (*http.Request, error) {
	if strings.TrimSpace(service.Configuration.LLMDSocketPath) == "" {
		return nil, errors.New("LLMD socket path is not configured")
	}
	return http.NewRequestWithContext(ctx, http.MethodGet, "http://blueclaw-llmd/health", nil)
}

func decodeLLMDHealthResponse(document []byte) (llmdHealthResponse, error) {
	var health llmdHealthResponse
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&health); errorValue != nil {
		return llmdHealthResponse{}, fmt.Errorf("%w: %v", errLLMDHealthInvalid, errorValue)
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		return llmdHealthResponse{}, fmt.Errorf("%w: trailing content", errLLMDHealthInvalid)
	}
	if health.Status != "ok" || health.ProtocolIdentity.Validate() != nil {
		return llmdHealthResponse{}, errLLMDHealthInvalid
	}
	return health, nil
}

func (service Service) handleLLMDBridge(responseWriter http.ResponseWriter, request *http.Request) {
	upstreamPath, isAllowed := llmdBridgeUpstreamPath(request.URL.Path)
	if !isAllowed {
		http.NotFound(responseWriter, request)
		return
	}
	requestBody, errorValue := readLLMDRequestBody(request)
	if errorValue != nil {
		if errors.Is(errorValue, errLLMDRequestTooLarge) {
			writeLLMDRequestTooLarge(responseWriter)
			return
		}
		writeLLMDBridgeUnavailable(responseWriter)
		return
	}
	proxyRequest, errorValue := service.newLLMDRequest(request, requestBody, upstreamPath)
	if errorValue != nil {
		writeLLMDBridgeConfigurationError(responseWriter)
		return
	}
	response, errorValue := service.llmdHTTPClient().Do(proxyRequest)
	if errorValue != nil {
		writeLLMDBridgeUnavailable(responseWriter)
		return
	}
	defer response.Body.Close()
	copyLLMDResponse(responseWriter, response)
}

func readLLMDRequestBody(request *http.Request) ([]byte, error) {
	if request.ContentLength > llmdMaximumBodyBytes {
		return nil, errLLMDRequestTooLarge
	}
	requestBody, errorValue := io.ReadAll(io.LimitReader(request.Body, llmdMaximumBodyBytes+1))
	if errorValue != nil {
		return nil, errorValue
	}
	if len(requestBody) > llmdMaximumBodyBytes {
		return nil, errLLMDRequestTooLarge
	}
	return requestBody, nil
}

func llmdBridgeUpstreamPath(ingressPath string) (string, bool) {
	switch ingressPath {
	case "/_internkim/llmd/v1/llm/structured":
		return "/v1/llm/structured", true
	case "/_internkim/llmd/v1/llm/chat":
		return "/v1/llm/chat", true
	default:
		return "", false
	}
}

func isLLMDBridgeUpstreamPath(upstreamPath string) bool {
	return upstreamPath == "/v1/llm/structured" || upstreamPath == "/v1/llm/chat"
}

func (service Service) newLLMDRequest(request *http.Request, requestBody []byte, upstreamPath string) (*http.Request, error) {
	if !isLLMDBridgeUpstreamPath(upstreamPath) {
		return nil, errors.New("LLMD upstream path is not allowed")
	}
	authKey := service.LLMDAuthKey
	if authKey == "" {
		authKey = readSecretValue(service.Configuration.LLMDAuthKeyPath)
	}
	if authKey == "" {
		return nil, errors.New("LLMD auth key is not configured")
	}
	proxyRequest, errorValue := http.NewRequestWithContext(
		request.Context(),
		http.MethodPost,
		"http://blueclaw-llmd"+upstreamPath,
		bytes.NewReader(requestBody),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	proxyRequest.Header.Set("Authorization", "Bearer "+authKey)
	proxyRequest.Header.Set("Content-Type", "application/json")
	return proxyRequest, nil
}

func (service Service) llmdHTTPClient() *http.Client {
	if service.LLMDHTTPClient != nil {
		return service.LLMDHTTPClient
	}
	return service.newLLMDHTTPClient()
}

func (service Service) newLLMDHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", service.Configuration.LLMDSocketPath)
		},
		IdleConnTimeout: 90 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func writeLLMDRequestTooLarge(responseWriter http.ResponseWriter) {
	writeLLMDError(responseWriter, http.StatusRequestEntityTooLarge, "request_too_large", false)
}

func writeLLMDBridgeConfigurationError(responseWriter http.ResponseWriter) {
	writeLLMDError(responseWriter, http.StatusInternalServerError, "llmd_bridge_configuration_invalid", false)
}

func writeLLMDBridgeUnavailable(responseWriter http.ResponseWriter) {
	writeLLMDError(responseWriter, http.StatusServiceUnavailable, "llmd_bridge_unavailable", true)
}

func writeLLMDBridgeInvalidResponse(responseWriter http.ResponseWriter) {
	writeLLMDError(responseWriter, http.StatusBadGateway, "llmd_bridge_response_invalid", false)
}

func writeLLMDError(responseWriter http.ResponseWriter, statusCode int, code string, allowLegacyFallback bool) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"error": map[string]any{
			"code":                code,
			"allowLegacyFallback": allowLegacyFallback,
		},
	})
}

func copyLLMDResponse(responseWriter http.ResponseWriter, response *http.Response) {
	responseBody, errorValue := io.ReadAll(io.LimitReader(response.Body, llmdMaximumBodyBytes+1))
	if errorValue != nil {
		writeLLMDBridgeUnavailable(responseWriter)
		return
	}
	if len(responseBody) > llmdMaximumBodyBytes {
		writeLLMDBridgeInvalidResponse(responseWriter)
		return
	}
	for _, headerName := range []string{"Content-Type", "Retry-After", "X-Request-ID"} {
		if headerValue := strings.TrimSpace(response.Header.Get(headerName)); headerValue != "" {
			responseWriter.Header().Set(headerName, headerValue)
		}
	}
	responseWriter.WriteHeader(response.StatusCode)
	_, _ = responseWriter.Write(responseBody)
}
