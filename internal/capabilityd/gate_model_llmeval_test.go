//go:build llmeval

package capabilityd

import (
	"context"
	"os"
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
	keyPath := strings.TrimSpace(os.Getenv("INTERNKIM_GATE_OPENROUTER_KEY_PATH"))
	if keyPath == "" {
		t.Fatal("INTERNKIM_GATE_OPENROUTER_KEY_PATH is required for live catalog model contracts")
	}
	if _, errorValue := os.Stat(keyPath); errorValue != nil {
		t.Fatalf("read OpenRouter key path: %v", errorValue)
	}

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
