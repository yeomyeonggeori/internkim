// Package modelladder is the only place a model is named.
package modelladder

import (
	"net/url"
	"strings"
)

const (
	Endpoint = "https://openrouter.ai/api/v1"

	PrimaryModel   = "z-ai/glm-5.3-flash"
	EmbeddingModel = "baai/bge-m3"
	ImageModel     = "google/gemini-3.1-flash-lite-image"
	DecisionModel  = "~typesafe/jev-latest"
)

var DegradedModels = []string{
	"google/gemini-3.1-flash-lite",
	"z-ai/glm-5.2",
}

var Tiers = []string{"xlow", "low", "medium", "high", "xhigh", "max"}

// OpenRouter balances a request across a model's providers by price unless
// the request names a sort.
// https://openrouter.ai/docs/features/provider-routing
const ProviderSort = "throughput"

// OpenRouter's effort levels: none, minimal, low, medium, high, xhigh, max.
// https://openrouter.ai/docs/use-cases/reasoning-tokens
var reasoningEffortByTier = map[string]string{
	"xlow":   "minimal",
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
	Endpoint        string `json:"endpoint"`
	Model           string `json:"model"`
	APIKeyPath      string `json:"apiKeyPath,omitempty"`
	ProviderSort    string `json:"providerSort,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

type Document struct {
	Tiers     map[string][]Rung `json:"tiers"`
	Embedding Rung              `json:"embedding"`
	Decision  Rung              `json:"decision"`
}

// OpenRouter serves decision-only models on its alpha decisions route, not on
// chat completions. github.com/OpenRouterTeam/go-sdk decisions.go
const DecisionsPath = "/api/alpha/decisions"

func DecisionsURL(modelEndpoint string) string {
	origin := originOf(modelEndpoint)
	if origin == "" {
		origin = originOf(Endpoint)
	}
	return origin + DecisionsPath
}

func originOf(rawURL string) string {
	parsed, errorValue := url.Parse(strings.TrimSpace(rawURL))
	if errorValue != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
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
		Decision:  Rung{Endpoint: DecisionsURL(reachedEndpoint), Model: DecisionModel, APIKeyPath: apiKeyPath},
	}
}

func TierModelNames() map[string]string {
	modelNames := map[string]string{}
	for _, tier := range Tiers {
		modelNames[tier] = PrimaryModel
	}
	return modelNames
}
