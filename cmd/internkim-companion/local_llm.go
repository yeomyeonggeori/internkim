package main

import (
	"flag"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

type localLLMSettings struct {
	Enabled       bool
	Configuration llmbackend.LocalProviderConfig
	ProviderSet   llmbackend.LocalProviderSet
}

type localLLMFlags struct {
	enabled         *bool
	providerOrder   *string
	ollamaBaseURL   *string
	ollamaModel     *string
	llamaCppBaseURL *string
	llamaCppModel   *string
	mlxBaseURL      *string
	mlxModel        *string
}

const companionLocalProviderOrderDefault = "llamacpp,ollama,mlx"

func registerLocalLLMFlags(flags *flag.FlagSet) localLLMFlags {
	return localLLMFlags{
		enabled:         flags.Bool("enable-local-llm", false, "enable local LLM inference on this machine"),
		providerOrder:   flags.String("local-backend-order", companionLocalProviderOrderDefault, "comma-separated provider priority (llamacpp,ollama,mlx)"),
		ollamaBaseURL:   flags.String("ollama-base-url", llmbackend.DefaultOllamaBaseURL, "ollama base URL"),
		ollamaModel:     flags.String("ollama-model", "", "ollama model name"),
		llamaCppBaseURL: flags.String("llamacpp-base-url", llmbackend.DefaultLlamaCppBaseURL, "llama.cpp server base URL"),
		llamaCppModel:   flags.String("llamacpp-model", "", "llama.cpp model alias"),
		mlxBaseURL:      flags.String("mlx-base-url", llmbackend.DefaultMLXBaseURL, "MLX server base URL"),
		mlxModel:        flags.String("mlx-model", "", "MLX model name"),
	}
}

func (localFlags localLLMFlags) settings(httpClient *http.Client) localLLMSettings {
	configuration := llmbackend.LocalProviderConfig{
		ProviderOrder:   llmbackend.ParseProviderOrder(*localFlags.providerOrder, llmbackend.DefaultCompanionLocalProviderOrder),
		HTTPClient:      httpClient,
		OllamaBaseURL:   *localFlags.ollamaBaseURL,
		OllamaModel:     *localFlags.ollamaModel,
		LlamaCppBaseURL: *localFlags.llamaCppBaseURL,
		LlamaCppModel:   *localFlags.llamaCppModel,
		MLXBaseURL:      *localFlags.mlxBaseURL,
		MLXModel:        *localFlags.mlxModel,
	}
	return localLLMSettings{
		Enabled:       *localFlags.enabled,
		Configuration: configuration,
		ProviderSet:   llmbackend.BuildLocalProviderSet(configuration),
	}
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

func localLLMProviderNames(backends []llmbackend.Backend) string {
	names := make([]string, 0, len(backends))
	for _, backend := range backends {
		names = append(names, backend.Name())
	}
	return strings.Join(names, ", ")
}
