package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

type grantListDocument struct {
	Grants []companionruntime.GrantSnapshot `json:"grants"`
}

type mountListDocument struct {
	Mounts []companionruntime.MountSnapshot `json:"mounts"`
}

type localLLMBackendStatus struct {
	Name          string `json:"name"`
	Model         string `json:"model,omitempty"`
	Available     bool   `json:"available"`
	LastError     string `json:"lastError,omitempty"`
	LastCheckedAt string `json:"lastCheckedAt,omitempty"`
}

type localLLMStatus struct {
	Enabled  bool                    `json:"enabled"`
	Backends []localLLMBackendStatus `json:"backends,omitempty"`
}

type controlBackendEndpoint struct {
	BaseURL        string `json:"baseURL"`
	Model          string `json:"model"`
	EmbeddingModel string `json:"embeddingModel,omitempty"`
}

type controlLocalLLMSettings struct {
	EnableLocalLLM    bool                   `json:"enableLocalLLM"`
	LocalBackendOrder []string               `json:"localBackendOrder"`
	Ollama            controlBackendEndpoint `json:"ollama"`
	LlamaCpp          controlBackendEndpoint `json:"llamacpp"`
	MLX               controlBackendEndpoint `json:"mlx"`
}

type runtimeStatusDocument struct {
	LastHeartbeatAt string         `json:"lastHeartbeatAt,omitempty"`
	LastError       string         `json:"lastError,omitempty"`
	LocalLLM        localLLMStatus `json:"localLLM"`
}

type runtimeState struct {
	mutex                 sync.Mutex
	lastHeartbeatAt       string
	lastError             string
	localLLM              localLLMStatus
	localLLMBackends      []llmbackend.Backend
	localLLMModelByName   map[string]string
	localLLMLastCheckAt   time.Time
	localLLMCacheLifetime time.Duration
}

const defaultLocalLLMCacheLifetime = 30 * time.Second

func (state *runtimeState) recordHeartbeat(errorValue error) {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	if errorValue != nil {
		state.lastError = errorValue.Error()
		return
	}
	state.lastHeartbeatAt = time.Now().UTC().Format(time.RFC3339)
	state.lastError = ""
}

func (state *runtimeState) RecordHeartbeat(errorValue error) {
	state.recordHeartbeat(errorValue)
}

func (state *runtimeState) replaceLocalLLM(settings localLLMSettings) localLLMStatus {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	state.localLLMLastCheckAt = time.Time{}
	state.localLLMBackends = nil
	state.localLLMModelByName = nil
	if !settings.Enabled {
		state.localLLM = localLLMStatus{}
		return state.localLLM
	}
	state.localLLMBackends = settings.ProviderSet.Backends
	state.localLLMModelByName = settings.ProviderSet.ModelByBackendName
	state.localLLM = localBackendsSummary(settings.ProviderSet.Backends)
	if state.localLLMCacheLifetime == 0 {
		state.localLLMCacheLifetime = defaultLocalLLMCacheLifetime
	}
	return state.localLLM
}

func (state *runtimeState) localLLMAvailable() bool {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	for _, backend := range state.localLLM.Backends {
		if backend.Available {
			return true
		}
	}
	return false
}

func (state *runtimeState) LocalLLMAvailable() bool {
	return state.localLLMAvailable()
}

func (state *runtimeState) refreshLocalLLM(ctx context.Context) localLLMStatus {
	state.mutex.Lock()
	if !state.localLLM.Enabled || len(state.localLLMBackends) == 0 {
		summary := state.localLLM
		state.mutex.Unlock()
		return summary
	}
	if !state.localLLMLastCheckAt.IsZero() && time.Since(state.localLLMLastCheckAt) < state.localLLMCacheLifetime {
		summary := state.localLLM
		state.mutex.Unlock()
		return summary
	}
	backends := state.localLLMBackends
	modelByName := state.localLLMModelByName
	state.mutex.Unlock()

	statuses := make([]localLLMBackendStatus, 0, len(backends))
	for _, backend := range backends {
		pingContext, cancel := context.WithTimeout(ctx, time.Second)
		errorValue := backend.Ping(pingContext)
		cancel()
		status := localLLMBackendStatus{
			Name:          backend.Name(),
			Model:         modelByName[backend.Name()],
			Available:     errorValue == nil,
			LastCheckedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if errorValue != nil {
			status.LastError = errorValue.Error()
		}
		statuses = append(statuses, status)
	}
	summary := localLLMStatus{Enabled: true, Backends: statuses}

	state.mutex.Lock()
	state.localLLM = summary
	state.localLLMLastCheckAt = time.Now()
	state.mutex.Unlock()
	return summary
}

func (state *runtimeState) snapshot() runtimeStatusDocument {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	return runtimeStatusDocument{
		LastHeartbeatAt: state.lastHeartbeatAt,
		LastError:       state.lastError,
		LocalLLM:        state.localLLM,
	}
}

func startControlServer(listenAddress string, grantStore *companionruntime.MemoryGrantStore, mountStore *companionruntime.MountStore, handoffStore *companionruntime.BrowserHandoffStore, browserRuntime browserruntime.Runtime, handoffCompletionHandler func(context.Context, companionruntime.HandoffCompletion) error, runtime *runtimeState, localLLM *dynamicLocalLLM, httpClient *http.Client) (*http.Server, error) {
	trimmedAddress := strings.TrimSpace(listenAddress)
	if trimmedAddress == "" {
		return nil, nil
	}
	listener, errorValue := listenLoopbackOnly("tcp", trimmedAddress, "companion control server")
	if errorValue != nil {
		return nil, errorValue
	}
	server := &http.Server{Handler: controlHandler(grantStore, mountStore, handoffStore, browserRuntime, handoffCompletionHandler, runtime, localLLM, httpClient)}
	go func() {
		errorValue := server.Serve(listener)
		if errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
			_ = errorValue
		}
	}()
	return server, nil
}

func controlHandler(grantStore *companionruntime.MemoryGrantStore, mountStore *companionruntime.MountStore, handoffStore *companionruntime.BrowserHandoffStore, browserRuntime browserruntime.Runtime, handoffCompletionHandler func(context.Context, companionruntime.HandoffCompletion) error, runtime *runtimeState, localLLM *dynamicLocalLLM, httpClient *http.Client) http.Handler {
	multiplexer := http.NewServeMux()
	handoffBridgeHandler := companionruntime.HandoffBridgeHandler{
		Store:             handoffStore,
		BrowserRuntime:    browserRuntime,
		CompletionHandler: handoffCompletionHandler,
	}
	multiplexer.Handle("/v1/browser/handoff", handoffBridgeHandler)
	multiplexer.Handle("/v1/browser/handoff/complete", handoffBridgeHandler)
	multiplexer.HandleFunc("GET /v1/runtime/status", func(responseWriter http.ResponseWriter, request *http.Request) {
		if runtime == nil {
			writeJSON(responseWriter, runtimeStatusDocument{})
			return
		}
		runtime.refreshLocalLLM(request.Context())
		writeJSON(responseWriter, runtime.snapshot())
	})
	multiplexer.HandleFunc("POST /v1/runtime/local-llm", func(responseWriter http.ResponseWriter, request *http.Request) {
		if runtime == nil || localLLM == nil {
			http.Error(responseWriter, "runtime local LLM is unavailable", http.StatusServiceUnavailable)
			return
		}
		settings, errorValue := decodeControlLocalLLMSettings(request, httpClient)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		localLLM.update(settings)
		runtime.replaceLocalLLM(settings)
		writeJSON(responseWriter, runtime.refreshLocalLLM(request.Context()))
	})
	multiplexer.HandleFunc("GET /v1/security/grants", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		writeJSON(responseWriter, grantListDocument{Grants: grantStore.ListActive()})
	})
	multiplexer.HandleFunc("POST /v1/security/grants/{grantID}/revoke", func(responseWriter http.ResponseWriter, request *http.Request) {
		grantID := request.PathValue("grantID")
		if grantID == "" || !grantStore.Revoke(grantID) {
			http.Error(responseWriter, "grant not found", http.StatusNotFound)
			return
		}
		writeJSON(responseWriter, map[string]bool{"revoked": true})
	})
	multiplexer.HandleFunc("GET /v1/filesystem/mounts", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		writeJSON(responseWriter, mountListDocument{Mounts: mountStore.List()})
	})
	multiplexer.HandleFunc("POST /v1/filesystem/mounts", func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload struct {
			Path        string `json:"path"`
			DisplayName string `json:"displayName"`
		}
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		mount, errorValue := mountStore.Create(payload.Path, payload.DisplayName)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(responseWriter, mount)
	})
	multiplexer.HandleFunc("DELETE /v1/filesystem/mounts/{mountID}", func(responseWriter http.ResponseWriter, request *http.Request) {
		mount, errorValue := mountStore.Revoke(request.PathValue("mountID"))
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusNotFound)
			return
		}
		writeJSON(responseWriter, mount)
	})
	multiplexer.HandleFunc("POST /v1/filesystem/mounts/{mountID}/pause", func(responseWriter http.ResponseWriter, request *http.Request) {
		mount, errorValue := mountStore.Pause(request.PathValue("mountID"))
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusNotFound)
			return
		}
		writeJSON(responseWriter, mount)
	})
	multiplexer.HandleFunc("POST /v1/filesystem/mounts/{mountID}/resume", func(responseWriter http.ResponseWriter, request *http.Request) {
		mount, errorValue := mountStore.Resume(request.PathValue("mountID"))
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusNotFound)
			return
		}
		writeJSON(responseWriter, mount)
	})
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if !isLoopbackRemoteAddress(request.RemoteAddr) {
			http.Error(responseWriter, "loopback only", http.StatusForbidden)
			return
		}
		multiplexer.ServeHTTP(responseWriter, request)
	})
}

func decodeControlLocalLLMSettings(request *http.Request, httpClient *http.Client) (localLLMSettings, error) {
	var payload controlLocalLLMSettings
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return localLLMSettings{}, errorValue
	}
	if errorValue := validateControlLocalLLMSettings(payload); errorValue != nil {
		return localLLMSettings{}, errorValue
	}
	configuration := llmbackend.LocalProviderConfig{
		ProviderOrder:            payload.LocalBackendOrder,
		HTTPClient:               httpClient,
		OllamaBaseURL:            payload.Ollama.BaseURL,
		OllamaModel:              payload.Ollama.Model,
		LlamaCppEmbeddingBaseURL: payload.LlamaCpp.BaseURL,
		LlamaCppEmbeddingModel:   firstNonEmpty(payload.LlamaCpp.EmbeddingModel, llmbackend.DefaultEmbeddingModelName),
		LlamaCppBaseURL:          payload.LlamaCpp.BaseURL,
		LlamaCppModel:            payload.LlamaCpp.Model,
		MLXBaseURL:               payload.MLX.BaseURL,
		MLXModel:                 payload.MLX.Model,
	}
	if len(configuration.ProviderOrder) == 0 {
		configuration.ProviderOrder = llmbackend.DefaultCompanionLocalProviderOrder
	}
	return localLLMSettings{
		Enabled:       payload.EnableLocalLLM,
		Configuration: configuration,
		ProviderSet:   llmbackend.BuildLocalProviderSet(configuration),
	}, nil
}

func validateControlLocalLLMSettings(settings controlLocalLLMSettings) error {
	for _, name := range settings.LocalBackendOrder {
		switch name {
		case "ollama", "llamacpp", "mlx":
		default:
			return errors.New("unsupported backend: " + name)
		}
	}
	if errorValue := validateControlEndpoint("ollama", settings.Ollama); errorValue != nil {
		return errorValue
	}
	if errorValue := validateControlEndpoint("llamacpp", settings.LlamaCpp); errorValue != nil {
		return errorValue
	}
	return validateControlEndpoint("mlx", settings.MLX)
}

func validateControlEndpoint(name string, endpoint controlBackendEndpoint) error {
	trimmedURL := strings.TrimSpace(endpoint.BaseURL)
	if trimmedURL == "" {
		return nil
	}
	if strings.HasPrefix(trimmedURL, "http://") || strings.HasPrefix(trimmedURL, "https://") {
		return nil
	}
	return errors.New(name + " base URL must start with http:// or https://")
}

func isLoopbackRemoteAddress(remoteAddress string) bool {
	host, _, errorValue := net.SplitHostPort(remoteAddress)
	if errorValue != nil {
		return false
	}
	parsedIP := net.ParseIP(host)
	return parsedIP != nil && parsedIP.IsLoopback()
}

func listenLoopbackOnly(network string, address string, serverLabel string) (net.Listener, error) {
	listener, errorValue := net.Listen(network, address)
	if errorValue != nil {
		return nil, errorValue
	}
	tcpAddress, isTCPAddress := listener.Addr().(*net.TCPAddr)
	if !isTCPAddress || !tcpAddress.IP.IsLoopback() {
		_ = listener.Close()
		return nil, errors.New(serverLabel + " must listen on loopback")
	}
	return listener, nil
}

func localBackendsSummary(backends []llmbackend.Backend) localLLMStatus {
	statuses := make([]localLLMBackendStatus, 0, len(backends))
	for _, backend := range backends {
		statuses = append(statuses, localLLMBackendStatus{
			Name:      backend.Name(),
			Available: false,
		})
	}
	return localLLMStatus{Enabled: true, Backends: statuses}
}
