//go:build llmeval

package capabilityd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveCatalogModelToolContracts(t *testing.T) {
	if os.Getenv("INTERNKIM_LIVE_LLM_TEST") != "1" {
		t.Skip("set INTERNKIM_LIVE_LLM_TEST=1 to run live catalog model contracts")
	}
	modelName := strings.TrimSpace(os.Getenv("INTERNKIM_GATE_MODEL"))
	if modelName == "" {
		t.Fatal("INTERNKIM_GATE_MODEL is required for live catalog model contracts")
	}
	keyPath := openRouterKeyFileFromEnvironment(t)

	for name, gateCase := range gateCases() {
		if _, reachesOpenRouter := gateCase.reaches[openRouterOverHTTP]; !reachesOpenRouter {
			continue
		}
		t.Run(name, func(t *testing.T) {
			service := serviceReaching(t, gateCase.reaches)
			service.Configuration.OpenRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"
			service.Configuration.OpenRouterWebBaseURL = service.Configuration.OpenRouterBaseURL
			service.Configuration.OpenRouterKeyPath = keyPath
			service.Configuration.OpenRouterModel = modelName
			service.Configuration.ForceOpenRouterModel = true

			route, hasRoute := capabilityToolRouteFor(name)
			if !hasRoute {
				t.Fatalf("%s is in the catalog and nothing serves it", name)
			}
			answered, errorValue := route.Handler(service, context.Background(), arrivingCall(name, gateCase))
			if errorValue != nil {
				t.Fatalf("%s: %v", name, errorValue)
			}
			expectAnswerKeepsItsContract(t, name, answered)
		})
	}
}

func openRouterKeyFileFromEnvironment(t *testing.T) string {
	t.Helper()
	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		t.Fatal("OPENROUTER_API_KEY is required for live catalog model contracts: run through monkeys run @test")
	}
	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte(key), 0o600); errorValue != nil {
		t.Fatalf("write OpenRouter key file: %v", errorValue)
	}
	return keyPath
}
