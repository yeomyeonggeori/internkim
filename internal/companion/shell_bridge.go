package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ShellBridgePromptHandler struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (handler ShellBridgePromptHandler) post(ctx context.Context, path string, requestBody any, responseBody any) error {
	baseURL, errorValue := validateShellBridgeURL(handler.BaseURL)
	if errorValue != nil {
		return errorValue
	}
	document, errorValue := json.Marshal(requestBody)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(handler.Token) != "" {
		request.Header.Set("X-InternKim-Shell-Bridge-Token", strings.TrimSpace(handler.Token))
	}
	httpClient := handler.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, errorValue := httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("companion shell bridge returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(responseBody)
}

func validateShellBridgeURL(value string) (string, error) {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(value))
	if errorValue != nil {
		return "", errorValue
	}
	if parsedURL.Scheme != "http" {
		return "", errors.New("companion shell bridge must use local http")
	}
	if parsedURL.Hostname() != "127.0.0.1" && parsedURL.Hostname() != "localhost" {
		return "", errors.New("companion shell bridge must be local")
	}
	if parsedURL.Port() == "" {
		return "", errors.New("companion shell bridge port is required")
	}
	return strings.TrimRight(parsedURL.String(), "/"), nil
}
