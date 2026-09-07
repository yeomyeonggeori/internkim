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

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func TestStructuredLLMFailurePersistsProviderExchanges(t *testing.T) {
	responses := []string{
		`{"choices":[{"message":{"content":""}}]}`,
		`{"choices":[{"message":{"content":""}}]}`,
		`invalid provider response`,
	}
	requestBodies := []string{}
	providerServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestBody, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		requestBodies = append(requestBodies, string(requestBody))
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(responseWriter, responses[len(requestBodies)-1])
	}))
	defer providerServer.Close()

	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("test-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	workspacePath := t.TempDir()
	configuration := DefaultConfiguration()
	configuration.OpenRouterKeyPath = secretPath
	configuration.OpenRouterBaseURL = providerServer.URL
	configuration.OpenRouterModel = "router-model"
	configuration.ForceOpenRouterModel = true
	configuration.BlueclawWorkspacePath = workspacePath
	service := Service{Configuration: configuration, HTTPClient: providerServer.Client()}

	requestBody := `{
		"model":"router-model",
		"executionMode":"remote",
		"messages":[{"role":"user","content":"hello"}],
		"structuredOutputSchema":{"name":"reply","document":{"type":"object"},"isStrictlyEnforced":true}
	}`
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/structured", strings.NewReader(requestBody))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected provider failure, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if len(requestBodies) != 3 {
		t.Fatalf("expected schema, prompted-json, and retry requests, got %d", len(requestBodies))
	}
	if !strings.Contains(responseRecorder.Body.String(), "/admin/api/diagnostics/llm-failure?id=") {
		t.Fatalf("expected evidence link in failure, got %s", responseRecorder.Body.String())
	}

	evidenceFiles, errorValue := filepath.Glob(filepath.Join(llmbackend.FailureEvidenceDirectory(workspacePath), "*.json"))
	if errorValue != nil || len(evidenceFiles) != 1 {
		t.Fatalf("expected one evidence file, files=%v error=%v", evidenceFiles, errorValue)
	}
	document, errorValue := os.ReadFile(evidenceFiles[0])
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var evidence struct {
		Exchanges llmbackend.FailureCaptureSnapshot `json:"exchanges"`
	}
	if errorValue := json.Unmarshal(document, &evidence); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(evidence.Exchanges.Attempts) != 3 {
		t.Fatalf("expected three captured exchanges, got %+v", evidence.Exchanges)
	}
	for index, attempt := range evidence.Exchanges.Attempts {
		if attempt.RequestBody != requestBodies[index] {
			t.Fatalf("exchange %d did not preserve exact request body", index)
		}
		if attempt.ResponseBody != responses[index] {
			t.Fatalf("exchange %d did not preserve exact response body, got %q", index, attempt.ResponseBody)
		}
		if !attempt.ResponseComplete {
			t.Fatalf("exchange %d response was not marked complete", index)
		}
	}
	if strings.Contains(string(document), "test-key") {
		t.Fatal("evidence artifact contains the provider secret")
	}
	var firstRequest map[string]any
	var secondRequest map[string]any
	var thirdRequest map[string]any
	for index, target := range []*map[string]any{&firstRequest, &secondRequest, &thirdRequest} {
		if errorValue := json.Unmarshal([]byte(requestBodies[index]), target); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if firstRequest["response_format"] == nil || secondRequest["response_format"] != nil || thirdRequest["response_format"] == nil {
		t.Fatalf("expected schema, prompted-json, schema request modes: %v, %v, %v", firstRequest, secondRequest, thirdRequest)
	}
}

func TestStructuredLLMSuccessDoesNotWriteFailureEvidence(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(responseWriter, `{"choices":[{"message":{"content":"{\"reply\":\"ok\"}"}}]}`)
	}))
	defer providerServer.Close()

	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("test-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	workspacePath := t.TempDir()
	configuration := DefaultConfiguration()
	configuration.OpenRouterKeyPath = secretPath
	configuration.OpenRouterBaseURL = providerServer.URL
	configuration.BlueclawWorkspacePath = workspacePath
	service := Service{Configuration: configuration, HTTPClient: providerServer.Client()}
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/structured", strings.NewReader(`{
		"model":"router-model","executionMode":"remote","messages":[],
		"structuredOutputSchema":{"name":"reply","document":{"type":"object"}}
	}`))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected success, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	evidenceFiles, errorValue := filepath.Glob(filepath.Join(llmbackend.FailureEvidenceDirectory(workspacePath), "*.json"))
	if errorValue != nil || len(evidenceFiles) != 0 {
		t.Fatalf("expected no evidence files, files=%v error=%v", evidenceFiles, errorValue)
	}
}
