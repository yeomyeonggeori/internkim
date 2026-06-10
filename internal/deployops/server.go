package deployops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/localfleet"
)

type Server struct {
	options  ServerOptions
	registry TargetRegistry
	jobs     *JobStore
	client   *http.Client
	uiProxy  *httputil.ReverseProxy
	uiServer *exec.Cmd
}

func Serve(contextValue context.Context, options ServerOptions) error {
	normalizedOptions, errorValue := normalizeServerOptions(options)
	if errorValue != nil {
		return errorValue
	}
	registry, errorValue := LoadRegistry(normalizedOptions.RepositoryRootPath, normalizedOptions.InternKimHomePath)
	if errorValue != nil {
		return errorValue
	}
	server := &Server{
		options:  normalizedOptions,
		registry: registry,
		jobs:     NewJobStore(),
		client:   &http.Client{Timeout: 8 * time.Second},
	}
	if normalizedOptions.EnableSvelteUI {
		if errorValue := server.startSvelteUI(contextValue); errorValue != nil {
			return errorValue
		}
	}
	httpServer := &http.Server{
		Addr:              normalizedOptions.ListenAddress,
		Handler:           server.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("Ops console: http://%s\n", normalizedOptions.ListenAddress)
	errorChannel := make(chan error, 1)
	go func() {
		errorChannel <- httpServer.ListenAndServe()
	}()
	select {
	case <-contextValue.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownContext)
		server.stopSvelteUI()
		return contextValue.Err()
	case errorValue := <-errorChannel:
		server.stopSvelteUI()
		if errors.Is(errorValue, http.ErrServerClosed) {
			return nil
		}
		return errorValue
	}
}

func normalizeServerOptions(options ServerOptions) (ServerOptions, error) {
	if strings.TrimSpace(options.ListenAddress) == "" {
		options.ListenAddress = "127.0.0.1:8789"
	}
	if !isLoopbackAddress(options.ListenAddress) {
		return options, fmt.Errorf("ops serve only supports loopback bind addresses, got %q", options.ListenAddress)
	}
	if strings.TrimSpace(options.RepositoryRootPath) == "" {
		repositoryRootPath, errorValue := os.Getwd()
		if errorValue != nil {
			return options, errorValue
		}
		options.RepositoryRootPath = repositoryRootPath
	}
	if strings.TrimSpace(options.InternKimHomePath) == "" {
		homePath, errorValue := os.UserHomeDir()
		if errorValue != nil {
			return options, errorValue
		}
		options.InternKimHomePath = filepath.Join(homePath, ".internkim")
	}
	if strings.TrimSpace(options.ExecutablePath) == "" {
		executablePath, errorValue := os.Executable()
		if errorValue != nil {
			return options, errorValue
		}
		options.ExecutablePath = executablePath
	}
	return options, nil
}

func isLoopbackAddress(address string) bool {
	host, _, errorValue := net.SplitHostPort(address)
	if errorValue != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	parsedIP := net.ParseIP(host)
	return parsedIP != nil && parsedIP.IsLoopback()
}

func (server *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/api/locale", server.handleLocale)
	mux.HandleFunc("/api/targets", server.handleTargets)
	mux.HandleFunc("/api/targets/", server.handleTargetResource)
	mux.HandleFunc("/api/local-fleet/status", server.handleLocalFleetStatus)
	mux.HandleFunc("/api/local-fleet/jobs", server.handleLocalFleetJobs)
	mux.HandleFunc("/api/jobs/", server.handleJobResource)
	mux.HandleFunc("/", server.handleUI)
	return mux
}

func (server *Server) handleLocale(responseWriter http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(responseWriter, http.StatusOK, map[string]string{"locale": "ko"})
	case http.MethodPut:
		writeJSON(responseWriter, http.StatusOK, map[string]string{"locale": "ko"})
	default:
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (server *Server) handleTargets(responseWriter http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		writeJSON(responseWriter, http.StatusOK, server.registry.Targets)
	case http.MethodPost:
		server.handleCreateTarget(responseWriter, request)
	default:
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (server *Server) handleCreateTarget(responseWriter http.ResponseWriter, request *http.Request) {
	var target Target
	if errorValue := json.NewDecoder(request.Body).Decode(&target); errorValue != nil {
		writeError(responseWriter, http.StatusBadRequest, errorValue.Error())
		return
	}
	target = normalizeTarget(target)
	if target.AdminURL == "" {
		writeError(responseWriter, http.StatusBadRequest, "adminURL is required")
		return
	}
	server.registry.Targets = append(server.registry.Targets, target)
	server.registry = normalizeRegistry(server.registry)
	if errorValue := SaveRegistry(server.options.RepositoryRootPath, server.registry); errorValue != nil {
		writeError(responseWriter, http.StatusInternalServerError, errorValue.Error())
		return
	}
	writeJSON(responseWriter, http.StatusCreated, target)
}

func (server *Server) handleTargetResource(responseWriter http.ResponseWriter, request *http.Request) {
	targetID, suffix := splitResourcePath(strings.TrimPrefix(request.URL.Path, "/api/targets/"))
	target, ok := server.findTarget(targetID)
	if !ok {
		writeError(responseWriter, http.StatusNotFound, "target not found")
		return
	}
	switch {
	case request.Method == http.MethodGet && suffix == "status":
		writeJSON(responseWriter, http.StatusOK, server.CheckStatus(request.Context(), target))
	case request.Method == http.MethodGet && suffix == "llm-model":
		writeJSON(responseWriter, http.StatusOK, server.ReadLLMModel(request.Context(), target))
	case request.Method == http.MethodPut && suffix == "llm-model":
		server.handleUpdateLLMModel(responseWriter, request, target)
	case request.Method == http.MethodPost && suffix == "jobs":
		server.handleCreateJob(responseWriter, request, target)
	default:
		writeError(responseWriter, http.StatusNotFound, "resource not found")
	}
}

func (server *Server) handleUpdateLLMModel(responseWriter http.ResponseWriter, request *http.Request, target Target) {
	var payload UpdateLLMModelRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeError(responseWriter, http.StatusBadRequest, errorValue.Error())
		return
	}
	status, errorValue := server.UpdateLLMModel(request.Context(), target, payload.Model)
	if errorValue != nil {
		writeError(responseWriter, http.StatusBadRequest, Redact(errorValue.Error()))
		return
	}
	writeJSON(responseWriter, http.StatusOK, status)
}

func (server *Server) handleCreateJob(responseWriter http.ResponseWriter, request *http.Request, target Target) {
	var jobRequest JobRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&jobRequest); errorValue != nil {
		writeError(responseWriter, http.StatusBadRequest, errorValue.Error())
		return
	}
	job, errorValue := server.jobs.Start(target.ID, jobRequest.Action, func(job *JobRunner) {
		server.runJob(request.Context(), job, target, jobRequest.Action)
	})
	if errorValue != nil {
		writeError(responseWriter, http.StatusConflict, errorValue.Error())
		return
	}
	writeJSON(responseWriter, http.StatusAccepted, job)
}

func (server *Server) handleLocalFleetStatus(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	service, errorValue := server.localFleetService()
	if errorValue != nil {
		writeError(responseWriter, http.StatusInternalServerError, errorValue.Error())
		return
	}
	writeJSON(responseWriter, http.StatusOK, service.Status(request.Context()))
}

func (server *Server) handleLocalFleetJobs(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var jobRequest localfleet.JobRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&jobRequest); errorValue != nil {
		writeError(responseWriter, http.StatusBadRequest, errorValue.Error())
		return
	}
	job, errorValue := server.jobs.Start("local-fleet", "local-fleet:"+jobRequest.Action, func(job *JobRunner) {
		server.runLocalFleetJob(request.Context(), job, jobRequest)
	})
	if errorValue != nil {
		writeError(responseWriter, http.StatusConflict, errorValue.Error())
		return
	}
	writeJSON(responseWriter, http.StatusAccepted, job)
}

func (server *Server) handleJobResource(responseWriter http.ResponseWriter, request *http.Request) {
	jobID, suffix := splitResourcePath(strings.TrimPrefix(request.URL.Path, "/api/jobs/"))
	if suffix == "events" {
		server.jobs.Stream(responseWriter, request, jobID)
		return
	}
	job, ok := server.jobs.Get(jobID)
	if !ok {
		writeError(responseWriter, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(responseWriter, http.StatusOK, job)
}

func (server *Server) handleUI(responseWriter http.ResponseWriter, request *http.Request) {
	if server.uiProxy == nil {
		writeError(responseWriter, http.StatusServiceUnavailable, "Svelte UI is not enabled")
		return
	}
	if request.URL.Path == "/" && !isWebSocketUpgrade(request) {
		http.Redirect(responseWriter, request, "/ops", http.StatusTemporaryRedirect)
		return
	}
	server.uiProxy.ServeHTTP(responseWriter, request)
}

func isWebSocketUpgrade(request *http.Request) bool {
	return strings.EqualFold(request.Header.Get("Upgrade"), "websocket")
}

func (server *Server) findTarget(targetID string) (Target, bool) {
	for _, target := range server.registry.Targets {
		if target.ID == targetID {
			return target, true
		}
	}
	return Target{}, false
}

func (server *Server) localFleetService() (localfleet.Service, error) {
	return localfleet.NewService(localfleet.Options{
		RepositoryRootPath: server.options.RepositoryRootPath,
		ExecutablePath:     server.options.ExecutablePath,
	})
}

func splitResourcePath(path string) (string, string) {
	parts := strings.SplitN(strings.Trim(path, "/"), "/", 2)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func (server *Server) startSvelteUI(contextValue context.Context) error {
	port, errorValue := availablePort()
	if errorValue != nil {
		return errorValue
	}
	targetURL, errorValue := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if errorValue != nil {
		return errorValue
	}
	command := exec.CommandContext(contextValue, "bun", "run", "dev", "--", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port))
	command.Dir = filepath.Join(server.options.RepositoryRootPath, "web")
	command.Env = append(os.Environ(), "BROWSER=none")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if errorValue := command.Start(); errorValue != nil {
		return fmt.Errorf("start Svelte ops UI: %w", errorValue)
	}
	server.uiServer = command
	server.uiProxy = httputil.NewSingleHostReverseProxy(targetURL)
	return waitForHTTP(targetURL.String()+"/ops", 15*time.Second)
}

func (server *Server) stopSvelteUI() {
	if server.uiServer == nil || server.uiServer.Process == nil {
		return
	}
	_ = server.uiServer.Process.Kill()
	_, _ = server.uiServer.Process.Wait()
}

func availablePort() (int, error) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		return 0, errorValue
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForHTTP(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		response, errorValue := http.Get(address)
		if errorValue == nil {
			_ = response.Body.Close()
			if response.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("Svelte ops UI did not become ready at %s", address)
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, value any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(value)
}

func writeError(responseWriter http.ResponseWriter, statusCode int, message string) {
	writeJSON(responseWriter, statusCode, map[string]string{"error": message})
}
