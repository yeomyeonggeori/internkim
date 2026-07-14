package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const sdkdMaximumBodyBytes = 8 * 1024 * 1024

var errSDKDRequestTooLarge = errors.New("SDKD request exceeds 8 MiB")

func (service Service) handleSDKDStructured(responseWriter http.ResponseWriter, request *http.Request) {
	requestBody, errorValue := readSDKDRequestBody(request)
	if errorValue != nil {
		if errors.Is(errorValue, errSDKDRequestTooLarge) {
			writeSDKDRequestTooLarge(responseWriter)
			return
		}
		writeSDKDBridgeUnavailable(responseWriter)
		return
	}
	proxyRequest, errorValue := service.newSDKDRequest(request, requestBody)
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

func (service Service) newSDKDRequest(request *http.Request, requestBody []byte) (*http.Request, error) {
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
		"http://blueclaw-sdkd/v1/llm/structured",
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
	timeout := service.httpClientTimeout()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", service.Configuration.SDKDSocketPath)
		},
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
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
