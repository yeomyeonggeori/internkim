package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestArtifactReviewUsesOpenRouterMultimodalMessage(t *testing.T) {
	workspacePath := t.TempDir()
	imagePath := filepath.Join(workspacePath, "tmp", "site", "desktop.png")
	if errorValue := os.MkdirAll(filepath.Dir(imagePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(imagePath, []byte("image"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	var requestBody map[string]any
	service := Service{
		Configuration: Configuration{
			BlueclawWorkspacePath: workspacePath,
			OpenRouterKeyPath:     writeOpenRouterSecretForWebToolTest(t, "sk-review"),
			OpenRouterBaseURL:     "https://openrouter.test/chat/completions",
			OpenRouterModel:       "openrouter/vision-model",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "https://openrouter.test/chat/completions" {
				t.Fatalf("unexpected request URL %s", request.URL.String())
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponseBody(`{"choices":[{"message":{"content":"{\"passed\":true,\"issues\":[],\"acceptedWarnings\":[],\"summary\":\"ok\"}"}}]}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "artifact.review", strings.NewReader(`{"input":{"artifactKind":"site","intent":"booking flow","rubric":"check layout","evidence":[{"role":"desktopScreenshot","path":"tmp/site/desktop.png","mimeType":"image/png","label":"desktop"}]}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError || response.Provider != "openrouter" || response.SelectedBackend != "remote" || response.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("unexpected artifact review response: %+v", response)
	}

	messages := requestBody["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	imagePart := content[1].(map[string]any)["image_url"].(map[string]any)
	if imagePart["url"] != "data:image/png;base64,aW1hZ2U=" {
		t.Fatalf("expected image data URL, got %#v", imagePart)
	}
}

func TestArtifactReviewRejectsBlueclawInternalPath(t *testing.T) {
	service := Service{Configuration: Configuration{BlueclawWorkspacePath: t.TempDir()}}
	response, errorValue := service.invokeCapabilityTool(context.Background(), "artifact.review", strings.NewReader(`{"input":{"artifactKind":"site","intent":"booking flow","rubric":"check layout","evidence":[{"role":"desktopScreenshot","path":"/workspace/.blueclaw/secret.png","mimeType":"image/png","label":"desktop"}]}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.FailureStage != "evidence_loading" || response.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("expected evidence loading error, got %+v", response)
	}
}
