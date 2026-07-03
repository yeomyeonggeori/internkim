package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const maximumImageGeneratePromptLength = 4000

var allowedImageGenerateAspectRatios = []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}

type imageGenerateInput struct {
	Prompt      string `json:"prompt"`
	Path        string `json:"path"`
	AspectRatio string `json:"aspectRatio"`
}

func (service Service) invokeImageGenerateTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if service.Configuration.LocalOnly {
		return imageGenerateErrorResponse(request.ToolName, "remote image generation is disabled in local-only mode", "local_only", false), nil
	}
	input, errorValue := decodeImageGenerateInput(request.Input)
	if errorValue != nil {
		return imageGenerateErrorResponse(request.ToolName, errorValue.Error(), "invalid_input", false), nil
	}
	hostPath, agentPath, errorValue := service.resolveImageGeneratePath(input.Path)
	if errorValue != nil {
		return imageGenerateErrorResponse(request.ToolName, errorValue.Error(), "invalid_workspace_path", false), nil
	}
	apiKey := readSecretValue(service.Configuration.OpenRouterKeyPath)
	if strings.TrimSpace(apiKey) == "" || isPlaceholderOpenRouterKey(apiKey) {
		return imageGenerateErrorResponse(request.ToolName, "OpenRouter API key is not configured", "missing_openrouter_key", false), nil
	}
	imageBytes, errorValue := service.generateOpenRouterImage(ctx, input, apiKey)
	if errorValue != nil {
		return imageGenerateErrorResponse(request.ToolName, errorValue.Error(), "image_generation_failed", true), nil
	}
	if errorValue := os.MkdirAll(filepath.Dir(hostPath), 0755); errorValue != nil {
		return imageGenerateErrorResponse(request.ToolName, errorValue.Error(), "image_write_failed", true), nil
	}
	if errorValue := os.WriteFile(hostPath, imageBytes, 0644); errorValue != nil {
		return imageGenerateErrorResponse(request.ToolName, errorValue.Error(), "image_write_failed", true), nil
	}
	result := map[string]any{
		"status": "ok",
		"path":   agentPath,
		"attachments": []map[string]any{{
			"devicePath":    agentPath,
			"filename":      filepath.Base(agentPath),
			"contentType":   "image/png",
			"sizeBytes":     len(imageBytes),
			"contentBase64": base64.StdEncoding.EncodeToString(imageBytes),
		}},
	}
	resultDocument, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "openrouter",
		SelectedBackend: capabilities.LLMBackendRemote,
		ToolName:        request.ToolName,
		Status:          "ok",
		Content:         "image generated",
		Result:          resultDocument,
	}, nil
}

func decodeImageGenerateInput(document json.RawMessage) (imageGenerateInput, error) {
	var input imageGenerateInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return imageGenerateInput{}, errorValue
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Path = strings.TrimSpace(input.Path)
	input.AspectRatio = strings.TrimSpace(input.AspectRatio)
	if input.Prompt == "" {
		return imageGenerateInput{}, errors.New("prompt is required")
	}
	if len(input.Prompt) > maximumImageGeneratePromptLength {
		return imageGenerateInput{}, fmt.Errorf("prompt must be at most %d characters", maximumImageGeneratePromptLength)
	}
	if input.Path == "" {
		return imageGenerateInput{}, errors.New("path is required")
	}
	if input.AspectRatio == "" {
		input.AspectRatio = "1:1"
	}
	if !isAllowedImageGenerateAspectRatio(input.AspectRatio) {
		return imageGenerateInput{}, fmt.Errorf("aspectRatio must be one of %s", strings.Join(allowedImageGenerateAspectRatios, ", "))
	}
	return input, nil
}

func isAllowedImageGenerateAspectRatio(aspectRatio string) bool {
	for _, allowedValue := range allowedImageGenerateAspectRatios {
		if aspectRatio == allowedValue {
			return true
		}
	}
	return false
}

func (service Service) resolveImageGeneratePath(path string) (string, string, error) {
	agentPath, errorValue := cleanFileReadAgentPath(path)
	if errorValue != nil {
		return "", "", errorValue
	}
	if !strings.HasSuffix(strings.ToLower(agentPath), ".png") {
		return "", "", errors.New("path must end in .png")
	}
	workspacePath := service.Configuration.WithDefaults().BlueclawWorkspacePath
	hostPath := filepath.Join(workspacePath, strings.TrimPrefix(agentPath, "/workspace/"))
	hostPath, errorValue = cleanHostWorkspacePath(workspacePath, hostPath)
	if errorValue != nil {
		return "", "", errorValue
	}
	return hostPath, agentPath, nil
}

func (service Service) generateOpenRouterImage(ctx context.Context, input imageGenerateInput, apiKey string) ([]byte, error) {
	requestBody, errorValue := json.Marshal(map[string]any{
		"model":      service.Configuration.OpenRouterImageModel,
		"messages":   []map[string]any{{"role": "user", "content": input.Prompt}},
		"modalities": []string{"image", "text"},
		"image_config": map[string]any{
			"aspect_ratio": input.AspectRatio,
		},
	})
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.Configuration.OpenRouterBaseURL, bytes.NewReader(requestBody))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return nil, errors.New("read openrouter image response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(safeProviderError(responseBody))
	}
	return decodeOpenRouterImageResponse(responseBody)
}

func decodeOpenRouterImageResponse(responseBody []byte) ([]byte, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				Images  []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
	}
	if errorValue := json.Unmarshal(responseBody, &parsed); errorValue != nil {
		return nil, errorValue
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("openrouter image response did not include choices")
	}
	message := parsed.Choices[0].Message
	for _, image := range message.Images {
		if encodedImage := extractBase64FromDataURL(image.ImageURL.URL); encodedImage != "" {
			return base64.StdEncoding.DecodeString(encodedImage)
		}
	}
	if encodedImage := extractBase64FromDataURL(message.Content); encodedImage != "" {
		return base64.StdEncoding.DecodeString(encodedImage)
	}
	hint := strings.TrimSpace(message.Content)
	if len(hint) > 200 {
		hint = hint[:200]
	}
	if hint != "" {
		return nil, fmt.Errorf("openrouter image response did not include image data: %s", hint)
	}
	return nil, errors.New("openrouter image response did not include image data")
}

func extractBase64FromDataURL(value string) string {
	markerIndex := strings.Index(value, "data:image/")
	if markerIndex < 0 {
		return ""
	}
	remainder := value[markerIndex:]
	commaIndex := strings.Index(remainder, ",")
	if commaIndex < 0 {
		return ""
	}
	encodedImage := remainder[commaIndex+1:]
	if endIndex := strings.IndexAny(encodedImage, "\"' \n\r\t"); endIndex >= 0 {
		encodedImage = encodedImage[:endIndex]
	}
	return strings.TrimSpace(encodedImage)
}

func imageGenerateErrorResponse(toolName string, message string, code string, retryable bool) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(map[string]any{
		"error":    message,
		"code":     code,
		"provider": "openrouter",
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "openrouter",
		SelectedBackend: capabilities.LLMBackendRemote,
		ToolName:        toolName,
		Status:          "error",
		Content:         message,
		IsError:         true,
		ErrorCode:       code,
		Retryable:       retryable,
		SafeRetry:       retryable,
		Result:          result,
	}
}
