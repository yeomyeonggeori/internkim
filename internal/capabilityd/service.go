package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
	"github.com/yeomyeonggeori/internkim/internal/modelladder"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

type Configuration struct {
	SocketPath                    string
	OpenRouterKeyPath             string
	BlueclawBaseURL               string
	AdmindBaseURL                 string
	AdmindSocketPath              string
	OpenRouterBaseURL             string
	OpenRouterModel               string
	ForceOpenRouterModel          bool
	OpenRouterGatewaySecretPath   string
	OpenRouterGatewaySecretHeader string
	OpenRouterWebBaseURL          string
	EmbeddingServerURL            string
	OpenRouterImageModel          string
	SocketGroupName               string
	LocalOnly                     bool
	ProviderAttemptTimeout        time.Duration
	AgentBrowserPath              string
	DeviceBrowserExecutablePath   string
	DeviceBrowserStateDirectory   string
	DeviceBrowserFirstPort        int
	DeviceBrowserCapacity         int
	DeviceBrowserUserName         string
	AttachmentFileDirectory       string
	APIURLPath                    string
	BlueclawWorkspacePath         string
	FileReadPythonPath            string
	ChatdEndpoint                 string
	ChatdPlatform                 string
}

type Service struct {
	Configuration    Configuration
	HTTPClient       *http.Client
	RunCommand       func(context.Context, string, []string, []byte) ([]byte, error)
	LookupExecutable func(string) (string, error)
	ProgressManager  *platformProgressManager
	HealthState      *platformHealthState
	DeviceBrowsers   *browserruntime.DeviceBrowsers
}

type platformHealthState struct {
	mutex                   sync.RWMutex
	LastSuccessfulForwardAt time.Time `json:"lastSuccessfulForwardAt,omitempty"`
	LastForwardError        string    `json:"lastForwardError,omitempty"`
}

func DefaultConfiguration() Configuration {
	return Configuration{
		SocketPath:                    "/run/internkim/capability.sock",
		OpenRouterKeyPath:             "/root/.internkim/secrets/openrouter-api-key",
		BlueclawBaseURL:               "http://127.0.0.1:8080",
		AdmindBaseURL:                 "http://127.0.0.1:18080",
		AdmindSocketPath:              blueclaw.AdmindSocketPath,
		OpenRouterBaseURL:             "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterModel:               blueclaw.BlueclawDefaultModelName,
		OpenRouterGatewaySecretHeader: "X-INTERNKIM-GATEWAY-SECRET",
		OpenRouterWebBaseURL:          "https://openrouter.ai/api/v1/chat/completions",
		EmbeddingServerURL:            "http://" + blueclaw.EmbeddingListenAddress,
		OpenRouterImageModel:          modelladder.ImageModel,
		SocketGroupName:               "blueclaw",
		LocalOnly:                     false,
		ProviderAttemptTimeout:        0,
		AgentBrowserPath:              "agent-browser",
		DeviceBrowserExecutablePath:   browserruntime.DeviceBrowserExecutablePath,
		DeviceBrowserStateDirectory:   browserruntime.DeviceBrowsersStateDirectory,
		DeviceBrowserFirstPort:        browserruntime.DeviceBrowsersFirstPort,
		DeviceBrowserCapacity:         browserruntime.DeviceBrowsersCapacity,
		DeviceBrowserUserName:         browserruntime.DeviceBrowsersUserName,
		AttachmentFileDirectory:       "/tmp/internkim-attachment-files",
		APIURLPath:                    "/root/.internkim/env/api-url",
		BlueclawWorkspacePath:         "/root/.blueclaw/workspace",
		FileReadPythonPath:            blueclaw.LinuxCompanyHostLayout().DocumentPythonPath(),
	}
}

func (service Service) Run(ctx context.Context) error {
	if service.HealthState == nil {
		service.HealthState = &platformHealthState{}
	}
	if service.DeviceBrowsers == nil {
		service.DeviceBrowsers = service.Configuration.WithDefaults().newDeviceBrowsers()
	}
	go service.DeviceBrowsers.KeepTidy(ctx)
	listener, errorValue := service.listen()
	if errorValue != nil {
		return errorValue
	}
	defer listener.Close()

	server := &http.Server{
		Handler:           service.router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	errorValue = server.Serve(listener)
	if errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
		return errorValue
	}
	return nil
}

func (service Service) router() http.Handler {
	multiplexer := http.NewServeMux()
	multiplexer.HandleFunc("POST /v1/llm/structured", handleLLMRoute(service, Service.completeStructured))
	multiplexer.HandleFunc("POST /v1/llm/decide", handleLLMRoute(service, Service.decide))
	multiplexer.HandleFunc("POST /v1/llm/chat", handleLLMRoute(service, Service.completeChat))
	multiplexer.HandleFunc("POST /v1/llm/text", handleLLMRoute(service, Service.completeText))
	multiplexer.HandleFunc("POST /v1/embedding/create", service.handleEmbeddingCreate)
	multiplexer.HandleFunc("POST /v1/directory/person", service.handleDirectoryPerson)
	multiplexer.HandleFunc("POST /v1/tools/{toolName}/invoke", service.handleToolInvoke)
	multiplexer.HandleFunc("POST /v1/tools/{toolName}/target.resolve", service.handleToolTargetResolve)
	multiplexer.HandleFunc("GET /v1/capabilities", service.handleCapabilities)
	multiplexer.HandleFunc("GET /health", service.handleHealth)
	return multiplexer
}

func (service Service) handleHealth(responseWriter http.ResponseWriter, request *http.Request) {
	health := service.platformHealth(request.Context())
	statusCode := http.StatusOK
	if health["status"] != "ok" {
		statusCode = http.StatusServiceUnavailable
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(health)
}

func (service Service) platformHealth(ctx context.Context) map[string]any {
	protocolIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	return map[string]any{
		"status":                "ok",
		"protocolVersion":       protocolIdentity.ProtocolVersion,
		"aggregateProtocolHash": protocolIdentity.AggregateProtocolHash,
		"providers":             service.providerHealth(ctx),
		"checkedAt":             time.Now().UTC(),
	}
}

func (service Service) handleToolInvoke(responseWriter http.ResponseWriter, request *http.Request) {
	toolName := strings.TrimSpace(request.PathValue("toolName"))
	if toolName == "" {
		http.Error(responseWriter, "tool name is required", http.StatusBadRequest)
		return
	}
	response, errorValue := service.invokeCapabilityTool(request.Context(), toolName, request.Body)
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleToolTargetResolve(responseWriter http.ResponseWriter, request *http.Request) {
	toolName := strings.TrimSpace(request.PathValue("toolName"))
	if toolName == "" {
		http.Error(responseWriter, "tool name is required", http.StatusBadRequest)
		return
	}
	response, errorValue := service.resolveCapabilityToolTarget(request.Context(), toolName, request.Body)
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleCapabilities(responseWriter http.ResponseWriter, request *http.Request) {
	response, errorValue := service.capabilityRegistry(request.Context())
	service.writeResponse(responseWriter, response, errorValue)
}

func handleLLMRoute[RequestBody any, ResponseBody any](service Service, complete func(Service, context.Context, RequestBody) (ResponseBody, error)) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		var requestBody RequestBody
		if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		capturingService := service
		capturedClient, capture := llmbackend.NewExchangeCapture(capturingService.providerHTTPClient())
		capturingService.HTTPClient = capturedClient
		response, errorValue := complete(capturingService, request.Context(), requestBody)
		capturingService.writeLLMResponse(responseWriter, response, errorValue, requestBody, capture)
	}
}

func (service Service) handleEmbeddingCreate(responseWriter http.ResponseWriter, request *http.Request) {
	var embeddingRequest EmbeddingRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&embeddingRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	response, errorValue := service.createEmbedding(request.Context(), embeddingRequest)
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) writeResponse(responseWriter http.ResponseWriter, response any, errorValue error) {
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, response)
}

func (service Service) writeJSON(responseWriter http.ResponseWriter, response any) {
	body, errorValue := json.Marshal(response)
	responseWriter.Header().Set("Content-Type", "application/json")
	if errorValue != nil {
		log.Printf("capability.response_serialization_failed: type=%T error=%v", response, errorValue)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		_, _ = responseWriter.Write([]byte(`{"error":"capability response serialization failed"}`))
		return
	}
	if _, errorValue := responseWriter.Write(append(body, '\n')); errorValue != nil {
		log.Printf("capability.response_write_failed: type=%T error=%v", response, errorValue)
	}
}

func (state *platformHealthState) Snapshot() platformHealthState {
	if state == nil {
		return platformHealthState{}
	}
	state.mutex.RLock()
	defer state.mutex.RUnlock()
	return platformHealthState{
		LastSuccessfulForwardAt: state.LastSuccessfulForwardAt,
		LastForwardError:        state.LastForwardError,
	}
}

func (state *platformHealthState) Update(update func(*platformHealthState)) {
	if state == nil {
		return
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	update(state)
}

func (service Service) listen() (net.Listener, error) {
	socketPath := service.Configuration.SocketPath
	if socketPath == "" {
		socketPath = DefaultConfiguration().SocketPath
	}
	_ = os.Remove(socketPath)
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.Chmod(socketPath, blueclaw.CapabilitySocketMode); errorValue != nil {
		_ = listener.Close()
		return nil, errorValue
	}
	groupID, errorValue := lookupGroupID(service.Configuration.SocketGroupName)
	if errorValue == nil {
		_ = os.Chown(socketPath, 0, groupID)
	}
	return listener, nil
}

func (service Service) httpClient() *http.Client {
	if service.HTTPClient != nil {
		return service.HTTPClient
	}
	return &http.Client{Timeout: service.httpClientTimeout()}
}

func (service Service) providerHTTPClient() *http.Client {
	if service.HTTPClient != nil {
		return service.HTTPClient
	}
	if service.Configuration.ProviderAttemptTimeout <= 0 {
		return &http.Client{}
	}
	return &http.Client{Timeout: service.Configuration.ProviderAttemptTimeout + 30*time.Second}
}

func (service Service) httpClientTimeout() time.Duration {
	providerTimeout := service.Configuration.ProviderAttemptTimeout
	if providerTimeout <= 0 {
		providerTimeout = DefaultConfiguration().ProviderAttemptTimeout
	}
	if providerTimeout <= 120*time.Second {
		return 120 * time.Second
	}
	return providerTimeout + 30*time.Second
}

func readSecretValue(path string) string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	value := strings.TrimSpace(string(document))
	return strings.TrimPrefix(value, "OPENROUTER_API_KEY=")
}

func isPlaceholderOpenRouterKey(value string) bool {
	normalizedValue := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(normalizedValue, "internkim-simulation-openrouter-api-key") ||
		strings.Contains(normalizedValue, "simulation-openrouter")
}

func lookupGroupID(name string) (int, error) {
	group, errorValue := user.LookupGroup(name)
	if errorValue != nil {
		return -1, errorValue
	}
	return strconv.Atoi(group.Gid)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}

const platformProgressTTL = 3 * time.Minute

type platformProgressManager struct {
	mutex      sync.Mutex
	leaseByKey map[string]*platformProgressLease
}

type platformProgressLease struct {
	cancel context.CancelFunc
	timer  *time.Timer
}

func newPlatformProgressManager() *platformProgressManager {
	return &platformProgressManager{leaseByKey: map[string]*platformProgressLease{}}
}

func (manager *platformProgressManager) Start(key string, maximumLifetime time.Duration, run func(context.Context)) {
	if maximumLifetime <= 0 {
		maximumLifetime = platformProgressTTL
	}
	manager.mutex.Lock()
	if lease, isFound := manager.leaseByKey[key]; isFound {
		lease.timer.Reset(maximumLifetime)
		manager.mutex.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	lease := &platformProgressLease{cancel: cancel}
	lease.timer = time.AfterFunc(maximumLifetime, func() {
		manager.Stop(key)
	})
	manager.leaseByKey[key] = lease
	manager.mutex.Unlock()

	go run(ctx)
}

func (manager *platformProgressManager) Stop(key string) {
	manager.mutex.Lock()
	lease, isFound := manager.leaseByKey[key]
	if isFound {
		delete(manager.leaseByKey, key)
	}
	manager.mutex.Unlock()
	if isFound {
		lease.timer.Stop()
		lease.cancel()
	}
}

func (configuration Configuration) WithDefaults() Configuration {
	defaultConfiguration := DefaultConfiguration()
	if configuration.SocketPath == "" {
		configuration.SocketPath = defaultConfiguration.SocketPath
	}
	if configuration.OpenRouterKeyPath == "" {
		configuration.OpenRouterKeyPath = defaultConfiguration.OpenRouterKeyPath
	}
	if configuration.BlueclawBaseURL == "" {
		configuration.BlueclawBaseURL = defaultConfiguration.BlueclawBaseURL
	}
	if configuration.AdmindBaseURL == "" {
		configuration.AdmindBaseURL = defaultConfiguration.AdmindBaseURL
	}
	if configuration.AdmindSocketPath == "" {
		configuration.AdmindSocketPath = defaultConfiguration.AdmindSocketPath
	}
	if configuration.OpenRouterBaseURL == "" {
		configuration.OpenRouterBaseURL = defaultConfiguration.OpenRouterBaseURL
	}
	if configuration.OpenRouterGatewaySecretHeader == "" {
		configuration.OpenRouterGatewaySecretHeader = defaultConfiguration.OpenRouterGatewaySecretHeader
	}
	if configuration.OpenRouterModel == "" {
		configuration.OpenRouterModel = defaultConfiguration.OpenRouterModel
	}
	if configuration.OpenRouterWebBaseURL == "" {
		configuration.OpenRouterWebBaseURL = defaultConfiguration.OpenRouterWebBaseURL
	}
	if configuration.EmbeddingServerURL == "" {
		configuration.EmbeddingServerURL = defaultConfiguration.EmbeddingServerURL
	}
	if configuration.OpenRouterImageModel == "" {
		configuration.OpenRouterImageModel = defaultConfiguration.OpenRouterImageModel
	}
	if configuration.SocketGroupName == "" {
		configuration.SocketGroupName = defaultConfiguration.SocketGroupName
	}
	if configuration.AgentBrowserPath == "" {
		configuration.AgentBrowserPath = defaultConfiguration.AgentBrowserPath
	}
	if configuration.DeviceBrowserExecutablePath == "" {
		configuration.DeviceBrowserExecutablePath = defaultConfiguration.DeviceBrowserExecutablePath
	}
	if configuration.DeviceBrowserStateDirectory == "" {
		configuration.DeviceBrowserStateDirectory = defaultConfiguration.DeviceBrowserStateDirectory
	}
	if configuration.DeviceBrowserFirstPort <= 0 {
		configuration.DeviceBrowserFirstPort = defaultConfiguration.DeviceBrowserFirstPort
	}
	if configuration.DeviceBrowserCapacity <= 0 {
		configuration.DeviceBrowserCapacity = defaultConfiguration.DeviceBrowserCapacity
	}
	if configuration.DeviceBrowserUserName == "" {
		configuration.DeviceBrowserUserName = defaultConfiguration.DeviceBrowserUserName
	}
	if configuration.AttachmentFileDirectory == "" {
		configuration.AttachmentFileDirectory = defaultConfiguration.AttachmentFileDirectory
	}
	if configuration.APIURLPath == "" {
		configuration.APIURLPath = defaultConfiguration.APIURLPath
	}
	if configuration.BlueclawWorkspacePath == "" {
		configuration.BlueclawWorkspacePath = defaultConfiguration.BlueclawWorkspacePath
	}
	if configuration.FileReadPythonPath == "" {
		configuration.FileReadPythonPath = defaultConfiguration.FileReadPythonPath
	}
	return configuration
}

func Run(configuration Configuration) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	service := Service{Configuration: configuration.WithDefaults()}
	if errorValue := service.Run(ctx); errorValue != nil {
		return fmt.Errorf("capabilityd failed: %w", errorValue)
	}
	return nil
}
