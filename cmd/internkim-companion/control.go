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

func (state *runtimeState) setLocalLLM(summary localLLMStatus) {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	state.localLLM = summary
}

func (state *runtimeState) registerLocalLLMBackends(backends []llmbackend.Backend, modelByName map[string]string) {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	state.localLLMBackends = backends
	state.localLLMModelByName = modelByName
	if state.localLLMCacheLifetime == 0 {
		state.localLLMCacheLifetime = defaultLocalLLMCacheLifetime
	}
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

func startControlServer(listenAddress string, grantStore *companionruntime.MemoryGrantStore, mountStore *companionruntime.MountStore, runtime *runtimeState) (*http.Server, error) {
	trimmedAddress := strings.TrimSpace(listenAddress)
	if trimmedAddress == "" {
		return nil, nil
	}
	listener, errorValue := net.Listen("tcp", trimmedAddress)
	if errorValue != nil {
		return nil, errorValue
	}
	if !listener.Addr().(*net.TCPAddr).IP.IsLoopback() {
		_ = listener.Close()
		return nil, errors.New("companion control server must listen on loopback")
	}
	server := &http.Server{Handler: controlHandler(grantStore, mountStore, runtime)}
	go func() {
		errorValue := server.Serve(listener)
		if errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
			_ = errorValue
		}
	}()
	return server, nil
}

func controlHandler(grantStore *companionruntime.MemoryGrantStore, mountStore *companionruntime.MountStore, runtime *runtimeState) http.Handler {
	multiplexer := http.NewServeMux()
	multiplexer.HandleFunc("GET /v1/runtime/status", func(responseWriter http.ResponseWriter, request *http.Request) {
		if runtime == nil {
			writeJSON(responseWriter, runtimeStatusDocument{})
			return
		}
		runtime.refreshLocalLLM(request.Context())
		writeJSON(responseWriter, runtime.snapshot())
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

func isLoopbackRemoteAddress(remoteAddress string) bool {
	host, _, errorValue := net.SplitHostPort(remoteAddress)
	if errorValue != nil {
		return false
	}
	parsedIP := net.ParseIP(host)
	return parsedIP != nil && parsedIP.IsLoopback()
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
