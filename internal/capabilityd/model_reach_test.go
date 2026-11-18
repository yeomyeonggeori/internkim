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

func TestCapabilitydOnACentralPlaneAsksTheModelRouteWithTheServiceToken(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "openrouter-key")
	serviceTokenPath := filepath.Join(directory, "service-token")
	writeFileForModelReachTest(t, keyPath, "sk-own")
	writeFileForModelReachTest(t, serviceTokenPath, "service-token")
	askedURL, authorization := askedByAWebSearch(t, Configuration{
		OpenRouterKeyPath:       keyPath,
		ServiceTokenPath:        serviceTokenPath,
		CentralPlaneURL:         "https://plane.example",
		ModelRouteListenAddress: "127.0.0.1:18093",
	})
	if askedURL != "http://127.0.0.1:18093/api/v1/chat/completions" || authorization != "Bearer service-token" {
		t.Fatalf("capabilityd must ask the model route, which follows whoever pays; asked %s with %q", askedURL, authorization)
	}
}

func TestCapabilitydWithoutACentralPlanePaysWithTheCompanyKey(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "openrouter-key")
	writeFileForModelReachTest(t, keyPath, "sk-own")
	askedURL, authorization := askedByAWebSearch(t, Configuration{
		OpenRouterKeyPath:       keyPath,
		ServiceTokenPath:        filepath.Join(directory, "service-token"),
		ModelRouteListenAddress: "127.0.0.1:18093",
	})
	if askedURL != DefaultConfiguration().OpenRouterWebBaseURL || authorization != "Bearer sk-own" {
		t.Fatalf("with no central plane there is no one but the company to pay; asked %s with %q", askedURL, authorization)
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
