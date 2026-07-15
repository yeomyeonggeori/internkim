package llmbackend

import (
	"context"
	"net/http"
	"strings"
	"time"
)

var DefaultLocalEmbeddingProviderOrder = []string{"llamacpp"}

type LocalEmbeddingProviderConfig struct {
	ProviderOrder   []string
	ProviderName    string
	AttemptTimeout  time.Duration
	HTTPClient      *http.Client
	LlamaCppBaseURL string
	LlamaCppModel   string
}

type LocalEmbeddingProviderSet struct {
	Provider           EmbeddingProvider
	Backends           []EmbeddingBackend
	ModelByBackendName map[string]string
}

func BuildLocalEmbeddingProviderSet(configuration LocalEmbeddingProviderConfig) LocalEmbeddingProviderSet {
	backends := buildLocalEmbeddingBackends(configuration)
	providers := make([]EmbeddingProvider, 0, len(backends))
	modelByBackendName := make(map[string]string, len(backends))
	for _, backend := range backends {
		providers = append(providers, backend)
		modelByBackendName[backend.Name()] = localEmbeddingProviderModel(backend, configuration)
	}
	return LocalEmbeddingProviderSet{
		Provider: AutoEmbeddingProvider{
			Providers:      providers,
			AttemptTimeout: localEmbeddingProviderAttemptTimeout(configuration),
		},
		Backends:           backends,
		ModelByBackendName: modelByBackendName,
	}
}

func buildLocalEmbeddingBackends(configuration LocalEmbeddingProviderConfig) []EmbeddingBackend {
	order := localEmbeddingProviderOrder(configuration)
	backends := make([]EmbeddingBackend, 0, len(order))
	for _, providerName := range order {
		backends = append(backends, buildLocalEmbeddingBackendsForProvider(providerName, configuration)...)
	}
	return backends
}

func buildLocalEmbeddingBackendsForProvider(providerName string, configuration LocalEmbeddingProviderConfig) []EmbeddingBackend {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "llamacpp":
		return []EmbeddingBackend{LlamaCppEmbeddingBackend{
			BaseURL:    firstNonEmpty(configuration.LlamaCppBaseURL, DefaultLlamaCppBaseURL),
			ModelName:  firstNonEmpty(configuration.LlamaCppModel, DefaultEmbeddingModelName),
			HTTPClient: configuration.HTTPClient,
		}}
	default:
		return nil
	}
}

func localEmbeddingProviderOrder(configuration LocalEmbeddingProviderConfig) []string {
	if strings.TrimSpace(configuration.ProviderName) != "" {
		return []string{strings.ToLower(strings.TrimSpace(configuration.ProviderName))}
	}
	if len(configuration.ProviderOrder) > 0 {
		return append([]string{}, configuration.ProviderOrder...)
	}
	return append([]string{}, DefaultLocalEmbeddingProviderOrder...)
}

func localEmbeddingProviderModel(backend EmbeddingBackend, configuration LocalEmbeddingProviderConfig) string {
	switch backend.Name() {
	case "llamacpp":
		return firstNonEmpty(configuration.LlamaCppModel, DefaultEmbeddingModelName)
	default:
		return ""
	}
}

func localEmbeddingProviderAttemptTimeout(configuration LocalEmbeddingProviderConfig) time.Duration {
	return configuration.AttemptTimeout
}

func pingEmbeddingBackend(ctx context.Context, backend EmbeddingBackend) error {
	if backend == nil {
		return nil
	}
	return backend.Ping(ctx)
}
