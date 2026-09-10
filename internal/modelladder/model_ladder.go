// Package modelladder is the only place a model is named.
package modelladder

import "strings"

const (
	Endpoint = "https://openrouter.ai/api/v1"

	PrimaryModel   = "z-ai/glm-5.3-flash"
	EmbeddingModel = "baai/bge-m3"
	ImageModel     = "google/gemini-3.1-flash-lite-image"
)

var DegradedModels = []string{
	"google/gemini-3.1-flash-lite",
	"z-ai/glm-5.2",
}

var Tiers = []string{"xlow", "low", "medium", "high", "xhigh", "max"}

// OpenRouter balances a request across a model's providers by price unless
// the request names an order or a sort.
// https://openrouter.ai/docs/features/provider-routing
const ProviderSort = "throughput"

var preferredProviders = []string{"modal", "baseten"}

func PreferredProviders() []string {
	return append([]string{}, preferredProviders...)
}

// OpenRouter's effort levels: none, minimal, low, medium, high, xhigh, max.
// https://openrouter.ai/docs/use-cases/reasoning-tokens
var reasoningEffortByTier = map[string]string{
	"xlow":   "none",
	"low":    "low",
	"medium": "medium",
	"high":   "high",
	"xhigh":  "xhigh",
	"max":    "max",
}

func ReasoningEffort(tier string) string {
	return reasoningEffortByTier[strings.TrimSpace(tier)]
}

type Rung struct {
	Endpoint        string   `json:"endpoint"`
	Model           string   `json:"model"`
	APIKeyPath      string   `json:"apiKeyPath,omitempty"`
	ProviderOrder   []string `json:"providerOrder,omitempty"`
	ProviderSort    string   `json:"providerSort,omitempty"`
	ReasoningEffort string   `json:"reasoningEffort,omitempty"`
}

type Document struct {
	Tiers     map[string][]Rung `json:"tiers"`
	Embedding Rung              `json:"embedding"`
}

func ModelNames() []string {
	return append([]string{PrimaryModel}, DegradedModels...)
}

func rungsForOneTier(tier string, endpointURL string, apiKeyPath string) []Rung {
	rungs := []Rung{}
	for _, modelName := range ModelNames() {
		rungs = append(rungs, Rung{
			Endpoint:        endpointURL,
			Model:           modelName,
			APIKeyPath:      apiKeyPath,
			ProviderOrder:   PreferredProviders(),
			ProviderSort:    ProviderSort,
			ReasoningEffort: ReasoningEffort(tier),
		})
	}
	return rungs
}

func LanguageModelDocument(endpointURL string, apiKeyPath string) Document {
	reachedEndpoint := strings.TrimSpace(endpointURL)
	if reachedEndpoint == "" {
		reachedEndpoint = Endpoint
	}
	tiers := map[string][]Rung{}
	for _, tier := range Tiers {
		tiers[tier] = rungsForOneTier(tier, reachedEndpoint, apiKeyPath)
	}
	return Document{
		Tiers:     tiers,
		Embedding: Rung{Endpoint: reachedEndpoint, Model: EmbeddingModel, APIKeyPath: apiKeyPath},
	}
}

func TierModelNames() map[string]string {
	modelNames := map[string]string{}
	for _, tier := range Tiers {
		modelNames[tier] = PrimaryModel
	}
	return modelNames
}
