package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

const webSearchAnswer = `{"choices":[{"message":{"content":"{\"provider\":\"openrouter\",\"remoteLLMInvolved\":true,\"compatibility\":\"openrouter_server_tool_auto\",\"query\":\"internkim\",\"answer\":\"result\",\"results\":[]}"}}]}`

func TestALadderThatReachesTheGatewayMakesCapabilitydPayThroughItAsThisComputer(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "openrouter-key")
	hostSessionPath := filepath.Join(directory, "agent-key")
	writeFileForModelReachTest(t, keyPath, "sk-own")
	writeFileForModelReachTest(t, hostSessionPath, "host-session")
	askedURL, authorization := askedByAWebSearch(t, Configuration{
		OpenRouterKeyPath:   keyPath,
		HostSessionPath:     hostSessionPath,
		CentralPlaneURL:     "https://plane.example",
		RuntimeDocumentPath: runtimeDocumentPaying(t, directory, true, keyPath, hostSessionPath),
	})
	if askedURL != "https://plane.example/functions/v1/model-gateway/api/v1/chat/completions" || authorization != "Bearer host-session" {
		t.Fatalf("the preparation chose the gateway, so capabilityd must ask it as this computer; asked %s with %q", askedURL, authorization)
	}
}

func TestALadderThatReachesTheCompanyKeyMakesCapabilitydPayWithIt(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "openrouter-key")
	writeFileForModelReachTest(t, keyPath, "sk-own")
	askedURL, authorization := askedByAWebSearch(t, Configuration{
		OpenRouterKeyPath:   keyPath,
		HostSessionPath:     filepath.Join(directory, "agent-key"),
		CentralPlaneURL:     "https://plane.example",
		RuntimeDocumentPath: runtimeDocumentPaying(t, directory, false, keyPath, ""),
	})
	if askedURL != DefaultConfiguration().OpenRouterWebBaseURL || authorization != "Bearer sk-own" {
		t.Fatalf("the preparation chose the company's key, so capabilityd must ask OpenRouter with it; asked %s with %q", askedURL, authorization)
	}
}

func TestCapabilitydWithoutTheDocumentThePreparationWroteDoesNotStart(t *testing.T) {
	_, errorValue := Configuration{CentralPlaneURL: "https://plane.example", RuntimeDocumentPath: filepath.Join(t.TempDir(), "runtime.json")}.WithDefaults().reachingModels()
	if errorValue == nil {
		t.Fatal("capabilityd must not choose a payer the preparation did not write down")
	}
}

func runtimeDocumentPaying(t *testing.T, directory string, throughTheGateway bool, keyPath string, hostSessionPath string) string {
	t.Helper()
	ladder, errorValue := modelladder.CompanyLanguageModelDocument("https://plane.example", hostSessionPath, "", keyPath, func(bool) (bool, error) { return !throughTheGateway, nil })
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := json.Marshal(map[string]any{"languageModel": ladder})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	path := filepath.Join(directory, "runtime.json")
	writeFileForModelReachTest(t, path, string(document))
	return path
}

func askedByAWebSearch(t *testing.T, configuration Configuration) (string, string) {
	t.Helper()
	var askedURL, authorization string
	reaching, errorValue := configuration.WithDefaults().reachingModels()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service := Service{
		Configuration: reaching,
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
