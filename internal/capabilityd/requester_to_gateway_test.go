package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

func TestADecisionThroughTheGatewayNamesWhoAskedForIt(t *testing.T) {
	received := ""
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		received = request.Header.Get(llmbackend.RequesterEmailHeader)
		_, _ = io.Copy(io.Discard, request.Body)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(responseWriter, decideAnswerDocument)
	}))
	t.Cleanup(upstreamServer.Close)
	keyPath := filepath.Join(t.TempDir(), "host-session")
	if errorValue := os.WriteFile(keyPath, []byte("session"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.OpenRouterKeyPath = keyPath
	configuration.OpenRouterBaseURL = upstreamServer.URL + "/functions/v1/model-gateway/api/v1/chat/completions"
	configuration.ModelGatewayNamesRequester = true
	service := Service{Configuration: configuration, HTTPClient: upstreamServer.Client()}
	requestBody := strings.Replace(decideRequestBody, `"sessionID": "session-1"`, `"sessionID": "session-1", "context": {"requesterEmail": "park.sample@example.com"}`, 1)

	responseRecorder := decide(t, service, requestBody)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected the decision to be answered, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if received != "park.sample@example.com" {
		t.Fatalf("a decision made for a person must name them to the gateway, got %q", received)
	}
}

func TestOCRNamesWhoAskedForItOnlyToTheGateway(t *testing.T) {
	for _, testCase := range []struct {
		name            string
		namesRequester  bool
		expectedHeaders map[string]string
	}{
		{name: "through the gateway", namesRequester: true, expectedHeaders: map[string]string{llmbackend.RequesterEmailHeader: "park.sample@example.com"}},
		{name: "straight to OpenRouter", namesRequester: false, expectedHeaders: nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			workspacePath := t.TempDir()
			writeFileReadTestFile(t, filepath.Join(workspacePath, "scan.pdf"), "pdf")
			configuration := Configuration{
				OpenRouterKeyPath:     writeOpenRouterSecretForWebToolTest(t, "sk-file"),
				OpenRouterBaseURL:     "https://gateway.test/api/v1/chat/completions",
				OpenRouterModel:       "openrouter/vision-model",
				BlueclawWorkspacePath: workspacePath,
				FileReadPythonPath:    "/test/python",
			}.WithDefaults()
			configuration.ModelGatewayNamesRequester = testCase.namesRequester
			helperRequests := []fileReadHelperRequest{}
			service := Service{
				Configuration: configuration,
				RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
					var helperRequest fileReadHelperRequest
					if errorValue := json.Unmarshal(input, &helperRequest); errorValue != nil {
						t.Fatal(errorValue)
					}
					helperRequests = append(helperRequests, helperRequest)
					if helperRequest.OCRMode == "never" {
						return []byte(`{"content":"","errorCode":"ocr_required","message":"PDF has no extractable text"}`), nil
					}
					return []byte(`{"content":"scanned text"}`), nil
				},
			}
			body := carryingTheFile(t, workspacePath, `{"input":{"path":"/workspace/scan.pdf"},"context":{"requesterEmail":"park.sample@example.com"}}`)

			response, errorValue := service.invokeCapabilityTool(context.Background(), "document_read", strings.NewReader(body))

			if errorValue != nil || response.IsError || len(helperRequests) != 2 {
				t.Fatalf("expected an OCR read, got %+v, %v", response, errorValue)
			}
			if len(helperRequests[1].OpenRouterHeaders) != len(testCase.expectedHeaders) {
				t.Fatalf("expected OCR headers %v, got %v", testCase.expectedHeaders, helperRequests[1].OpenRouterHeaders)
			}
			for name, value := range testCase.expectedHeaders {
				if helperRequests[1].OpenRouterHeaders[name] != value {
					t.Fatalf("expected OCR headers %v, got %v", testCase.expectedHeaders, helperRequests[1].OpenRouterHeaders)
				}
			}
		})
	}
}
