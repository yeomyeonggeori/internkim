package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"strings"
	"sync"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

type localLLMSettings struct {
	Enabled       bool
	Configuration llmbackend.LocalProviderConfig
	ProviderSet   llmbackend.LocalProviderSet
}

type dynamicLocalLLM struct {
	mutex    sync.RWMutex
	settings localLLMSettings
}

type localLLMFlags struct {
	enabled                  *bool
	providerOrder            *string
	ollamaBaseURL            *string
	ollamaModel              *string
	llamaCppEmbeddingBaseURL *string
	llamaCppEmbeddingModel   *string
	llamaCppBaseURL          *string
	llamaCppModel            *string
	mlxBaseURL               *string
	mlxModel                 *string
}

const companionLocalProviderOrderDefault = "llamacpp,ollama,mlx"

func registerLocalLLMFlags(flags *flag.FlagSet) localLLMFlags {
	return localLLMFlags{
		enabled:                  flags.Bool("enable-local-llm", false, "enable local LLM inference on this machine"),
		providerOrder:            flags.String("local-backend-order", companionLocalProviderOrderDefault, "comma-separated provider priority (llamacpp,ollama,mlx)"),
		ollamaBaseURL:            flags.String("ollama-base-url", llmbackend.DefaultOllamaBaseURL, "ollama base URL"),
		ollamaModel:              flags.String("ollama-model", "", "ollama model name"),
		llamaCppEmbeddingBaseURL: flags.String("llamacpp-embedding-base-url", llmbackend.DefaultLlamaCppBaseURL, "llama.cpp embedding server base URL"),
		llamaCppEmbeddingModel:   flags.String("llamacpp-embedding-model", llmbackend.DefaultEmbeddingModelName, "llama.cpp embedding model name"),
		llamaCppBaseURL:          flags.String("llamacpp-base-url", llmbackend.DefaultLlamaCppBaseURL, "llama.cpp server base URL"),
		llamaCppModel:            flags.String("llamacpp-model", "", "llama.cpp model alias"),
		mlxBaseURL:               flags.String("mlx-base-url", llmbackend.DefaultMLXBaseURL, "MLX server base URL"),
		mlxModel:                 flags.String("mlx-model", "", "MLX model name"),
	}
}

func (localFlags localLLMFlags) settings(httpClient *http.Client) localLLMSettings {
	configuration := llmbackend.LocalProviderConfig{
		ProviderOrder:            llmbackend.ParseProviderOrder(*localFlags.providerOrder, llmbackend.DefaultCompanionLocalProviderOrder),
		HTTPClient:               httpClient,
		OllamaBaseURL:            *localFlags.ollamaBaseURL,
		OllamaModel:              *localFlags.ollamaModel,
		LlamaCppEmbeddingBaseURL: *localFlags.llamaCppEmbeddingBaseURL,
		LlamaCppEmbeddingModel:   *localFlags.llamaCppEmbeddingModel,
		LlamaCppBaseURL:          *localFlags.llamaCppBaseURL,
		LlamaCppModel:            *localFlags.llamaCppModel,
		MLXBaseURL:               *localFlags.mlxBaseURL,
		MLXModel:                 *localFlags.mlxModel,
	}
	return localLLMSettings{
		Enabled:       *localFlags.enabled,
		Configuration: configuration,
		ProviderSet:   llmbackend.BuildLocalProviderSet(configuration),
	}
}

func newDynamicLocalLLM(settings localLLMSettings) *dynamicLocalLLM {
	return &dynamicLocalLLM{settings: settings}
}

func (provider *dynamicLocalLLM) update(settings localLLMSettings) {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	provider.settings = settings
}

func (provider *dynamicLocalLLM) currentSettings() localLLMSettings {
	provider.mutex.RLock()
	defer provider.mutex.RUnlock()
	return provider.settings
}

func (provider *dynamicLocalLLM) providerSetFor(providerName string, accelerator string) (llmbackend.LocalProviderSet, bool) {
	settings := provider.currentSettings()
	if !settings.Enabled {
		return llmbackend.LocalProviderSet{}, false
	}
	return settings.providerSetFor(providerName, accelerator), true
}

func (provider *dynamicLocalLLM) CompleteStructured(ctx context.Context, request llmbackend.StructuredRequest) (llmbackend.Response, error) {
	providerSet, isEnabled := provider.providerSetFor(request.Provider, request.Accelerator)
	if !isEnabled {
		return llmbackend.Response{}, errors.New("companion LLM is not configured")
	}
	return providerSet.Provider.CompleteStructured(ctx, request)
}

func (provider *dynamicLocalLLM) CompleteText(ctx context.Context, request llmbackend.TextRequest) (llmbackend.Response, error) {
	providerSet, isEnabled := provider.providerSetFor(request.Provider, request.Accelerator)
	if !isEnabled {
		return llmbackend.Response{}, errors.New("companion LLM is not configured")
	}
	return providerSet.Provider.CompleteText(ctx, request)
}

func (provider *dynamicLocalLLM) CreateEmbedding(ctx context.Context, request llmbackend.EmbeddingRequest) (llmbackend.EmbeddingResponse, error) {
	providerSet, isEnabled := provider.embeddingProviderSetFor(request.Provider)
	if !isEnabled {
		return llmbackend.EmbeddingResponse{}, errors.New("companion embedding is not configured")
	}
	return providerSet.Provider.CreateEmbedding(ctx, request)
}

func (settings localLLMSettings) providerSetFor(providerName string, accelerator string) llmbackend.LocalProviderSet {
	if strings.TrimSpace(providerName) == "" && strings.TrimSpace(accelerator) == "" {
		return settings.ProviderSet
	}
	configuration := settings.Configuration
	configuration.ProviderName = providerName
	configuration.Accelerator = accelerator
	return llmbackend.BuildLocalProviderSet(configuration)
}

func (provider *dynamicLocalLLM) embeddingProviderSetFor(providerName string) (llmbackend.LocalEmbeddingProviderSet, bool) {
	settings := provider.currentSettings()
	if !settings.Enabled {
		return llmbackend.LocalEmbeddingProviderSet{}, false
	}
	return settings.embeddingProviderSetFor(providerName), true
}

func (settings localLLMSettings) embeddingProviderSetFor(providerName string) llmbackend.LocalEmbeddingProviderSet {
	configuration := llmbackend.LocalEmbeddingProviderConfig{
		ProviderOrder:   llmbackend.DefaultLocalEmbeddingProviderOrder,
		ProviderName:    providerName,
		AttemptTimeout:  settings.Configuration.AttemptTimeout,
		HTTPClient:      settings.Configuration.HTTPClient,
		LlamaCppBaseURL: firstNonEmpty(settings.Configuration.LlamaCppEmbeddingBaseURL, settings.Configuration.LlamaCppBaseURL),
		LlamaCppModel:   firstNonEmpty(settings.Configuration.LlamaCppEmbeddingModel, llmbackend.DefaultEmbeddingModelName),
	}
	return llmbackend.BuildLocalEmbeddingProviderSet(configuration)
}

func localLLMProviderNames(backends []llmbackend.Backend) string {
	names := make([]string, 0, len(backends))
	for _, backend := range backends {
		names = append(names, backend.Name())
	}
	return strings.Join(names, ", ")
}

func localEmbeddingProviderNames(backends []llmbackend.EmbeddingBackend) string {
	names := make([]string, 0, len(backends))
	for _, backend := range backends {
		names = append(names, backend.Name())
	}
	return strings.Join(names, ", ")
}
