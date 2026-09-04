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

type Rung struct {
	Endpoint   string `json:"endpoint"`
	Model      string `json:"model"`
	APIKeyPath string `json:"apiKeyPath,omitempty"`
}

type Document struct {
	Tiers     map[string][]Rung `json:"tiers"`
	Embedding Rung              `json:"embedding"`
}

func ModelNames() []string {
	return append([]string{PrimaryModel}, DegradedModels...)
}

func rungsForOneTier(endpointURL string, apiKeyPath string) []Rung {
	rungs := []Rung{}
	for _, modelName := range ModelNames() {
		rungs = append(rungs, Rung{Endpoint: endpointURL, Model: modelName, APIKeyPath: apiKeyPath})
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
		tiers[tier] = rungsForOneTier(reachedEndpoint, apiKeyPath)
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
