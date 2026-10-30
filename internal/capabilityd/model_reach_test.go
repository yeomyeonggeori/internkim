package capabilityd

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const webSearchAnswer = `{"choices":[{"message":{"content":"{\"provider\":\"openrouter\",\"remoteLLMInvolved\":true,\"compatibility\":\"openrouter_server_tool_auto\",\"query\":\"internkim\",\"answer\":\"result\",\"results\":[]}"}}]}`

func TestACompanyWithoutItsOwnKeyAsksTheGatewayWithThisComputersCredential(t *testing.T) {
	directory := t.TempDir()
	hostSessionPath := filepath.Join(directory, "agent-key")
	writeFileForModelReachTest(t, hostSessionPath, "host-session")
	askedURL, authorization := askedByAWebSearch(t, Configuration{
		OpenRouterKeyPath: filepath.Join(directory, "openrouter-key"),
		HostSessionPath:   hostSessionPath,
		CentralPlaneURL:   "https://plane.example",
	})
	if askedURL != "https://plane.example/functions/v1/model-gateway/api/v1/chat/completions" || authorization != "Bearer host-session" {
		t.Fatalf("a company that brought no key must ask the model gateway as this computer; asked %s with %q", askedURL, authorization)
	}
}

func TestACompanyWithItsOwnKeyAsksOpenRouterWithIt(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "openrouter-key")
	writeFileForModelReachTest(t, keyPath, "sk-own")
	askedURL, authorization := askedByAWebSearch(t, Configuration{
		OpenRouterKeyPath: keyPath,
		HostSessionPath:   filepath.Join(directory, "agent-key"),
		CentralPlaneURL:   "https://plane.example",
	})
	if askedURL != DefaultConfiguration().OpenRouterWebBaseURL || authorization != "Bearer sk-own" {
		t.Fatalf("a company that brought its own key must ask OpenRouter with it; asked %s with %q", askedURL, authorization)
	}
}

func askedByAWebSearch(t *testing.T, configuration Configuration) (string, string) {
	t.Helper()
	var askedURL, authorization string
	service := Service{
		Configuration: configuration.WithDefaults().reachingModels(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			askedURL, authorization = request.URL.String(), request.Header.Get("Authorization")
			return jsonResponseBody(webSearchAnswer), nil
		})},
	}
	if _, errorValue := service.invokeCapabilityTool(context.Background(), "web_search", strings.NewReader(`{"input":{"query":"internkim","limit":3}}`)); errorValue != nil {
		t.Fatal(errorValue)
	}
	return askedURL, authorization
}

func writeFileForModelReachTest(t *testing.T, path string, contents string) {
	t.Helper()
	if errorValue := os.WriteFile(path, []byte(contents+"\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}
