package capabilityd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
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
	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

type Configuration struct {
	SocketPath                    string
	VSockPort                     int
	OpenRouterKeyPath             string
	SlackTokenPath                string
	SlackAppTokenPath             string
	SignalJSONRPCURL              string
	SignalAccount                 string
	SignalJSONRPCURLPath          string
	SignalAccountPath             string
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
	DeviceBrowserPath             string
	DeviceBrowserProfilePath      string
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
	EventLocker      *platformEventLocker
	ProgressManager  *platformProgressManager
	HealthState      *platformHealthState
}

type userLookupRequest struct {
	SenderID string `json:"senderID"`
}

type historyFetchRequest struct {
	HistoryCursor string `json:"historyCursor"`
	Limit         int    `json:"limit"`
	Direction     string `json:"direction"`
}

type replyRequest struct {
	ReplyTargetID   string                        `json:"replyTargetID"`
	Message         string                        `json:"message"`
	RawEventID      string                        `json:"rawEventID,omitempty"`
	OutboxID        string                        `json:"outboxID,omitempty"`
	ReplyKind       string                        `json:"replyKind,omitempty"`
	Attachments     []platformFileSpec            `json:"attachments,omitempty"`
	RecoveryActions []capabilities.RecoveryAction `json:"recoveryActions,omitempty"`
	Interaction     *platformAskInteraction       `json:"interaction,omitempty"`
}

type platformReplyResult struct {
	Platform              string   `json:"platform"`
	DispatchID            string   `json:"dispatchID"`
	Visibility            string   `json:"visibility"`
	MessageDelivered      bool     `json:"messageDelivered"`
	NativeAttachmentCount int      `json:"nativeAttachmentCount"`
	NativeAttachmentIDs   []string `json:"nativeAttachmentIDs,omitempty"`
}

func newPlatformReplyResult(platform string, dispatchID string, visibility string, message string, nativeAttachmentIDs []string) platformReplyResult {
	return newPlatformReplyResultWithAttachmentCount(platform, dispatchID, visibility, message, len(nativeAttachmentIDs), nativeAttachmentIDs)
}

func newPlatformReplyResultWithAttachmentCount(platform string, dispatchID string, visibility string, message string, nativeAttachmentCount int, nativeAttachmentIDs []string) platformReplyResult {
	return platformReplyResult{
		Platform:              platform,
		DispatchID:            strings.TrimSpace(dispatchID),
		Visibility:            strings.TrimSpace(visibility),
		MessageDelivered:      strings.TrimSpace(message) != "",
		NativeAttachmentCount: nativeAttachmentCount,
		NativeAttachmentIDs:   nativeAttachmentIDs,
	}
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
		SlackTokenPath:                "/root/.internkim/secrets/slack-bot-token",
		SlackAppTokenPath:             "/root/.internkim/secrets/slack-app-token",
		SignalJSONRPCURLPath:          "/root/.internkim/config/signal-jsonrpc-url",
		SignalAccountPath:             "/root/.internkim/config/signal-account",
		BlueclawBaseURL:               "http://127.0.0.1:8080",
		AdmindBaseURL:                 "http://127.0.0.1:18080",
		AdmindSocketPath:              blueclaw.AdmindSocketPath,
		OpenRouterBaseURL:             "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterModel:               blueclaw.BlueclawDefaultModelName,
		OpenRouterGatewaySecretHeader: "X-INTERNKIM-GATEWAY-SECRET",
		OpenRouterWebBaseURL:          "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterEmbeddingBaseURL:    "https://openrouter.ai/api/v1/embeddings",
		OpenRouterEmbeddingModel:      llmbackend.DefaultRemoteEmbeddingModelName,
		OpenRouterImageModel:          "google/gemini-3.1-flash-lite-image",
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
		DeviceBrowserPath:             browserruntime.DeviceBrowserExecutablePath,
		DeviceBrowserProfilePath:      "",
		CompanionFileDirectory:        "/tmp/internkim-companion-files",
		APIURLPath:                    "/root/.internkim/env/api-url",
		FleetIDPath:                   "/root/.internkim/env/fleet-id",
		BlueclawWorkspacePath:         "/root/.blueclaw/workspace",
		FileReadPythonPath:            "/opt/internkim/document-venv/bin/python",
	}
}

func (service Service) Run(ctx context.Context) error {
	if service.HealthState == nil {
		service.HealthState = &platformHealthState{}
	}
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

	go service.startSlackSocketMode(ctx)
	go service.startSignalJSONRPCReceiver(ctx)

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
	multiplexer.HandleFunc("POST /v1/llm/structured", service.handleStructuredLLM)
	multiplexer.HandleFunc("POST /v1/llm/chat", service.handleChatLLM)
	multiplexer.HandleFunc("POST /v1/llm/text", service.handleTextLLM)
	multiplexer.HandleFunc("POST /v1/embedding/create", service.handleEmbeddingCreate)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/identity.resolve", service.handleIdentityResolve)
	multiplexer.HandleFunc("POST /v1/directory/person", service.handleDirectoryPerson)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/reply.send", service.handleReplySend)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/reaction.add", service.handleReactionAdd)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/reaction.remove", service.handleReactionRemove)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/history.fetch", service.handleHistoryFetch)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/progress.start", service.handleProgressStart)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/progress.stop", service.handleProgressStop)
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

func (service Service) handleStructuredLLM(responseWriter http.ResponseWriter, request *http.Request) {
	var structuredRequest StructuredLLMRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&structuredRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	response, errorValue := service.completeStructured(request.Context(), structuredRequest)
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleChatLLM(responseWriter http.ResponseWriter, request *http.Request) {
	var chatRequest ChatLLMRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&chatRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	response, errorValue := service.completeChat(request.Context(), chatRequest)
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleTextLLM(responseWriter http.ResponseWriter, request *http.Request) {
	var textRequest TextLLMRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&textRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	response, errorValue := service.completeText(request.Context(), textRequest)
	service.writeResponse(responseWriter, response, errorValue)
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

func (service Service) handleIdentityResolve(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "slack":
		response, errorValue = service.slackLookupUserFromRequest(request.Context(), request.Body)
	case "signal":
		response, errorValue = service.signalLookupUserFromRequest(request.Context(), request.Body)
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleReplySend(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "slack":
		response, errorValue = service.slackReplyFromRequest(request.Context(), request.Body)
	case "signal":
		response, errorValue = service.signalReplyFromRequest(request.Context(), request.Body)
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleReactionAdd(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "slack", "signal":
		response = map[string]string{"status": "noop"}
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleReactionRemove(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "slack", "signal":
		response = map[string]string{"status": "noop"}
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleHistoryFetch(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "slack":
		response, errorValue = service.slackHistoryFromRequest(request.Context(), request.Body)
	case "signal":
		response, errorValue = service.signalHistoryFromRequest(request.Context(), request.Body)
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleProgressStart(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "slack", "signal":
		response = map[string]string{"status": "noop"}
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleProgressStop(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "slack", "signal":
		response = map[string]string{"status": "noop"}
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
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
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(response)
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

// The options go in the message itself. They used to live in an attachment
// beside the buttons, and removing the buttons would have taken the choices
// with them.
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

func (service Service) slackLookupUserFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.slackLookupUser(ctx, payload)
}

func (service Service) slackLookupUser(ctx context.Context, payload json.RawMessage) (any, error) {
	var request userLookupRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	senderID := strings.TrimSpace(request.SenderID)
	if senderID == "" {
		return nil, errors.New("senderID is required")
	}
	var response struct {
		IsOK bool `json:"ok"`
		User struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			RealName string `json:"real_name"`
			Profile  struct {
				Email string `json:"email"`
			} `json:"profile"`
		} `json:"user"`
		Error string `json:"error"`
	}
	errorValue := service.slackRequest(ctx, http.MethodGet, "/users.info?user="+url.QueryEscape(senderID), nil, &response)
	if errorValue != nil {
		return nil, errorValue
	}
	if !response.IsOK {
		return nil, errors.New("slack user lookup failed: " + response.Error)
	}
	return map[string]string{
		"platform":    "slack",
		"senderID":    response.User.ID,
		"userID":      response.User.ID,
		"email":       response.User.Profile.Email,
		"displayName": firstNonEmpty(response.User.RealName, response.User.Name),
	}, nil
}

func (service Service) slackReplyFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.slackReply(ctx, payload)
}

func (service Service) slackReply(ctx context.Context, payload json.RawMessage) (any, error) {
	var request replyRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	handle, errorValue := decodePlatformHandle(request.ReplyTargetID)
	if errorValue != nil {
		return nil, errorValue
	}
	if handle.Platform != "slack" {
		return nil, errors.New("reply target platform mismatch")
	}
	if len(request.Attachments) > 0 {
		dispatchID, errorValue := service.postSlackReplyWithAttachments(ctx, handle, request)
		if errorValue != nil {
			return nil, errorValue
		}
		return newPlatformReplyResultWithAttachmentCount("slack", dispatchID, "public", request.Message, len(request.Attachments), nil), nil
	}
	body := map[string]string{
		"channel": handle.ChannelID,
		"text":    request.Message,
	}
	if strings.TrimSpace(handle.ThreadTimestamp) != "" {
		body["thread_ts"] = handle.ThreadTimestamp
	}
	var response struct {
		IsOK  bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	errorValue = service.slackRequest(ctx, http.MethodPost, "/chat.postMessage", body, &response)
	if errorValue != nil {
		return nil, errorValue
	}
	if !response.IsOK {
		return nil, errors.New("slack reply failed: " + response.Error)
	}
	return newPlatformReplyResult("slack", response.TS, "public", request.Message, nil), nil
}

func (service Service) slackHistoryFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	var request historyFetchRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	handle, errorValue := decodePlatformHandle(request.HistoryCursor)
	if errorValue != nil {
		return nil, errorValue
	}
	if handle.Platform != "slack" {
		return nil, errors.New("history cursor platform mismatch")
	}

	contextValue := service.slackContext(ctx, handle, request.Limit)
	return map[string]any{
		"messages":      contextValue.Messages,
		"hasMoreBefore": contextValue.HasMoreBefore,
		"historyCursor": contextValue.HistoryCursor,
	}, nil
}

func (service Service) slackRequest(ctx context.Context, method string, path string, body any, responseValue any) error {
	token := readSecretValue(service.Configuration.SlackTokenPath)
	if token == "" {
		return errors.New("slack bot token is not configured")
	}
	return service.authenticatedJSONRequest(ctx, method, strings.TrimRight("https://slack.com/api", "/")+path, token, body, responseValue)
}

type httpStatusError struct {
	StatusCode int
	Body       string
}

func (errorValue *httpStatusError) Error() string {
	return fmt.Sprintf("http status %d: %s", errorValue.StatusCode, errorValue.Body)
}

func isHTTPStatusNotFound(errorValue error) bool {
	var statusError *httpStatusError
	if !errors.As(errorValue, &statusError) {
		return false
	}
	return statusError.StatusCode == http.StatusNotFound
}

func isHTTPStatusForbidden(errorValue error) bool {
	var statusError *httpStatusError
	if !errors.As(errorValue, &statusError) {
		return false
	}
	return statusError.StatusCode == http.StatusForbidden
}

func (service Service) authenticatedJSONRequest(ctx context.Context, method string, requestURL string, token string, body any, responseValue any) error {
	var reader io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return errorValue
		}
		reader = bytes.NewReader(document)
	}
	request, errorValue := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	responseDocument, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &httpStatusError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(responseDocument))}
	}
	if responseValue == nil {
		return nil
	}
	return json.Unmarshal(responseDocument, responseValue)
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

func readOptionalFileValue(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
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

var fallbackPlatformEventLocker = newPlatformEventLocker()
var fallbackPlatformProgressManager = newPlatformProgressManager()

type platformEventLocker struct {
	mutex      sync.Mutex
	lockByName map[string]*sync.Mutex
}

func newPlatformEventLocker() *platformEventLocker {
	return &platformEventLocker{lockByName: map[string]*sync.Mutex{}}
}

func (locker *platformEventLocker) WithLock(name string, work func() error) error {
	lock := locker.lockForName(name)
	lock.Lock()
	defer lock.Unlock()
	return work()
}

func (locker *platformEventLocker) lockForName(name string) *sync.Mutex {
	locker.mutex.Lock()
	defer locker.mutex.Unlock()
	lock, isFound := locker.lockByName[name]
	if isFound {
		return lock
	}
	lock = &sync.Mutex{}
	locker.lockByName[name] = lock
	return lock
}

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

func (service Service) eventLocker() *platformEventLocker {
	if service.EventLocker != nil {
		return service.EventLocker
	}
	return fallbackPlatformEventLocker
}

func (service Service) progressManager() *platformProgressManager {
	if service.ProgressManager != nil {
		return service.ProgressManager
	}
	return fallbackPlatformProgressManager
}

const backupDeferredForwardRetryLimit = 8

var backupDeferredForwardRetryDelay = 30 * time.Second

func (service Service) forwardPlatformEvent(ctx context.Context, platform string, event platformInboundEvent) error {
	lockName := platform + ":" + event.ConversationID
	return service.eventLocker().WithLock(lockName, func() error {
		return service.forwardPlatformEventWithRetry(ctx, platform, event)
	})
}

func (service Service) forwardPlatformEventWithRetry(ctx context.Context, platform string, event platformInboundEvent) error {
	for attempt := 0; ; attempt++ {
		isDeferred, errorValue := service.forwardPlatformEventWithoutLock(ctx, platform, event)
		if errorValue != nil || !isDeferred {
			return errorValue
		}
		if attempt >= backupDeferredForwardRetryLimit {
			return errors.New("platform event stayed deferred through the whole backup retry window: " + event.MessageID)
		}
		log.Printf("platform event deferred by backup; retrying: platform=%s messageID=%s attempt=%d", platform, event.MessageID, attempt+1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backupDeferredForwardRetryDelay):
		}
	}
}

func (service Service) forwardPlatformEventWithoutLock(ctx context.Context, platform string, event platformInboundEvent) (bool, error) {
	document, errorValue := json.Marshal(event)
	if errorValue != nil {
		return false, errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/connectors/" + url.PathEscape(platform) + "/events"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(document))
	if errorValue != nil {
		return false, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return false, errorValue
	}
	defer response.Body.Close()
	responseDocument, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, errors.New(string(responseDocument))
	}
	log.Printf("platform event forward result: platform=%s messageID=%s response=%s", platform, event.MessageID, strings.TrimSpace(string(responseDocument)))
	var forwardResult struct {
		Ignored bool   `json:"ignored"`
		Reason  string `json:"reason"`
	}
	if json.Unmarshal(responseDocument, &forwardResult) == nil &&
		forwardResult.Ignored && forwardResult.Reason == "backup_prepare_active" {
		return true, nil
	}
	return false, nil
}

func (configuration Configuration) WithDefaults() Configuration {
	defaultConfiguration := DefaultConfiguration()
	if configuration.SocketPath == "" {
		configuration.SocketPath = defaultConfiguration.SocketPath
	}
	if configuration.OpenRouterKeyPath == "" {
		configuration.OpenRouterKeyPath = defaultConfiguration.OpenRouterKeyPath
	}
	if configuration.SlackTokenPath == "" {
		configuration.SlackTokenPath = defaultConfiguration.SlackTokenPath
	}
	if configuration.SlackAppTokenPath == "" {
		configuration.SlackAppTokenPath = defaultConfiguration.SlackAppTokenPath
	}
	if configuration.SignalJSONRPCURLPath == "" {
		configuration.SignalJSONRPCURLPath = defaultConfiguration.SignalJSONRPCURLPath
	}
	if configuration.SignalAccountPath == "" {
		configuration.SignalAccountPath = defaultConfiguration.SignalAccountPath
	}
	if configuration.SignalJSONRPCURL == "" {
		configuration.SignalJSONRPCURL = readOptionalFileValue(configuration.SignalJSONRPCURLPath)
	}
	if configuration.SignalAccount == "" {
		configuration.SignalAccount = readOptionalFileValue(configuration.SignalAccountPath)
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
	if configuration.DeviceBrowserPath == "" {
		configuration.DeviceBrowserPath = defaultConfiguration.DeviceBrowserPath
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
