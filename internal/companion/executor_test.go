package companion

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

type fakeBrowserOpener struct {
	openedURLs []string
}

func (opener *fakeBrowserOpener) OpenBrowser(ctx context.Context, targetURL string) error {
	_ = ctx
	opener.openedURLs = append(opener.openedURLs, targetURL)
	return nil
}

func TestBrowserNavigateOpensValidatedURL(t *testing.T) {
	opener := &fakeBrowserOpener{}
	executor := Executor{BrowserOpener: opener}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.navigate",
		Input:    json.RawMessage(`{"url":"https://example.com/path"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected browser navigate success: %v", errorValue)
	}
	if response.ToolName != "browser.navigate" || len(opener.openedURLs) != 1 || opener.openedURLs[0] != "https://example.com/path" {
		t.Fatalf("unexpected browser result: response=%+v urls=%v", response, opener.openedURLs)
	}
}

func TestBrowserNavigateRejectsNonHTTPURL(t *testing.T) {
	opener := &fakeBrowserOpener{}
	executor := Executor{BrowserOpener: opener}

	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.navigate",
		Input:    json.RawMessage(`{"url":"file:///etc/passwd"}`),
	})
	if errorValue == nil {
		t.Fatal("expected non-http browser URL to fail")
	}
	if len(opener.openedURLs) != 0 {
		t.Fatalf("expected browser not to open, got %v", opener.openedURLs)
	}
}

func TestUserConfirmUsesPromptHandler(t *testing.T) {
	executor := Executor{
		PromptHandler: TerminalPromptHandler{
			Reader: strings.NewReader("yes\n"),
			Writer: &strings.Builder{},
		},
	}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "user.confirm",
		Input:    json.RawMessage(`{"message":"continue?"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected confirm success: %v", errorValue)
	}
	var result struct {
		Confirmed bool `json:"confirmed"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.Confirmed {
		t.Fatal("expected confirmation to be true")
	}
}

func TestUserConfirmWithoutPromptHandlerFailsSafely(t *testing.T) {
	executor := Executor{}

	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{ToolName: "user.confirm"})
	if errorValue == nil {
		t.Fatal("expected missing prompt handler to fail")
	}
	if strings.Contains(errorValue.Error(), "token") || strings.Contains(errorValue.Error(), "secret") {
		t.Fatalf("unexpected sensitive error: %v", errorValue)
	}
}

func TestShellBridgePromptHandlerConfirm(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/user/confirm" {
			t.Fatalf("unexpected bridge path: %s", request.URL.Path)
		}
		return textResponse(http.StatusOK, `{"confirmed":true}`), nil
	})}
	handler := ShellBridgePromptHandler{
		BaseURL:    "http://127.0.0.1:1234",
		HTTPClient: httpClient,
	}

	confirmed, errorValue := handler.Confirm(context.Background(), "continue?", false)
	if errorValue != nil {
		t.Fatalf("expected shell bridge confirm success: %v", errorValue)
	}
	if !confirmed {
		t.Fatal("expected shell bridge confirmation")
	}
}

func TestShellBridgePromptHandlerInput(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/user/input" {
			t.Fatalf("unexpected bridge path: %s", request.URL.Path)
		}
		return textResponse(http.StatusOK, `{"text":"approved text"}`), nil
	})}
	handler := ShellBridgePromptHandler{
		BaseURL:    "http://127.0.0.1:1234",
		HTTPClient: httpClient,
	}

	text, errorValue := handler.Input(context.Background(), "value?")
	if errorValue != nil {
		t.Fatalf("expected shell bridge input success: %v", errorValue)
	}
	if text != "approved text" {
		t.Fatalf("unexpected shell bridge input: %s", text)
	}
}

func TestShellBridgePromptHandlerRejectsNonLocalURL(t *testing.T) {
	handler := ShellBridgePromptHandler{BaseURL: "https://device.example.test"}

	_, errorValue := handler.Confirm(context.Background(), "continue?", false)
	if errorValue == nil {
		t.Fatal("expected non-local bridge to fail")
	}
	if strings.Contains(errorValue.Error(), "token") || strings.Contains(errorValue.Error(), "secret") {
		t.Fatalf("unexpected sensitive error: %v", errorValue)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func textResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestMockStructuredLLMUsesSchemaRequiredKeys(t *testing.T) {
	executor := Executor{DevMockLLM: true}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "llm.structured",
		Input:    json.RawMessage(`{"structuredOutputSchema":{"document":{"required":["reply"]}}}`),
	})
	if errorValue != nil {
		t.Fatalf("expected mock structured success: %v", errorValue)
	}
	var llmResponse struct {
		Content string `json:"content"`
	}
	if errorValue := json.Unmarshal(response.Result, &llmResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if llmResponse.Content != `{"reply":"ok"}` {
		t.Fatalf("unexpected mock content: %s", llmResponse.Content)
	}
}
