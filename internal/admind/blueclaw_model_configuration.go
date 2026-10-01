package admind

import (
	"log"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/modelladder"
	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func blueclawLanguageModelSections(runtimeDocument map[string]any) (languageModel map[string]any, capability map[string]any, exists bool) {
	languageModel, exists = runtimeDocument["languageModel"].(map[string]any)
	if !exists {
		return nil, nil, false
	}
	capability, exists = languageModel["capability"].(map[string]any)
	return languageModel, capability, exists
}

func stampBlueclawLadderOwnedModels(runtimeDocument map[string]any) {
	_, capability, exists := blueclawLanguageModelSections(runtimeDocument)
	if !exists {
		return
	}
	blueclawruntime.StampLadderOwnedModels(capability)
}

func migrateBlueclawCapabilityModelConfiguration(runtimeDocument map[string]any) {
	languageModel, capability, exists := blueclawLanguageModelSections(runtimeDocument)
	if !exists || languageModel["tiers"] != nil {
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
