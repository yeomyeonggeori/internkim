package admind

import (
	"log"
	"strings"

	"gitlab.com/eastriver/internkim/internal/modelladder"
)

func migrateBlueclawCapabilityModelConfiguration(runtimeDocument map[string]any) {
	languageModel, exists := runtimeDocument["languageModel"].(map[string]any)
	if !exists || languageModel["tiers"] != nil {
		return
	}
	capability, exists := languageModel["capability"].(map[string]any)
	if !exists {
		return
	}
	provider, isLegacy := languageModel["defaultProvider"].(string)
	if !isLegacy {
		return
	}
	if provider != "" && provider != "capability" && provider != "capabilityLLM" && provider != "llmd" {
		return
	}
	for tier, defaultModel := range modelladder.TierModelNames() {
		field := tier + "Model"
		modelName, _ := capability[field].(string)
		if strings.TrimSpace(modelName) == "" {
			capability[field] = defaultModel
		}
	}
	if languageModel["embedding"] == nil {
		languageModel["embedding"] = map[string]any{"model": modelladder.EmbeddingModel}
	}
	if languageModel["contextWindowTokens"] == nil && capability["contextWindowTokens"] != nil {
		languageModel["contextWindowTokens"] = capability["contextWindowTokens"]
	}
	delete(languageModel, "defaultProvider")
	delete(languageModel, "fallbackProvider")
	delete(capability, "contextWindowTokens")
	log.Print("blueclaw runtime configuration migrated from legacy capability provider to explicit model tiers")
}
