package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func (service Service) invokeGoogleWorkspaceTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	webhookURL := readSecretValue(service.Configuration.GoogleWorkspaceWebhookPath)
	if strings.TrimSpace(webhookURL) == "" {
		return googleWorkspaceErrorResponse(request.ToolName, "Google Workspace credentials are not installed"), nil
	}
	form, errorValue := googleWorkspaceForm(request)
	if errorValue != nil {
		return googleWorkspaceErrorResponse(request.ToolName, errorValue.Error()), nil
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, strings.NewReader(form.Encode()))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpClient := service.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	httpResponse, errorValue := httpClient.Do(httpRequest)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return googleWorkspaceErrorResponse(request.ToolName, strings.TrimSpace(string(responseBody))), nil
	}
	return googleWorkspaceSuccessResponse(request.ToolName, responseBody), nil
}

func googleWorkspaceForm(request capabilities.ToolInvokeRequest) (url.Values, error) {
	action := strings.TrimPrefix(strings.TrimSpace(request.ToolName), "google.")
	if action == "" || action == request.ToolName {
		return nil, errors.New("invalid Google Workspace tool name")
	}
	input := map[string]any{}
	if len(request.Input) > 0 {
		if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
			return nil, errors.New("Google Workspace tool input is not valid JSON")
		}
	}
	form := url.Values{}
	form.Set("action", action)
	for key, value := range input {
		if key == "path" || key == "file" || key == "pptx" {
			data, errorValue := encodeGoogleWorkspaceFile(value)
			if errorValue != nil {
				return nil, errorValue
			}
			form.Set("data", data)
			continue
		}
		form.Set(key, googleWorkspaceValue(value))
	}
	return form, nil
}

func encodeGoogleWorkspaceFile(value any) (string, error) {
	path := strings.TrimSpace(googleWorkspaceValue(value))
	if path == "" {
		return "", errors.New("file path is required")
	}
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", errorValue
	}
	return base64.StdEncoding.EncodeToString(document), nil
}

func googleWorkspaceValue(value any) string {
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case nil:
		return ""
	default:
		document, errorValue := json.Marshal(typedValue)
		if errorValue != nil {
			return ""
		}
		return string(document)
	}
}

func googleWorkspaceSuccessResponse(toolName string, responseBody []byte) capabilities.ToolInvokeResponse {
	trimmedBody := bytes.TrimSpace(responseBody)
	result := json.RawMessage(trimmedBody)
	if !json.Valid(result) {
		result, _ = json.Marshal(map[string]string{"content": string(trimmedBody)})
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "capabilityd",
		SelectedBackend: "google_workspace",
		ToolName:        toolName,
		Status:          "ok",
		Content:         string(trimmedBody),
		Result:          result,
	}
}

func googleWorkspaceErrorResponse(toolName string, message string) capabilities.ToolInvokeResponse {
	trimmedMessage := strings.TrimSpace(message)
	if trimmedMessage == "" {
		trimmedMessage = "Google Workspace tool failed"
	}
	result, _ := json.Marshal(map[string]string{"error": trimmedMessage})
	return capabilities.ToolInvokeResponse{
		Provider:        "capabilityd",
		SelectedBackend: "google_workspace",
		ToolName:        toolName,
		Status:          "error",
		Content:         trimmedMessage,
		IsError:         true,
		Result:          result,
	}
}
