package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/anthropic-lab/internkim/internal/llmbackend"
)

type localLLMSettings struct {
	Enabled          bool
	BackendOrder     []string
	OllamaBaseURL    string
	OllamaModel      string
	LlamaCppBaseURL  string
	LlamaCppModel    string
	MLXBaseURL       string
	MLXModel         string
	AttemptTimeout   time.Duration
}

const defaultLocalLLMTimeout = 90 * time.Second

func buildLocalChain(settings localLLMSettings, httpClient *http.Client) (llmbackend.Provider, []llmbackend.Backend) {
	backends := make([]llmbackend.Backend, 0, len(settings.BackendOrder))
	for _, name := range settings.BackendOrder {
		backend := buildLocalBackend(name, settings, httpClient)
		if backend != nil {
			backends = append(backends, backend)
		}
	}
	providers := make([]llmbackend.Provider, 0, len(backends))
	for _, backend := range backends {
		providers = append(providers, backend)
	}
	return llmbackend.AutoProvider{
		Providers:      providers,
		AttemptTimeout: localLLMAttemptTimeout(settings),
	}, backends
}

func buildLocalBackend(name string, settings localLLMSettings, httpClient *http.Client) llmbackend.Backend {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ollama":
		return llmbackend.OllamaBackend{
			BaseURL:    firstNonEmpty(settings.OllamaBaseURL, "http://127.0.0.1:11434"),
			ModelName:  settings.OllamaModel,
			HTTPClient: httpClient,
		}
	case "llamacpp":
		return llmbackend.LlamaCppBackend{
			BaseURL:    firstNonEmpty(settings.LlamaCppBaseURL, "http://127.0.0.1:8080"),
			ModelName:  settings.LlamaCppModel,
			HTTPClient: httpClient,
		}
	case "mlx":
		return llmbackend.MLXBackend{
			BaseURL:    firstNonEmpty(settings.MLXBaseURL, "http://127.0.0.1:10240"),
			ModelName:  settings.MLXModel,
			HTTPClient: httpClient,
		}
	default:
		return nil
	}
}

func localLLMAttemptTimeout(settings localLLMSettings) time.Duration {
	if settings.AttemptTimeout > 0 {
		return settings.AttemptTimeout
	}
	return defaultLocalLLMTimeout
}

func parseBackendOrder(value string) []string {
	parts := strings.Split(value, ",")
	order := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			order = append(order, trimmed)
		}
	}
	return order
}

func localBackendModelMap(settings localLLMSettings) map[string]string {
	return map[string]string{
		"ollama":   settings.OllamaModel,
		"llamacpp": settings.LlamaCppModel,
		"mlx":      settings.MLXModel,
	}
}
