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
	HTTPClient *http.Client
}

func (handler ShellBridgePromptHandler) Confirm(ctx context.Context, message string, defaultValue bool) (bool, error) {
	var response struct {
		Confirmed bool `json:"confirmed"`
	}
	if errorValue := handler.post(ctx, "/v1/user/confirm", map[string]any{
		"message": message,
		"default": defaultValue,
	}, &response); errorValue != nil {
		return false, errorValue
	}
	return response.Confirmed, nil
}

func (handler ShellBridgePromptHandler) Input(ctx context.Context, message string) (string, error) {
	var response struct {
		Text string `json:"text"`
	}
	if errorValue := handler.post(ctx, "/v1/user/input", map[string]string{"message": message}, &response); errorValue != nil {
		return "", errorValue
	}
	return response.Text, nil
}

func (handler ShellBridgePromptHandler) Approve(ctx context.Context, request ApprovalRequest) (ApprovalDecision, error) {
	var response ApprovalDecision
	if errorValue := handler.post(ctx, "/v1/security/approval", request, &response); errorValue != nil {
		return ApprovalDecision{}, errorValue
	}
	return response, nil
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
