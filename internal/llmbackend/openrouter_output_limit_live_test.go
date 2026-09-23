//go:build llmeval

package llmbackend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/modelladder"
)

func TestOpenRouterLiveSyntheticTruncationUsesActualFallback(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	primaryModel := backend.ModelName
	fallbackModel, isFallbackAvailable := firstLiveFallbackModel(primaryModel)
	if !isFallbackAvailable {
		t.Fatalf("no canonical fallback model is available for primary model %q", primaryModel)
	}
	backend.FallbackModelNames = []string{fallbackModel}
	evidenceDirectory := strings.TrimSpace(os.Getenv("INTERNKIM_LIVE_ARTIFACT_DIR"))
	if evidenceDirectory == "" {
		t.Skip("INTERNKIM_LIVE_ARTIFACT_DIR is required for live truncation evidence")
	}

	baseTransport := backend.HTTPClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	injectedClient := &http.Client{Timeout: backend.HTTPClient.Timeout, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			return baseTransport.RoundTrip(request)
		}
		body, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			return nil, errorValue
		}
		_ = request.Body.Close()
		request.Body = io.NopCloser(strings.NewReader(string(body)))
		var requestDocument struct {
			Model string `json:"model"`
		}
		if errorValue := json.Unmarshal(body, &requestDocument); errorValue != nil {
			return nil, errorValue
		}
		if requestDocument.Model == primaryModel {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"length","message":{"content":""}}],"usage":{"prompt_tokens":24,"completion_tokens":1600,"completion_tokens_details":{"reasoning_tokens":1600}}}`)),
				Header:     make(http.Header),
			}, nil
		}
		return baseTransport.RoundTrip(request)
	})}
	capturedClient, capture := NewExchangeCapture(injectedClient)
	backend.HTTPClient = capturedClient
	request := buildLiveTruncationRequest()
	t.Cleanup(func() {
		identifier, errorValue := WriteFailureEvidence(evidenceDirectory, request, capture)
		if errorValue != nil {
			t.Errorf("write live truncation evidence: %v", errorValue)
			return
		}
		t.Logf("synthetic primary truncation and actual fallback evidence: %s/%s.json", FailureEvidenceDirectory(evidenceDirectory), identifier)
	})

	response, errorValue := backend.CompleteStructured(context.Background(), request)
	if errorValue != nil {
		t.Fatalf("expected actual fallback model to succeed after injected truncation: %v", errorValue)
	}
	if response.Model != fallbackModel || !response.UsedFallback || strings.TrimSpace(response.Content) == "" {
		t.Fatalf("expected nonempty response from actual fallback model %q, got %+v", fallbackModel, response)
	}
	var decision struct {
		Route string `json:"route"`
	}
	if errorValue := json.Unmarshal([]byte(response.Content), &decision); errorValue != nil {
		t.Fatalf("expected fallback response JSON, got %q: %v", response.Content, errorValue)
	}
	if decision.Route != "clarify" {
		t.Fatalf("expected fallback model to classify ambiguous request as clarify, got %q", decision.Route)
	}
}

func buildLiveTruncationRequest() StructuredRequest {
	return StructuredRequest{
		Model:    "",
		Messages: []Message{{Role: "user", Content: "다음 목록은 이름과 번호만 있습니다. 무엇을 하려는지 명확하지 않으니 의도를 분류해 주세요. 이샘플, 박예시"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "router",
			Document:           []byte(`{"type":"object","properties":{"route":{"type":"string","enum":["clarify","execute"]},"question":{"type":"string"},"instructions":{"type":"string"}},"required":["route","question","instructions"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
		GenerationOptions: &GenerationOptions{MaxTokens: intPointer(1600)},
	}
}

func firstLiveFallbackModel(primaryModel string) (string, bool) {
	for _, modelName := range modelladder.DegradedModels {
		if modelName != primaryModel {
			return modelName, true
		}
	}
	return "", false
}
