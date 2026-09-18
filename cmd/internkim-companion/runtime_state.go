package main

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

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
