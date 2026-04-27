package blueclaw

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBlueclawRuntimeConfigUsesCapabilityBoundary(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocument(BlueclawDefaultModelName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var runtimeConfiguration map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatal(errorValue)
	}

	languageModel := runtimeConfiguration["languageModel"].(map[string]any)
	if languageModel["defaultProvider"] != "capabilityLLM" {
		t.Fatalf("expected capability default provider, got %q", languageModel["defaultProvider"])
	}
	capabilities := runtimeConfiguration["capabilities"].(map[string]any)
	if capabilities["unixSocketPath"] != CapabilitySocketPath {
		t.Fatalf("expected capability unix socket path, got %q", capabilities["unixSocketPath"])
	}
	capabilityToolNames := capabilities["toolNames"].([]any)
	if !containsStringValue(capabilityToolNames, "user.confirm") {
		t.Fatalf("expected companion capability tools, got %+v", capabilityToolNames)
	}
	routing := capabilities["routing"].(map[string]any)
	if routing["localOnly"] != false {
		t.Fatalf("expected default routing to allow remote fallback, got %v", routing["localOnly"])
	}
	capabilityLanguageModel := languageModel["capability"].(map[string]any)
	if capabilityLanguageModel["executionMode"] != "auto" {
		t.Fatalf("expected automatic execution mode, got %q", capabilityLanguageModel["executionMode"])
	}
	if _, isFound := languageModel["openRouter"]; isFound {
		t.Fatal("expected OpenRouter runtime details to be omitted")
	}
	if _, isFound := languageModel["liteRTLM"]; isFound {
		t.Fatal("expected LiteRT runtime details to be omitted")
	}
	memory := runtimeConfiguration["memory"].(map[string]any)
	if memory["graphitiEndpoint"] != GraphitiEndpoint {
		t.Fatalf("expected Graphiti endpoint, got %q", memory["graphitiEndpoint"])
	}
	if memory["graphitiKuzuPath"] != GraphitiKuzuPath {
		t.Fatalf("expected Graphiti Kuzu path, got %q", memory["graphitiKuzuPath"])
	}
	if memory["timeoutSecond"] != float64(60) {
		t.Fatalf("expected Graphiti timeout, got %v", memory["timeoutSecond"])
	}
	agent := runtimeConfiguration["agent"].(map[string]any)
	intake := agent["intake"].(map[string]any)
	if intake["enabled"] != true {
		t.Fatalf("expected agent intake enabled, got %v", intake["enabled"])
	}
	if intake["model"] != "local/gemma-4-E4B-it-litert-lm" {
		t.Fatalf("expected agent intake model, got %v", intake["model"])
	}
	if intake["executionMode"] != "local" {
		t.Fatalf("expected agent intake execution mode, got %v", intake["executionMode"])
	}
	if agent["maxIterationsPerRequest"] != float64(8) {
		t.Fatalf("expected agent max iterations, got %v", agent["maxIterationsPerRequest"])
	}
	if agent["maxToolCallsPerRequest"] != float64(8) {
		t.Fatalf("expected agent max tool calls, got %v", agent["maxToolCallsPerRequest"])
	}
	if agent["maxWallClockSecond"] != float64(120) {
		t.Fatalf("expected agent wall clock budget, got %v", agent["maxWallClockSecond"])
	}
	if agent["toolResultMaxBytes"] != float64(32768) {
		t.Fatalf("expected agent tool result limit, got %v", agent["toolResultMaxBytes"])
	}
	agentProfiles := runtimeConfiguration["agentProfiles"].([]any)
	defaultProfile := agentProfiles[0].(map[string]any)
	allowedToolNames := defaultProfile["allowedToolNames"].([]any)
	if !containsStringValue(allowedToolNames, "conversation.history") || !containsStringValue(allowedToolNames, "memory.search") {
		t.Fatalf("expected default agent profile to allow internal tools, got %+v", allowedToolNames)
	}
	for _, expectedToolName := range []string{"browser.session.start", "browser.navigate", "browser.click", "browser.fill", "browser.select", "browser.press", "browser.wait", "user.confirm", "file.pick"} {
		if !containsStringValue(allowedToolNames, expectedToolName) {
			t.Fatalf("expected default profile to allow %q, got %+v", expectedToolName, allowedToolNames)
		}
	}
	connectors := runtimeConfiguration["connectors"].(map[string]any)
	mattermost := connectors["mattermost"].(map[string]any)
	if _, isFound := mattermost["botTokenPath"]; isFound {
		t.Fatal("expected Mattermost bot token path to be omitted")
	}
	forbiddenFragments := []string{"apiKeyPath", "botTokenPath", "signingSecretPath", "wrapperPath", "modelPath", "backend"}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(document, fragment) {
			t.Fatalf("expected runtime config to omit %q", fragment)
		}
	}
}

func containsStringValue(values []any, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}

func TestBlueclawServiceDoesNotExposeOpenRouterKeyAsEnvironmentFile(t *testing.T) {
	serviceDocument := BlueclawServiceUnit()
	if strings.Contains(serviceDocument, "EnvironmentFile=") {
		t.Fatal("expected Blueclaw service to avoid OpenRouter key environment files")
	}
}
