package capabilityd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLocalStructuredCompletionUsesCPUAfterGPUFailure(t *testing.T) {
	var backends []string
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			document := string(standardInput)
			if strings.Contains(document, `"backend":"gpu"`) {
				backends = append(backends, "gpu")
				return nil, errors.New("gpu unavailable")
			}
			if strings.Contains(document, `"backend":"cpu"`) {
				backends = append(backends, "cpu")
				return []byte(`{"content":"{\"content\":\"ok\"}"}`), nil
			}
			t.Fatalf("unexpected wrapper request: %s", document)
			return nil, nil
		},
	}

	response, errorValue := service.completeStructured(context.Background(), llmRequest{
		ExecutionMode: "local",
		Messages:      []message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: schemaRequest{
			Name:     "plain_text_response",
			Document: `{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`,
		},
	})
	if errorValue != nil {
		t.Fatalf("expected local completion to succeed: %v", errorValue)
	}

	responseMap, isMap := response.(map[string]string)
	if !isMap {
		t.Fatalf("expected map response, got %T", response)
	}
	if responseMap["selectedBackend"] != "cpu" {
		t.Fatalf("expected cpu backend, got %q", responseMap["selectedBackend"])
	}
	if strings.Join(backends, ",") != "gpu,cpu" {
		t.Fatalf("expected gpu then cpu, got %v", backends)
	}
}

func TestLocalStructuredCompletionRejectsInvalidStructuredOutput(t *testing.T) {
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"{\"message\":\"wrong\"}"}`), nil
		},
	}

	_, errorValue := service.completeStructured(context.Background(), llmRequest{
		ExecutionMode: "local",
		StructuredOutputSchema: schemaRequest{
			Name:     "plain_text_response",
			Document: `{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`,
		},
	})
	if errorValue == nil {
		t.Fatalf("expected invalid structured output to fail")
	}
}
