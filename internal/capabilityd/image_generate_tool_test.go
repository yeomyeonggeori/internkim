package capabilityd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestImageGenerateRefusalsCarryAFailedOutcome(t *testing.T) {
	workspacePath := t.TempDir()
	missingKeyPath := filepath.Join(t.TempDir(), "missing-openrouter-key")
	cases := []struct {
		name          string
		configuration Configuration
		input         string
		code          string
	}{
		{name: "local only", configuration: Configuration{LocalOnly: true}, input: `{"prompt":"logo","path":"/workspace/shared/logo.png"}`, code: "local_only"},
		{name: "no prompt", configuration: Configuration{}, input: `{"path":"/workspace/shared/logo.png"}`, code: "invalid_input"},
		{name: "not a png", configuration: Configuration{BlueclawWorkspacePath: workspacePath}, input: `{"prompt":"logo","path":"/workspace/shared/logo.jpg"}`, code: "invalid_workspace_path"},
		{name: "no key", configuration: Configuration{BlueclawWorkspacePath: workspacePath, OpenRouterKeyPath: missingKeyPath}, input: `{"prompt":"logo","path":"/workspace/shared/logo.png"}`, code: "missing_openrouter_key"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := Service{Configuration: testCase.configuration}
			response, errorValue := service.invokeImageGenerateTool(context.Background(), capabilities.ToolInvokeRequest{
				ToolName: "image_generate",
				Input:    json.RawMessage(testCase.input),
			})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != testCase.code {
				t.Fatalf("response = %+v", response)
			}
		})
	}
	if _, errorValue := os.Stat(filepath.Join(workspacePath, "shared", "logo.png")); !os.IsNotExist(errorValue) {
		t.Fatalf("a refused generation wrote an image: %v", errorValue)
	}
}
