package capabilityd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/modelladder"
)

const decideRequestBody = `{
	"state": {"newestMessage": {"sender": "이샘플", "text": "응 진행해"}},
	"questions": {
		"shouldRespond": {"type": "noul", "instructions": "Should the assistant reply?"}
	},
	"sessionID": "session-1"
}`

const decideAnswerDocument = `{
  "id": "gen-dec-1789697042-s7VZqLokxV3PsVjUcjjQ",
  "model": "typesafe/jev-1.13-20260917",
  "provider": "TypeSafe",
  "answers": {"shouldRespond": {"type": "noul", "noul": 0.73}},
  "usage": {"input_tokens": 2285, "output_tokens": 826, "cost": 9.597e-05}
}`

func decideService(t *testing.T, statusCode int, responseBody string) (Service, *string) {
	t.Helper()
	upstreamRequestBody := ""
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		document, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			t.Error(errorValue)
			return
		}
		upstreamRequestBody = string(document)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(statusCode)
		_, _ = io.WriteString(responseWriter, responseBody)
	}))
	t.Cleanup(upstreamServer.Close)

	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte("test-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.OpenRouterKeyPath = keyPath
	configuration.OpenRouterBaseURL = upstreamServer.URL + "/api/v1/chat/completions"
	configuration.OpenRouterModel = "forced-chat-model"
	configuration.ForceOpenRouterModel = true
	return Service{Configuration: configuration, HTTPClient: upstreamServer.Client()}, &upstreamRequestBody
}

func decide(t *testing.T, service Service, requestBody string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/decide", strings.NewReader(requestBody))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func TestDecideAnswersWithTheDecisionModelsAnswersUsageAndLatency(t *testing.T) {
	service, upstreamRequestBody := decideService(t, http.StatusOK, decideAnswerDocument)

	responseRecorder := decide(t, service, decideRequestBody)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected the decision to be answered, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response DecideLLMResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	noul, isNoul := response.Answers["shouldRespond"].AsNoul()
	if !isNoul || noul.Noul != 0.73 {
		t.Fatalf("the answer did not reach the caller: %s", responseRecorder.Body.String())
	}
	if response.ModelName != "typesafe/jev-1.13-20260917" || response.ProviderName != "openrouter" || response.UpstreamProvider != "TypeSafe" {
		t.Fatalf("the answering model and providers were lost: %+v", response)
	}
	if response.Usage.InputTokens != 2285 || response.Usage.OutputTokens != 826 || response.Usage.CostUSD != 9.597e-05 {
		t.Fatalf("usage did not reach the caller: %+v", response.Usage)
	}
	if response.LatencyMilliseconds < 0 {
		t.Fatalf("latency was not measured: %+v", response)
	}
	if !strings.Contains(*upstreamRequestBody, `"model":"`+modelladder.DecisionModel+`"`) {
		t.Fatalf("a forced chat model displaced the decision model: %s", *upstreamRequestBody)
	}
}

func TestDecideRefusesWhenTheDecisionsRouteRefuses(t *testing.T) {
	service, _ := decideService(t, http.StatusNotFound, `{"error":{"message":"No endpoints found","code":404}}`)

	responseRecorder := decide(t, service, decideRequestBody)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected the upstream refusal to be reported, got %d", responseRecorder.Code)
	}
	if !strings.Contains(responseRecorder.Body.String(), "404") || !strings.Contains(responseRecorder.Body.String(), "No endpoints found") {
		t.Fatalf("the refusal hid the upstream status or body: %s", responseRecorder.Body.String())
	}
}

func TestDecideRefusesInLocalOnlyMode(t *testing.T) {
	service, _ := decideService(t, http.StatusOK, decideAnswerDocument)
	service.Configuration.LocalOnly = true

	responseRecorder := decide(t, service, decideRequestBody)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected local-only mode to refuse the decision, got %d", responseRecorder.Code)
	}
	if !strings.Contains(responseRecorder.Body.String(), "local-only") {
		t.Fatalf("the refusal did not say why: %s", responseRecorder.Body.String())
	}
}
