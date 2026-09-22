package capabilityd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
	"gitlab.com/eastriver/internkim/internal/modelladder"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

type Configuration struct {
	SocketPath                    string
	VSockPort                     int
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
	OpenRouterEmbeddingBaseURL    string
	OpenRouterEmbeddingModel      string
	OpenRouterImageModel          string
	EmbeddingProviderOrder        []string
	OllamaBaseURL                 string
	OllamaModel                   string
	LlamaCppBaseURL               string
	LlamaCppModel                 string
	LlamaCppEmbeddingBaseURL      string
	LlamaCppEmbeddingModel        string
	SocketGroupName               string
	LiteRTModelPath               string
	LocalLLMRunnerPath            string
	CompanionBaseURL              string
	PreferCompanionLLM            bool
	LocalInferenceMode            string
	PreferCompanionBrowser        bool
	LocalOnly                     bool
	LocalBackendOrder             []string
	ProviderAttemptTimeout        time.Duration
	AgentBrowserPath              string
	DeviceBrowserExecutablePath   string
	DeviceBrowserStateDirectory   string
	DeviceBrowserFirstPort        int
	DeviceBrowserCapacity         int
	DeviceBrowserUserName         string
	CompanionFileDirectory        string
	APIURLPath                    string
	FleetIDPath                   string
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

type interactionResolveRequest struct {
	DispatchID string `json:"dispatchID"`
}

type reactionAddRequest struct {
	ConversationID string `json:"conversationID"`
	MessageID      string `json:"messageID"`
	EmojiName      string `json:"emojiName"`
	Reason         string `json:"reason"`
}

type platformAskInteraction struct {
	InteractionID        string                    `json:"interactionID"`
	TaskRunID            string                    `json:"taskRunID"`
	Kind                 string                    `json:"kind"`
	Message              string                    `json:"message,omitempty"`
	Question             string                    `json:"question,omitempty"`
	Options              []platformAskChoiceOption `json:"options,omitempty"`
	RecommendedOptionKey string                    `json:"recommendedOptionKey,omitempty"`
	SelectionMode        string                    `json:"selectionMode,omitempty"`
	ResponseLanguage     string                    `json:"responseLanguage,omitempty"`
	TargetPlatformUserID string                    `json:"targetPlatformUserID,omitempty"`
}

type platformAskChoiceOption struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	ShortLabel string `json:"shortLabel,omitempty"`
	Value      string `json:"value,omitempty"`
}

type platformHealthState struct {
	mutex                   sync.RWMutex
	LastSuccessfulForwardAt time.Time `json:"lastSuccessfulForwardAt,omitempty"`
	LastForwardError        string    `json:"lastForwardError,omitempty"`
}

var fallbackPlatformHealthState = &platformHealthState{}

type progressRequest struct {
	ReplyTargetID string `json:"replyTargetID"`
}

func DefaultConfiguration() Configuration {
	return Configuration{
		SocketPath:                    "/run/internkim/capability.sock",
		VSockPort:                     0,
		OpenRouterKeyPath:             "/root/.internkim/secrets/openrouter-api-key",
		BlueclawBaseURL:               "http://127.0.0.1:8080",
		AdmindBaseURL:                 "http://127.0.0.1:18080",
		AdmindSocketPath:              blueclaw.AdmindSocketPath,
		OpenRouterBaseURL:             "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterModel:               blueclaw.BlueclawDefaultModelName,
		OpenRouterGatewaySecretHeader: "X-INTERNKIM-GATEWAY-SECRET",
		OpenRouterWebBaseURL:          "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterEmbeddingBaseURL:    "https://openrouter.ai/api/v1/embeddings",
		OpenRouterEmbeddingModel:      llmbackend.DefaultEmbeddingModelName,
		OpenRouterImageModel:          modelladder.ImageModel,
		EmbeddingProviderOrder:        llmbackend.DefaultLocalEmbeddingProviderOrder,
		OllamaBaseURL:                 "http://127.0.0.1:11434",
		OllamaModel:                   "gemma3:1b",
		LlamaCppBaseURL:               locallm.LlamaCppBaseURL,
		LlamaCppModel:                 "local/gemma-4-E2B-it-qat-UD-Q4_K_XL",
		LlamaCppEmbeddingBaseURL:      locallm.LlamaCppEmbeddingBaseURL,
		LlamaCppEmbeddingModel:        llmbackend.DefaultEmbeddingModelName,
		SocketGroupName:               "blueclaw",
		LiteRTModelPath:               locallm.ModelPath(),
		LocalLLMRunnerPath:            "/usr/local/bin/internkim-local-llm-runner",
		CompanionBaseURL:              "",
		PreferCompanionLLM:            false,
		LocalInferenceMode:            "",
		LocalOnly:                     false,
		ProviderAttemptTimeout:        0,
		AgentBrowserPath:              "agent-browser",
		DeviceBrowserExecutablePath:   browserruntime.DeviceBrowserExecutablePath,
		DeviceBrowserStateDirectory:   browserruntime.DeviceBrowsersStateDirectory,
		DeviceBrowserFirstPort:        browserruntime.DeviceBrowsersFirstPort,
		DeviceBrowserCapacity:         browserruntime.DeviceBrowsersCapacity,
		DeviceBrowserUserName:         browserruntime.DeviceBrowsersUserName,
		CompanionFileDirectory:        "/tmp/internkim-companion-files",
		APIURLPath:                    "/root/.internkim/env/api-url",
		FleetIDPath:                   "/root/.internkim/env/fleet-id",
		BlueclawWorkspacePath:         "/root/.blueclaw/workspace",
		FileReadPythonPath:            blueclaw.CompanyPackageDocumentPythonPath,
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
	service.applyLocalInferenceMode(ctx)
	listener, errorValue := service.listen()
	if errorValue != nil {
		return errorValue
	}
	defer listener.Close()

	vsockListener, errorValue := service.listenVSock()
	if errorValue != nil {
		log.Printf("capabilityd vsock listener disabled: %v", errorValue)
		vsockListener = nil
	}
	if vsockListener != nil {
		defer vsockListener.Close()
	}

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
	if vsockListener != nil {
		go func() {
			errorValue := server.Serve(vsockListener)
			if errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
				log.Printf("capabilityd vsock listener stopped: %v", errorValue)
			}
		}()
	}

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
		capturedClient, capture := llmbackend.NewFailureCapture(capturingService.providerHTTPClient())
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

func (service Service) healthState() *platformHealthState {
	if service.HealthState != nil {
		return service.HealthState
	}
	return defaultPlatformHealthState()
}

func defaultPlatformHealthState() *platformHealthState {
	return fallbackPlatformHealthState
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

func numberedChoiceLabel(index int, label string) string {
	return strconv.Itoa(index+1) + ". " + strings.TrimSpace(label)
}

func trimNonEmptyPlatformAskOptions(options []platformAskChoiceOption) []platformAskChoiceOption {
	trimmedOptions := []platformAskChoiceOption{}
	for _, option := range options {
		option.Key = strings.TrimSpace(option.Key)
		option.Label = strings.TrimSpace(option.Label)
		option.ShortLabel = strings.TrimSpace(option.ShortLabel)
		option.Value = strings.TrimSpace(option.Value)
		if option.Key != "" && option.Label != "" {
			trimmedOptions = append(trimmedOptions, option)
		}
	}
	return trimmedOptions
}

func trimNonEmptyPlatformStrings(values []string) []string {
	trimmedValues := []string{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			trimmedValues = append(trimmedValues, trimmedValue)
		}
	}
	return trimmedValues
}

func randomCapabilityHex(size int) string {
	value := make([]byte, size)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
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
	if errorValue := os.Chmod(socketPath, 0o660); errorValue != nil {
		_ = listener.Close()
		return nil, errorValue
	}
	groupID, errorValue := lookupGroupID(service.Configuration.SocketGroupName)
	if errorValue == nil {
		_ = os.Chown(socketPath, 0, groupID)
	}
	return listener, nil
}

func (service Service) listenVSock() (net.Listener, error) {
	if service.Configuration.VSockPort <= 0 {
		return nil, nil
	}
	return listenVSock(service.Configuration.VSockPort)
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

func (service Service) runCommand(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
	if service.RunCommand != nil {
		return service.RunCommand(ctx, executablePath, arguments, standardInput)
	}

	return defaultCommandLimiter.Run(ctx, func() ([]byte, error) {
		commandContext, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		command := exec.CommandContext(commandContext, executablePath, arguments...)
		command.Stdin = bytes.NewReader(standardInput)
		output, errorValue := command.CombinedOutput()
		if errorValue != nil {
			return nil, fmt.Errorf("%s failed: %w: %s", executablePath, errorValue, strings.TrimSpace(string(output)))
		}
		return output, nil
	})
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

var fallbackPlatformProgressManager = newPlatformProgressManager()

const platformProgressTTL = 3 * time.Minute
const platformTypingInterval = 4 * time.Second

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

func (service Service) progressManager() *platformProgressManager {
	if service.ProgressManager != nil {
		return service.ProgressManager
	}
	return fallbackPlatformProgressManager
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
	if configuration.OpenRouterEmbeddingBaseURL == "" {
		configuration.OpenRouterEmbeddingBaseURL = defaultConfiguration.OpenRouterEmbeddingBaseURL
	}
	if configuration.OpenRouterEmbeddingModel == "" {
		configuration.OpenRouterEmbeddingModel = defaultConfiguration.OpenRouterEmbeddingModel
	}
	if configuration.OpenRouterImageModel == "" {
		configuration.OpenRouterImageModel = defaultConfiguration.OpenRouterImageModel
	}
	if len(configuration.EmbeddingProviderOrder) == 0 {
		configuration.EmbeddingProviderOrder = append([]string{}, defaultConfiguration.EmbeddingProviderOrder...)
	}
	if configuration.OllamaBaseURL == "" {
		configuration.OllamaBaseURL = defaultConfiguration.OllamaBaseURL
	}
	if configuration.OllamaModel == "" {
		configuration.OllamaModel = defaultConfiguration.OllamaModel
	}
	if configuration.LlamaCppBaseURL == "" {
		configuration.LlamaCppBaseURL = defaultConfiguration.LlamaCppBaseURL
	}
	if configuration.LlamaCppModel == "" {
		configuration.LlamaCppModel = defaultConfiguration.LlamaCppModel
	}
	if configuration.LlamaCppEmbeddingBaseURL == "" {
		configuration.LlamaCppEmbeddingBaseURL = defaultConfiguration.LlamaCppEmbeddingBaseURL
	}
	if configuration.LlamaCppEmbeddingModel == "" {
		configuration.LlamaCppEmbeddingModel = defaultConfiguration.LlamaCppEmbeddingModel
	}
	if configuration.SocketGroupName == "" {
		configuration.SocketGroupName = defaultConfiguration.SocketGroupName
	}
	if configuration.LiteRTModelPath == "" {
		configuration.LiteRTModelPath = defaultConfiguration.LiteRTModelPath
	}
	if configuration.LocalLLMRunnerPath == "" {
		configuration.LocalLLMRunnerPath = defaultConfiguration.LocalLLMRunnerPath
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
	if configuration.CompanionFileDirectory == "" {
		configuration.CompanionFileDirectory = defaultConfiguration.CompanionFileDirectory
	}
	if configuration.APIURLPath == "" {
		configuration.APIURLPath = defaultConfiguration.APIURLPath
	}
	if configuration.FleetIDPath == "" {
		configuration.FleetIDPath = defaultConfiguration.FleetIDPath
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
