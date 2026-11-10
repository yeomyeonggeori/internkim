package llmbackend

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRouterBackendNamesTheRequesterOnlyToTheGateway(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		namesRequester bool
		expected       string
	}{
		{name: "through the gateway", namesRequester: true, expected: "park.sample@example.com"},
		{name: "straight to OpenRouter", namesRequester: false, expected: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			apiKeyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
			if errorValue := os.WriteFile(apiKeyPath, []byte("sk-test"), 0o600); errorValue != nil {
				t.Fatal(errorValue)
			}
			received := "unset"
			backend := OpenRouterBackend{
				KeyPath:        apiKeyPath,
				BaseURL:        "https://gateway.test/api/v1/chat/completions",
				ModelName:      "configured-model",
				NamesRequester: testCase.namesRequester,
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					received = request.Header.Get(RequesterEmailHeader)
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
						Header:     make(http.Header),
					}, nil
				})},
			}

			_, errorValue := backend.CompleteText(context.Background(), TextRequest{
				Context:  RequestContext{RequesterEmail: "park.sample@example.com"},
				Messages: []Message{{Role: "user", Content: "hello"}},
			})

			if errorValue != nil {
				t.Fatalf("expected text response: %v", errorValue)
			}
			if received != testCase.expected {
				t.Fatalf("expected requester header %q, got %q", testCase.expected, received)
			}
		})
	}
}

func TestRequesterEmailHeaderIsTheOneTheModelGatewayReads(t *testing.T) {
	gateway, errorValue := os.ReadFile(filepath.Join("..", "..", "supabase", "functions", "model-gateway", "index.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expected := "'" + strings.ToLower(RequesterEmailHeader) + "'"
	if !strings.Contains(string(gateway), expected) {
		t.Fatalf("the model gateway no longer reads %s; the host and the gateway must name the requester with one header", expected)
	}
}
