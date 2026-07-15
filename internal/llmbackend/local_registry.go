package llmbackend

import (
	"context"
	"net/http"
	"strings"
	"time"
)

var DefaultCompanionLocalProviderOrder = []string{"llamacpp", "ollama", "mlx"}
var DefaultDeviceLocalProviderOrder = []string{"llamacpp"}

const DefaultOllamaBaseURL = "http://127.0.0.1:11434"
const DefaultLlamaCppBaseURL = "http://127.0.0.1:18081"
const DefaultMLXBaseURL = "http://127.0.0.1:10240"

type LocalProviderConfig struct {
	ProviderOrder            []string
	ProviderName             string
	Accelerator              string
	AttemptTimeout           time.Duration
	AllowStructuredFallback  bool
	HTTPClient               *http.Client
	RunCommand               func(context.Context, string, []string, []byte) ([]byte, error)
	LlamaCppServiceName      string
	LlamaCppStartTimeout     time.Duration
	LlamaCppPollInterval     time.Duration
	LiteRTModelPath          string
	LiteRTRunnerPath         string
	OllamaBaseURL            string
	OllamaModel              string
	LlamaCppBaseURL          string
	LlamaCppModel            string
	LlamaCppEmbeddingBaseURL string
	LlamaCppEmbeddingModel   string
	MLXBaseURL               string
	MLXModel                 string
}

type LocalProviderSet struct {
	Provider           Provider
	Backends           []Backend
	ModelByBackendName map[string]string
}

func BuildLocalProviderSet(configuration LocalProviderConfig) LocalProviderSet {
	backends := buildLocalBackends(configuration)
	providers := make([]Provider, 0, len(backends))
	modelByBackendName := make(map[string]string, len(backends))
	for _, backend := range backends {
		providers = append(providers, backend)
		modelByBackendName[backend.Name()] = localProviderModel(backend, configuration)
	}
	return LocalProviderSet{
		Provider: AutoProvider{
			Providers:               providers,
			AttemptTimeout:          localProviderAttemptTimeout(configuration),
			AllowStructuredFallback: configuration.AllowStructuredFallback,
		},
		Backends:           backends,
		ModelByBackendName: modelByBackendName,
	}
}

func ParseProviderOrder(value string, fallback []string) []string {
	parts := strings.Split(value, ",")
	order := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmedValue := strings.ToLower(strings.TrimSpace(part))
		if trimmedValue != "" {
			order = append(order, trimmedValue)
		}
	}
	if len(order) > 0 {
		return order
	}
	return append([]string{}, fallback...)
}

func buildLocalBackends(configuration LocalProviderConfig) []Backend {
	order := localProviderOrder(configuration)
	backends := make([]Backend, 0, len(order))
	for _, providerName := range order {
		backends = append(backends, buildLocalBackendsForProvider(providerName, configuration)...)
	}
	return backends
}

func buildLocalBackendsForProvider(providerName string, configuration LocalProviderConfig) []Backend {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "litert":
		return buildLiteRTProviders(configuration)
	case "llamacpp":
		backend := LlamaCppBackend{
			BaseURL:    firstNonEmpty(configuration.LlamaCppBaseURL, DefaultLlamaCppBaseURL),
			ModelName:  configuration.LlamaCppModel,
			HTTPClient: configuration.HTTPClient,
		}
		return []Backend{manageLlamaCppBackend(backend, configuration)}
	case "ollama":
		return []Backend{OllamaBackend{
			BaseURL:    firstNonEmpty(configuration.OllamaBaseURL, DefaultOllamaBaseURL),
			ModelName:  configuration.OllamaModel,
			HTTPClient: configuration.HTTPClient,
		}}
	case "mlx":
		return []Backend{MLXBackend{
			BaseURL:    firstNonEmpty(configuration.MLXBaseURL, DefaultMLXBaseURL),
			ModelName:  configuration.MLXModel,
			HTTPClient: configuration.HTTPClient,
		}}
	default:
		return nil
	}
}

func manageLlamaCppBackend(backend LlamaCppBackend, configuration LocalProviderConfig) Backend {
	if configuration.RunCommand == nil || strings.TrimSpace(configuration.LlamaCppServiceName) == "" {
		return backend
	}
	return ManagedLlamaCppBackend{
		Backend:      backend,
		ServiceName:  configuration.LlamaCppServiceName,
		RunCommand:   configuration.RunCommand,
		StartTimeout: configuration.LlamaCppStartTimeout,
		PollInterval: configuration.LlamaCppPollInterval,
	}
}

func buildLiteRTProviders(configuration LocalProviderConfig) []Backend {
	accelerators := localAcceleratorsFor(configuration.Accelerator)
	backends := make([]Backend, 0, len(accelerators))
	for _, accelerator := range accelerators {
		backends = append(backends, LiteRTProvider{
			ModelPath:  configuration.LiteRTModelPath,
			RunnerPath: configuration.LiteRTRunnerPath,
			Variant:    accelerator,
			RunCommand: configuration.RunCommand,
		})
	}
	return backends
}

func localProviderOrder(configuration LocalProviderConfig) []string {
	if strings.TrimSpace(configuration.ProviderName) != "" {
		return []string{strings.ToLower(strings.TrimSpace(configuration.ProviderName))}
	}
	return append([]string{}, configuration.ProviderOrder...)
}

func localAcceleratorsFor(accelerator string) []string {
	switch strings.ToLower(strings.TrimSpace(accelerator)) {
	case "gpu":
		return []string{"gpu"}
	case "cpu":
		return []string{"cpu"}
	default:
		return []string{"gpu"}
	}
}

func localProviderModel(backend Backend, configuration LocalProviderConfig) string {
	switch backend.Name() {
	case "ollama":
		return configuration.OllamaModel
	case "llamacpp":
		return configuration.LlamaCppModel
	case "mlx":
		return configuration.MLXModel
	default:
		if strings.HasPrefix(backend.Name(), "litert-") {
			return configuration.LiteRTModelPath
		}
		return ""
	}
}

func localProviderAttemptTimeout(configuration LocalProviderConfig) time.Duration {
	return configuration.AttemptTimeout
}
