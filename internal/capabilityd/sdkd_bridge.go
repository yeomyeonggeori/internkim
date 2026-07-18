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

const sdkdMaximumBodyBytes = 8 * 1024 * 1024

var errSDKDRequestTooLarge = errors.New("SDKD request exceeds 8 MiB")
var errSDKDHealthInvalid = errors.New("SDKD health response is invalid")

type sdkdHealthResponse struct {
	capabilityprotocol.ProtocolIdentity
	Status string `json:"status"`
}

func (service Service) handleSDKDStructured(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleSDKDBridge(responseWriter, request)
}

func (service Service) handleSDKDChat(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleSDKDBridge(responseWriter, request)
}

func (service Service) handleSDKDHealth(responseWriter http.ResponseWriter, request *http.Request) {
	health, errorValue := service.fetchSDKDHealth(request.Context())
	if errorValue != nil {
		if errors.Is(errorValue, errSDKDHealthInvalid) {
			writeSDKDBridgeInvalidResponse(responseWriter)
			return
		}
		writeSDKDBridgeUnavailable(responseWriter)
		return
	}
	service.writeJSON(responseWriter, health)
}

func (service Service) sdkdHealth(ctx context.Context) map[string]any {
	health, errorValue := service.fetchSDKDHealth(ctx)
	if errorValue != nil {
		reason := "unavailable"
		if errors.Is(errorValue, errSDKDHealthInvalid) {
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

func (service Service) fetchSDKDHealth(ctx context.Context) (sdkdHealthResponse, error) {
	request, errorValue := service.newSDKDHealthRequest(ctx)
	if errorValue != nil {
		return sdkdHealthResponse{}, errorValue
	}
	response, errorValue := service.sdkdHTTPClient().Do(request)
	if errorValue != nil {
		return sdkdHealthResponse{}, errorValue
	}
	if response.Body == nil {
		return sdkdHealthResponse{}, errSDKDHealthInvalid
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return sdkdHealthResponse{}, fmt.Errorf("SDKD health returned status %d", response.StatusCode)
	}
	document, errorValue := io.ReadAll(io.LimitReader(response.Body, sdkdMaximumBodyBytes+1))
	if errorValue != nil {
		return sdkdHealthResponse{}, errorValue
	}
	if len(document) > sdkdMaximumBodyBytes {
		return sdkdHealthResponse{}, errSDKDHealthInvalid
	}
	return decodeSDKDHealthResponse(document)
}

func (service Service) newSDKDHealthRequest(ctx context.Context) (*http.Request, error) {
	if strings.TrimSpace(service.Configuration.SDKDSocketPath) == "" {
		return nil, errors.New("SDKD socket path is not configured")
	}
	return http.NewRequestWithContext(ctx, http.MethodGet, "http://blueclaw-sdkd/health", nil)
}

func decodeSDKDHealthResponse(document []byte) (sdkdHealthResponse, error) {
	var health sdkdHealthResponse
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&health); errorValue != nil {
		return sdkdHealthResponse{}, fmt.Errorf("%w: %v", errSDKDHealthInvalid, errorValue)
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		return sdkdHealthResponse{}, fmt.Errorf("%w: trailing content", errSDKDHealthInvalid)
	}
	if health.Status != "ok" || health.ProtocolIdentity.Validate() != nil {
		return sdkdHealthResponse{}, errSDKDHealthInvalid
	}
	return health, nil
}

func (service Service) handleSDKDBridge(responseWriter http.ResponseWriter, request *http.Request) {
	upstreamPath, isAllowed := sdkdBridgeUpstreamPath(request.URL.Path)
	if !isAllowed {
		http.NotFound(responseWriter, request)
		return
	}
	requestBody, errorValue := readSDKDRequestBody(request)
	if errorValue != nil {
		if errors.Is(errorValue, errSDKDRequestTooLarge) {
			writeSDKDRequestTooLarge(responseWriter)
			return
		}
		writeSDKDBridgeUnavailable(responseWriter)
		return
	}
	proxyRequest, errorValue := service.newSDKDRequest(request, requestBody, upstreamPath)
	if errorValue != nil {
		writeSDKDBridgeConfigurationError(responseWriter)
		return
	}
	response, errorValue := service.sdkdHTTPClient().Do(proxyRequest)
	if errorValue != nil {
		writeSDKDBridgeUnavailable(responseWriter)
		return
	}
	defer response.Body.Close()
	copySDKDResponse(responseWriter, response)
}

func readSDKDRequestBody(request *http.Request) ([]byte, error) {
	if request.ContentLength > sdkdMaximumBodyBytes {
		return nil, errSDKDRequestTooLarge
	}
	requestBody, errorValue := io.ReadAll(io.LimitReader(request.Body, sdkdMaximumBodyBytes+1))
	if errorValue != nil {
		return nil, errorValue
	}
	if len(requestBody) > sdkdMaximumBodyBytes {
		return nil, errSDKDRequestTooLarge
	}
	return requestBody, nil
}

func sdkdBridgeUpstreamPath(ingressPath string) (string, bool) {
	switch ingressPath {
	case "/_internkim/sdkd/v1/llm/structured":
		return "/v1/llm/structured", true
	case "/_internkim/sdkd/v1/llm/chat":
		return "/v1/llm/chat", true
	default:
		return "", false
	}
}

func isSDKDBridgeUpstreamPath(upstreamPath string) bool {
	return upstreamPath == "/v1/llm/structured" || upstreamPath == "/v1/llm/chat"
}

func (service Service) newSDKDRequest(request *http.Request, requestBody []byte, upstreamPath string) (*http.Request, error) {
	if !isSDKDBridgeUpstreamPath(upstreamPath) {
		return nil, errors.New("SDKD upstream path is not allowed")
	}
	authKey := service.SDKDAuthKey
	if authKey == "" {
		authKey = readSecretValue(service.Configuration.SDKDAuthKeyPath)
	}
	if authKey == "" {
		return nil, errors.New("SDKD auth key is not configured")
	}
	proxyRequest, errorValue := http.NewRequestWithContext(
		request.Context(),
		http.MethodPost,
		"http://blueclaw-sdkd"+upstreamPath,
		bytes.NewReader(requestBody),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	proxyRequest.Header.Set("Authorization", "Bearer "+authKey)
	proxyRequest.Header.Set("Content-Type", "application/json")
	return proxyRequest, nil
}

func (service Service) sdkdHTTPClient() *http.Client {
	if service.SDKDHTTPClient != nil {
		return service.SDKDHTTPClient
	}
	return service.newSDKDHTTPClient()
}

func (service Service) newSDKDHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", service.Configuration.SDKDSocketPath)
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

func writeSDKDRequestTooLarge(responseWriter http.ResponseWriter) {
	writeSDKDError(responseWriter, http.StatusRequestEntityTooLarge, "request_too_large", false)
}

func writeSDKDBridgeConfigurationError(responseWriter http.ResponseWriter) {
	writeSDKDError(responseWriter, http.StatusInternalServerError, "sdkd_bridge_configuration_invalid", false)
}

func writeSDKDBridgeUnavailable(responseWriter http.ResponseWriter) {
	writeSDKDError(responseWriter, http.StatusServiceUnavailable, "sdkd_bridge_unavailable", true)
}

func writeSDKDBridgeInvalidResponse(responseWriter http.ResponseWriter) {
	writeSDKDError(responseWriter, http.StatusBadGateway, "sdkd_bridge_response_invalid", false)
}

func writeSDKDError(responseWriter http.ResponseWriter, statusCode int, code string, allowLegacyFallback bool) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"error": map[string]any{
			"code":                code,
			"allowLegacyFallback": allowLegacyFallback,
		},
	})
}

func copySDKDResponse(responseWriter http.ResponseWriter, response *http.Response) {
	responseBody, errorValue := io.ReadAll(io.LimitReader(response.Body, sdkdMaximumBodyBytes+1))
	if errorValue != nil {
		writeSDKDBridgeUnavailable(responseWriter)
		return
	}
	if len(responseBody) > sdkdMaximumBodyBytes {
		writeSDKDBridgeInvalidResponse(responseWriter)
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
