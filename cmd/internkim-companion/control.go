package main

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	companionruntime "github.com/anthropic-lab/internkim/internal/companion"
)

type grantListDocument struct {
	Grants []companionruntime.GrantSnapshot `json:"grants"`
}

type runtimeState struct {
	mutex           sync.Mutex
	lastHeartbeatAt string
	lastError       string
}

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

func (state *runtimeState) snapshot() map[string]string {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	return map[string]string{
		"lastHeartbeatAt": state.lastHeartbeatAt,
		"lastError":       state.lastError,
	}
}

func startControlServer(listenAddress string, grantStore *companionruntime.MemoryGrantStore, runtime *runtimeState) (*http.Server, error) {
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
	server := &http.Server{Handler: controlHandler(grantStore, runtime)}
	go func() {
		errorValue := server.Serve(listener)
		if errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
			_ = errorValue
		}
	}()
	return server, nil
}

func controlHandler(grantStore *companionruntime.MemoryGrantStore, runtime *runtimeState) http.Handler {
	multiplexer := http.NewServeMux()
	multiplexer.HandleFunc("GET /v1/runtime/status", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		if runtime == nil {
			writeJSON(responseWriter, map[string]string{})
			return
		}
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
