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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/identity"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
	"gitlab.com/eastriver/internkim/internal/mattermostinteractive"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

type Configuration struct {
	SocketPath                     string
	VSockPort                      int
	OpenRouterKeyPath              string
	GoogleWorkspaceWebhookPath     string
	MattermostBaseURL              string
	MattermostTokenPath            string
	MattermostInteractiveTokenPath string
	SlackTokenPath                 string
	SlackAppTokenPath              string
	SignalJSONRPCURL               string
	SignalAccount                  string
	SignalJSONRPCURLPath           string
	SignalAccountPath              string
	BlueclawBaseURL                string
	AdmindBaseURL                  string
	OpenRouterBaseURL              string
	OpenRouterModel                string
	ForceOpenRouterModel           bool
	OpenRouterGatewaySecretPath    string
	OpenRouterGatewaySecretHeader  string
	OpenRouterWebBaseURL           string
	OpenRouterEmbeddingBaseURL     string
	OpenRouterEmbeddingModel       string
	OpenRouterImageModel           string
	EmbeddingProviderOrder         []string
	OllamaBaseURL                  string
	OllamaModel                    string
	LlamaCppBaseURL                string
	LlamaCppModel                  string
	LlamaCppEmbeddingBaseURL       string
	LlamaCppEmbeddingModel         string
	SocketGroupName                string
	LiteRTModelPath                string
	LocalLLMRunnerPath             string
	CompanionBaseURL               string
	PreferCompanionLLM             bool
	LocalInferenceMode             string
	PreferCompanionBrowser         bool
	LocalOnly                      bool
	LocalBackendOrder              []string
	ProviderAttemptTimeout         time.Duration
	AgentBrowserPath               string
	DeviceBrowserPath              string
	DeviceBrowserProfilePath       string
	CompanionFileDirectory         string
	FleetIDPath                    string
	BlueclawWorkspacePath          string
	FileReadPythonPath             string
}

type Service struct {
	Configuration     Configuration
	HTTPClient        *http.Client
	RunCommand        func(context.Context, string, []string, []byte) ([]byte, error)
	EventLocker       *platformEventLocker
	ProgressManager   *platformProgressManager
	HealthState       *platformHealthState
	MattermostLimiter *mattermostRateLimiter
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
	EphemeralUserID string                        `json:"ephemeralUserID,omitempty"`
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
	mutex                      sync.RWMutex
	MattermostTokenConfigured  bool      `json:"mattermostTokenConfigured"`
	MattermostBotUserResolved  bool      `json:"mattermostBotUserResolved"`
	MattermostForwarderRunning bool      `json:"mattermostForwarderRunning"`
	MattermostFallbackActive   bool      `json:"mattermostFallbackActive"`
	LastSuccessfulForwardAt    time.Time `json:"lastSuccessfulForwardAt,omitempty"`
	LastForwardError           string    `json:"lastForwardError,omitempty"`
}

var fallbackPlatformHealthState = &platformHealthState{}

type progressRequest struct {
	ReplyTargetID string `json:"replyTargetID"`
}

type mattermostPolledPost struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	ChannelID string   `json:"channel_id"`
	Message   string   `json:"message"`
	RootID    string   `json:"root_id"`
	Type      string   `json:"type"`
	CreateAt  int64    `json:"create_at"`
	FileIDs   []string `json:"file_ids"`
	Metadata  struct {
		Mentions []string `json:"mentions"`
	} `json:"metadata"`
}

type mattermostPostsResponse struct {
	Order []string                        `json:"order"`
	Posts map[string]mattermostPolledPost `json:"posts"`
}

type mattermostPollState struct {
	LastSeenByChannel map[string]int64
	IsInitialized     bool
	LastPollStartedAt int64
}

func DefaultConfiguration() Configuration {
	return Configuration{
		SocketPath:                     "/run/internkim/capability.sock",
		VSockPort:                      0,
		OpenRouterKeyPath:              "/root/.internkim/secrets/openrouter-api-key",
		GoogleWorkspaceWebhookPath:     "/root/.internkim/secrets/gas-webhook-url",
		MattermostBaseURL:              "http://localhost:8065",
		MattermostTokenPath:            "/root/.internkim/secrets/mattermost-bot-token",
		MattermostInteractiveTokenPath: "/root/.internkim/state/admin/mattermost-interactive-action-token",
		SlackTokenPath:                 "/root/.internkim/secrets/slack-bot-token",
		SlackAppTokenPath:              "/root/.internkim/secrets/slack-app-token",
		SignalJSONRPCURLPath:           "/root/.internkim/config/signal-jsonrpc-url",
		SignalAccountPath:              "/root/.internkim/config/signal-account",
		BlueclawBaseURL:                "http://127.0.0.1:8080",
		AdmindBaseURL:                  "http://127.0.0.1:18080",
		OpenRouterBaseURL:              "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterModel:                blueclaw.BlueclawDefaultModelName,
		OpenRouterGatewaySecretHeader:  "X-InternKim-Gateway-Secret",
		OpenRouterWebBaseURL:           "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterEmbeddingBaseURL:     "https://openrouter.ai/api/v1/embeddings",
		OpenRouterEmbeddingModel:       "openai/text-embedding-3-small",
		OpenRouterImageModel:           "google/gemini-3.1-flash-lite-image",
		EmbeddingProviderOrder:         llmbackend.DefaultLocalEmbeddingProviderOrder,
		OllamaBaseURL:                  "http://127.0.0.1:11434",
		OllamaModel:                    "gemma3:1b",
		LlamaCppBaseURL:                locallm.LlamaCppBaseURL,
		LlamaCppModel:                  "local/gemma-4-E4B-it-gguf",
		LlamaCppEmbeddingBaseURL:       locallm.LlamaCppEmbeddingBaseURL,
		LlamaCppEmbeddingModel:         llmbackend.DefaultEmbeddingGemmaModel,
		SocketGroupName:                "blueclaw",
		LiteRTModelPath:                locallm.ModelPath(),
		LocalLLMRunnerPath:             "/usr/local/bin/internkim-local-llm-runner",
		CompanionBaseURL:               "",
		PreferCompanionLLM:             false,
		LocalInferenceMode:             "",
		LocalOnly:                      false,
		ProviderAttemptTimeout:         5 * time.Minute,
		AgentBrowserPath:               "agent-browser",
		DeviceBrowserPath:              browserruntime.DeviceBrowserExecutablePath,
		DeviceBrowserProfilePath:       "",
		CompanionFileDirectory:         "/tmp/internkim-companion-files",
		FleetIDPath:                    "/root/.internkim/env/fleet-id",
		BlueclawWorkspacePath:          "/root/.blueclaw/workspace",
		FileReadPythonPath:             "/opt/blueclaw/builtin-skills-venv/bin/python",
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

	go service.startMattermostForwarder(ctx)
	go service.startSlackSocketMode(ctx)
	go service.startSignalJSONRPCReceiver(ctx)

	server := &http.Server{Handler: service.router()}
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
	multiplexer.HandleFunc("POST /v1/platform/{platform}/reply.send", service.handleReplySend)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/interaction.resolve", service.handleInteractionResolve)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/reaction.add", service.handleReactionAdd)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/history.fetch", service.handleHistoryFetch)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/attachments.import", service.handleAttachmentsImport)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/progress.start", service.handleProgressStart)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/progress.stop", service.handleProgressStop)
	multiplexer.HandleFunc("POST /v1/tools/{toolName}/invoke", service.handleToolInvoke)
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
	mattermost := service.mattermostHealth(ctx)
	status := "ok"
	if value, _ := mattermost["ok"].(bool); !value {
		status = "unhealthy"
	}
	return map[string]any{
		"status":     status,
		"mattermost": mattermost,
		"providers":  service.providerHealth(ctx),
		"checkedAt":  time.Now().UTC(),
	}
}

func (service Service) mattermostHealth(ctx context.Context) map[string]any {
	state := service.healthState().Snapshot()
	state.MattermostTokenConfigured = readSecretValue(service.Configuration.MattermostTokenPath) != ""
	if state.MattermostTokenConfigured {
		var botUser struct {
			ID string `json:"id"`
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue == nil && strings.TrimSpace(botUser.ID) != "" {
			state.MattermostBotUserResolved = true
		}
	}
	return map[string]any{
		"ok":                      state.MattermostTokenConfigured && state.MattermostBotUserResolved && state.MattermostForwarderRunning,
		"botTokenConfigured":      state.MattermostTokenConfigured,
		"botUserResolved":         state.MattermostBotUserResolved,
		"forwarderRunning":        state.MattermostForwarderRunning,
		"fallbackPollActive":      state.MattermostFallbackActive,
		"lastSuccessfulForwardAt": state.LastSuccessfulForwardAt,
		"lastForwardError":        state.LastForwardError,
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
	case "mattermost":
		response, errorValue = service.mattermostLookupUserFromRequest(request.Context(), request.Body)
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
	case "mattermost":
		response, errorValue = service.mattermostReplyFromRequest(request.Context(), request.Body)
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

func (service Service) handleInteractionResolve(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "mattermost":
		response, errorValue = service.mattermostInteractionResolve(request.Context(), request.Body)
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
	case "mattermost":
		response, errorValue = service.mattermostAddReactionFromRequest(request.Context(), request.Body)
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
	case "mattermost":
		response, errorValue = service.mattermostHistoryFromRequest(request.Context(), request.Body)
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

func (service Service) handleAttachmentsImport(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "mattermost":
		response, errorValue = service.mattermostImportAttachmentsFromRequest(request.Context(), request.Body)
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
	case "mattermost":
		response, errorValue = service.mattermostStartProgressFromRequest(request.Context(), request.Body)
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
	case "mattermost":
		response, errorValue = service.mattermostStopProgressFromRequest(request.Context(), request.Body)
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
		MattermostTokenConfigured:  state.MattermostTokenConfigured,
		MattermostBotUserResolved:  state.MattermostBotUserResolved,
		MattermostForwarderRunning: state.MattermostForwarderRunning,
		MattermostFallbackActive:   state.MattermostFallbackActive,
		LastSuccessfulForwardAt:    state.LastSuccessfulForwardAt,
		LastForwardError:           state.LastForwardError,
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

func (service Service) mattermostLookupUserFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.mattermostLookupUser(ctx, payload)
}

func (service Service) mattermostLookupUser(ctx context.Context, payload json.RawMessage) (any, error) {
	var request userLookupRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	senderID := strings.TrimSpace(request.SenderID)
	if senderID == "" {
		return nil, errors.New("senderID is required")
	}
	var response struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(senderID), nil, &response)
	name := firstNonEmpty(strings.TrimSpace(response.Nickname), strings.TrimSpace(response.FirstName), response.Username, response.ID)
	return map[string]string{
		"platform":    "mattermost",
		"senderID":    response.ID,
		"userID":      response.ID,
		"email":       response.Email,
		"handle":      response.Username,
		"name":        name,
		"callingName": identity.CallingName(name),
		"displayName": name,
	}, errorValue
}

func (service Service) mattermostReplyFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.mattermostReply(ctx, payload)
}

func (service Service) mattermostReply(ctx context.Context, payload json.RawMessage) (any, error) {
	var request replyRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	handle, errorValue := decodePlatformHandle(request.ReplyTargetID)
	if errorValue != nil {
		return nil, errorValue
	}
	if handle.Platform != "mattermost" {
		return nil, errors.New("reply target platform mismatch")
	}
	if strings.TrimSpace(request.RawEventID) == "" || strings.TrimSpace(request.OutboxID) == "" {
		return nil, errors.New("mattermost reply requires connector outbox metadata")
	}
	message, errorValue := service.mattermostReplyMessageWithRecovery(ctx, handle, request)
	if errorValue != nil {
		return nil, errorValue
	}
	message = service.mattermostAskMentionPrefix(ctx, handle, request) + message
	if request.Interaction == nil && strings.TrimSpace(request.EphemeralUserID) != "" {
		if len(request.Attachments) > 0 {
			return nil, errors.New("mattermost ephemeral reply cannot send native file attachments")
		}
		service.stopMattermostProgress(request.ReplyTargetID)
		return service.sendMattermostEphemeralText(ctx, handle, request, message)
	}
	service.stopMattermostProgress(request.ReplyTargetID)
	defer service.stopMattermostProgress(request.ReplyTargetID)
	fileIDs, errorValue := service.uploadMattermostAttachments(ctx, handle.ChannelID, request.Attachments)
	if errorValue != nil {
		return nil, errorValue
	}
	body := map[string]any{
		"channel_id": handle.ChannelID,
		"message":    strings.TrimSpace(message),
		"props":      service.mattermostReplyProperties(request, handle),
	}
	if strings.TrimSpace(handle.RootID) != "" {
		body["root_id"] = handle.RootID
	}
	if len(fileIDs) > 0 {
		body["file_ids"] = fileIDs
	}
	var response struct {
		ID string `json:"id"`
	}
	errorValue = service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", body, &response)
	if errorValue != nil && mattermostRootIDCanFallback(errorValue, handle) {
		delete(body, "root_id")
		errorValue = service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", body, &response)
	}
	if errorValue != nil {
		log.Printf("mattermost reply failed: %v", errorValue)
	}
	if errorValue == nil && request.Interaction != nil {
		if ephemeralError := service.sendMattermostAskEphemeralAttachment(ctx, handle, request); ephemeralError != nil {
			log.Printf("mattermost ask ephemeral attachment failed: %v", ephemeralError)
		}
	}
	return newPlatformReplyResult("mattermost", response.ID, "public", message, fileIDs), errorValue
}

func (service Service) mattermostInteractionResolve(ctx context.Context, reader io.Reader) (any, error) {
	var request interactionResolveRequest
	if errorValue := json.NewDecoder(reader).Decode(&request); errorValue != nil {
		return nil, errorValue
	}
	dispatchID := strings.TrimSpace(request.DispatchID)
	if dispatchID == "" {
		return nil, errors.New("dispatchID is required")
	}
	body := map[string]any{"props": mattermostinteractive.ClearAttachmentsUpdate()["props"]}
	path := "/api/v4/posts/" + url.PathEscape(dispatchID) + "/patch"
	return map[string]bool{"resolved": true}, service.mattermostRequest(ctx, http.MethodPut, path, body, nil)
}

func (service Service) mattermostAddReactionFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	var request reactionAddRequest
	if errorValue := json.NewDecoder(reader).Decode(&request); errorValue != nil {
		return nil, errorValue
	}
	messageID := strings.TrimSpace(request.MessageID)
	emojiName := strings.TrimSpace(request.EmojiName)
	if messageID == "" {
		return nil, errors.New("messageID is required")
	}
	if emojiName == "" {
		return nil, errors.New("emojiName is required")
	}
	var botUser struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		return nil, errorValue
	}
	body := map[string]any{
		"user_id":    botUser.ID,
		"post_id":    messageID,
		"emoji_name": emojiName,
	}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/reactions", body, nil)
	return map[string]string{"status": "ok"}, errorValue
}

func (service Service) mattermostReplyProperties(request replyRequest, handle platformHandle) map[string]any {
	properties := map[string]any{
		"internkim_raw_event_id": request.RawEventID,
		"internkim_outbox_id":    request.OutboxID,
	}
	if request.shouldSendMattermostAskAttachmentInline() {
		if attachment := service.mattermostAskAttachment(request, handle); attachment != nil {
			properties["attachments"] = []any{attachment}
		}
	}
	return properties
}

func (request replyRequest) shouldSendMattermostAskAttachmentInline() bool {
	return false
}

func (service Service) mattermostAskMentionPrefix(ctx context.Context, handle platformHandle, request replyRequest) string {
	if strings.EqualFold(strings.TrimSpace(handle.ChannelType), "D") {
		return ""
	}
	if service.mattermostAskAttachment(request, handle) == nil {
		return ""
	}
	targetUserID := request.mattermostAskTargetUserID()
	if targetUserID == "" {
		return ""
	}
	var response struct {
		Username string `json:"username"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(targetUserID), nil, &response); errorValue != nil {
		return ""
	}
	if strings.TrimSpace(response.Username) == "" {
		return ""
	}
	return "@" + response.Username + " "
}

func (service Service) sendMattermostAskEphemeralAttachment(ctx context.Context, handle platformHandle, request replyRequest) error {
	targetUserID := request.mattermostAskTargetUserID()
	if targetUserID == "" {
		return nil
	}
	attachment := service.mattermostAskAttachment(request, handle)
	if attachment == nil {
		return nil
	}
	post := map[string]any{
		"channel_id": handle.ChannelID,
		"props": map[string]any{
			"internkim_raw_event_id": request.RawEventID,
			"internkim_outbox_id":    request.OutboxID,
			"attachments":            []any{attachment},
		},
	}
	if strings.TrimSpace(handle.RootID) != "" {
		post["root_id"] = handle.RootID
	}
	body := map[string]any{"user_id": targetUserID, "post": post}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts/ephemeral", body, nil)
}

func (service Service) sendMattermostEphemeralText(ctx context.Context, handle platformHandle, request replyRequest, message string) (any, error) {
	post := map[string]any{
		"channel_id": handle.ChannelID,
		"message":    strings.TrimSpace(message),
		"props": map[string]any{
			"internkim_raw_event_id": request.RawEventID,
			"internkim_outbox_id":    request.OutboxID,
		},
	}
	if strings.TrimSpace(handle.RootID) != "" {
		post["root_id"] = handle.RootID
	}
	body := map[string]any{
		"user_id": strings.TrimSpace(request.EphemeralUserID),
		"post":    post,
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts/ephemeral", body, nil); errorValue != nil {
		return nil, errorValue
	}
	return newPlatformReplyResult("mattermost", "", "ephemeral", message, nil), nil
}

func (request replyRequest) mattermostAskTargetUserID() string {
	if request.Interaction == nil {
		return ""
	}
	return firstNonEmpty(
		strings.TrimSpace(request.EphemeralUserID),
		strings.TrimSpace(request.Interaction.TargetPlatformUserID),
	)
}

func (service Service) mattermostAskAttachment(request replyRequest, handle platformHandle) *mattermostinteractive.Attachment {
	if request.Interaction == nil {
		return nil
	}
	switch strings.TrimSpace(request.Interaction.Kind) {
	case "ask_confirm":
		return service.mattermostConfirmAttachment(request, handle)
	case "ask_choice_single":
		return service.mattermostChoiceAttachment(request, handle)
	case "ask_choice_multiple":
		return service.mattermostChoiceAttachment(request, handle)
	default:
		return nil
	}
}

func (service Service) mattermostConfirmAttachment(request replyRequest, handle platformHandle) *mattermostinteractive.Attachment {
	return &mattermostinteractive.Attachment{
		Fallback: strings.TrimSpace(request.Message),
		Actions: []mattermostinteractive.Action{
			service.mattermostAskButton("askConfirm", "확인", "primary", request, handle, "ask.confirm", "", ""),
			service.mattermostAskButton("askCancel", "취소", "danger", request, handle, "ask.cancel", "", ""),
		},
	}
}

func (service Service) mattermostChoiceAttachment(request replyRequest, handle platformHandle) *mattermostinteractive.Attachment {
	options := trimNonEmptyPlatformAskOptions(request.Interaction.Options)
	if len(options) <= 3 && request.Interaction.Kind == "ask_choice_single" {
		actions := []mattermostinteractive.Action{}
		for _, option := range options {
			actions = append(actions, service.mattermostAskButton("askChoice"+option.Key, mattermostChoiceDisplayLabel(option), "", request, handle, "ask.choice", option.Key, mattermostChoiceResolvedLabel(option)))
		}
		return &mattermostinteractive.Attachment{Fallback: strings.TrimSpace(request.Message), Text: mattermostChoiceAttachmentText(request.Interaction), Actions: actions}
	}
	return &mattermostinteractive.Attachment{
		Fallback: strings.TrimSpace(request.Message),
		Text:     mattermostChoiceAttachmentText(request.Interaction),
		Actions: []mattermostinteractive.Action{
			mattermostinteractive.Select(
				"askChoiceMenu",
				"선택",
				service.mattermostAskActionURL(),
				service.mattermostAskActionContext(request, handle, "ask.choice", "", ""),
				mattermostAskMenuOptions(options),
			),
		},
	}
}

func mattermostChoiceAttachmentText(interaction *platformAskInteraction) string {
	if interaction == nil {
		return ""
	}
	lines := []string{}
	for index, option := range interaction.Options {
		label := strings.TrimSpace(option.Label)
		if label == "" {
			continue
		}
		suffix := ""
		if strings.TrimSpace(option.Key) == strings.TrimSpace(interaction.RecommendedOptionKey) {
			suffix = " (추천)"
		}
		lines = append(lines, numberedChoiceLabel(index, label)+suffix)
	}
	return strings.Join(lines, "\n")
}

func numberedChoiceLabel(index int, label string) string {
	return strconv.Itoa(index+1) + ". " + strings.TrimSpace(label)
}

func mattermostChoiceDisplayLabel(option platformAskChoiceOption) string {
	shortLabel := strings.TrimSpace(option.ShortLabel)
	if shortLabel != "" {
		return shortLabel
	}
	return truncateMattermostChoiceLabel(strings.TrimSpace(option.Label), 8)
}

func mattermostChoiceResolvedLabel(option platformAskChoiceOption) string {
	return firstNonEmpty(strings.TrimSpace(option.ShortLabel), strings.TrimSpace(option.Label))
}

func truncateMattermostChoiceLabel(label string, maximumLength int) string {
	words := strings.Fields(label)
	if len(words) == 0 {
		return ""
	}
	selectedWords := []string{}
	for _, word := range words {
		candidateWords := append(append([]string{}, selectedWords...), word)
		candidate := strings.Join(candidateWords, " ")
		if len([]rune(candidate)) > maximumLength {
			break
		}
		selectedWords = append(selectedWords, word)
	}
	if len(selectedWords) > 0 {
		return strings.Join(selectedWords, " ")
	}
	runes := []rune(words[0])
	if len(runes) <= maximumLength {
		return words[0]
	}
	return string(runes[:maximumLength])
}

func (service Service) mattermostAskButton(id string, name string, style string, request replyRequest, handle platformHandle, action string, choiceKey string, choiceLabel string) mattermostinteractive.Action {
	context := service.mattermostAskActionContext(request, handle, action, choiceKey, choiceLabel)
	return mattermostinteractive.Button(id, name, "", style, service.mattermostAskActionURL(), context)
}

func (service Service) mattermostAskActionContext(request replyRequest, handle platformHandle, action string, choiceKey string, choiceLabel string) mattermostinteractive.Context {
	return mattermostinteractive.Context{
		Action:           action,
		InteractionID:    request.Interaction.InteractionID,
		TaskRunID:        request.Interaction.TaskRunID,
		ConversationID:   handle.ConversationID,
		ReplyTargetID:    request.ReplyTargetID,
		ChoiceKey:        choiceKey,
		ChoiceLabel:      choiceLabel,
		ResponseLanguage: request.Interaction.ResponseLanguage,
		TargetUserID:     request.mattermostAskTargetUserID(),
		Token:            service.ensureMattermostInteractiveActionToken(),
	}
}

func mattermostAskMenuOptions(options []platformAskChoiceOption) []mattermostinteractive.Option {
	menuOptions := []mattermostinteractive.Option{}
	for _, option := range options {
		menuOptions = append(menuOptions, mattermostinteractive.Option{Text: mattermostChoiceDisplayLabel(option), Value: mattermostChoiceMenuOptionValue(option)})
	}
	return menuOptions
}

func mattermostChoiceMenuOptionValue(option platformAskChoiceOption) string {
	document, errorValue := json.Marshal(map[string]string{
		"key":   strings.TrimSpace(option.Key),
		"label": mattermostChoiceResolvedLabel(option),
	})
	if errorValue != nil {
		return strings.TrimSpace(option.Key)
	}
	return string(document)
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

func (service Service) mattermostAskActionURL() string {
	return strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/_internkim/mattermost/actions"
}

func (service Service) ensureMattermostInteractiveActionToken() string {
	path := strings.TrimSpace(service.Configuration.MattermostInteractiveTokenPath)
	token := strings.TrimSpace(readOptionalFileValue(path))
	if token != "" {
		return token
	}
	token = randomCapabilityHex(32)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return token
	}
	if errorValue := os.WriteFile(path, []byte(token+"\n"), 0o600); errorValue != nil {
		return token
	}
	return token
}

func randomCapabilityHex(size int) string {
	value := make([]byte, size)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}

func (service Service) stopMattermostProgress(replyTargetID string) {
	service.progressManager().Stop("mattermost:" + replyTargetID)
}

func (service Service) mattermostStartProgressFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	var request progressRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	handle, errorValue := decodePlatformHandle(request.ReplyTargetID)
	if errorValue != nil {
		return nil, errorValue
	}
	if handle.Platform != "mattermost" {
		return nil, errors.New("progress target platform mismatch")
	}
	if errorValue := service.sendMattermostTyping(ctx, handle); errorValue != nil {
		log.Printf("mattermost initial typing failed: %v", errorValue)
	}
	service.progressManager().Start("mattermost:"+request.ReplyTargetID, mattermostProgressTTL, func(progressContext context.Context) {
		service.refreshMattermostTyping(progressContext, handle)
	})
	return map[string]string{"status": "started"}, nil
}

func (service Service) mattermostStopProgressFromRequest(_ context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	var request progressRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	if _, errorValue := decodePlatformHandle(request.ReplyTargetID); errorValue != nil {
		return nil, errorValue
	}
	service.progressManager().Stop("mattermost:" + request.ReplyTargetID)
	return map[string]string{"status": "stopped"}, nil
}

func (service Service) refreshMattermostTyping(ctx context.Context, handle platformHandle) {
	ticker := time.NewTicker(mattermostTypingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if errorValue := service.sendMattermostTyping(ctx, handle); errorValue != nil {
				log.Printf("mattermost typing failed: %v", errorValue)
			}
		}
	}
}

func (service Service) sendMattermostTyping(ctx context.Context, handle platformHandle) error {
	var botUser struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(botUser.ID) == "" || strings.TrimSpace(handle.ChannelID) == "" {
		return nil
	}

	if strings.TrimSpace(handle.RootID) != "" {
		if errorValue := service.publishMattermostTyping(ctx, botUser.ID, handle.ChannelID, handle.RootID); errorValue != nil && !mattermostRootIDCanFallback(errorValue, handle) {
			return errorValue
		}
	}
	if errorValue := service.publishMattermostTyping(ctx, botUser.ID, handle.ChannelID, ""); errorValue != nil {
		return errorValue
	}
	return nil
}

func mattermostRootIDCanFallback(errorValue error, handle platformHandle) bool {
	if errorValue == nil || strings.TrimSpace(handle.RootID) == "" {
		return false
	}
	message := strings.ToLower(errorValue.Error())
	return strings.Contains(message, "invalid rootid") || strings.Contains(message, "invalid root_id") || strings.Contains(message, "root_id")
}

func (service Service) publishMattermostTyping(ctx context.Context, botUserID string, channelID string, rootID string) error {
	body := map[string]string{"channel_id": channelID}
	if strings.TrimSpace(rootID) != "" {
		body["parent_id"] = rootID
	}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/users/"+url.PathEscape(botUserID)+"/typing", body, nil)
}

func (service Service) mattermostHistoryFromRequest(ctx context.Context, reader io.Reader) (any, error) {
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
	if handle.Platform != "mattermost" {
		return nil, errors.New("history cursor platform mismatch")
	}

	contextValue := service.mattermostContext(ctx, handle, request.Limit)
	return map[string]any{
		"messages":      contextValue.Messages,
		"hasMoreBefore": contextValue.HasMoreBefore,
		"historyCursor": contextValue.HistoryCursor,
	}, nil
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

func (service Service) mattermostRequest(ctx context.Context, method string, path string, body any, responseValue any) error {
	token := readSecretValue(service.Configuration.MattermostTokenPath)
	if token == "" {
		return errors.New("mattermost bot token is not configured")
	}
	if errorValue := service.mattermostLimiter().wait(ctx); errorValue != nil {
		return errorValue
	}
	return service.authenticatedJSONRequest(ctx, method, strings.TrimRight(service.Configuration.MattermostBaseURL, "/")+path, token, body, responseValue)
}

func (service Service) mattermostLimiter() *mattermostRateLimiter {
	if service.MattermostLimiter != nil {
		return service.MattermostLimiter
	}
	return defaultMattermostRateLimiter
}

func (service Service) slackRequest(ctx context.Context, method string, path string, body any, responseValue any) error {
	token := readSecretValue(service.Configuration.SlackTokenPath)
	if token == "" {
		return errors.New("slack bot token is not configured")
	}
	return service.authenticatedJSONRequest(ctx, method, strings.TrimRight("https://slack.com/api", "/")+path, token, body, responseValue)
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
		return fmt.Errorf("http status %d: %s", response.StatusCode, strings.TrimSpace(string(responseDocument)))
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

const mattermostProgressTTL = 3 * time.Minute
const mattermostTypingInterval = 4 * time.Second

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
		maximumLifetime = mattermostProgressTTL
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

func (service Service) startMattermostForwarder(ctx context.Context) {
	token := readSecretValue(service.Configuration.MattermostTokenPath)
	if token == "" {
		service.healthState().Update(func(state *platformHealthState) {
			state.MattermostTokenConfigured = false
			state.MattermostForwarderRunning = false
			state.LastForwardError = "mattermost bot token is not configured"
		})
		log.Print("mattermost forwarder disabled: token missing")
		return
	}
	service.healthState().Update(func(state *platformHealthState) {
		state.MattermostTokenConfigured = true
	})
	var botUser struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		service.healthState().Update(func(state *platformHealthState) {
			state.MattermostBotUserResolved = false
			state.MattermostForwarderRunning = false
			state.LastForwardError = errorValue.Error()
		})
		log.Printf("mattermost websocket forwarder disabled: bot lookup failed: %v", errorValue)
		return
	}
	service.healthState().Update(func(state *platformHealthState) {
		state.MattermostBotUserResolved = strings.TrimSpace(botUser.ID) != ""
		state.MattermostForwarderRunning = true
		state.LastForwardError = ""
	})
	log.Printf("mattermost forwarder enabled: botUserID=%s botUsername=%s baseURL=%s", botUser.ID, botUser.Username, service.Configuration.MattermostBaseURL)
	pollState := mattermostPollState{LastSeenByChannel: map[string]int64{}}
	var lastSeenMutex sync.Mutex
	listener := MattermostWebSocketForwarder{
		URL:         deriveMattermostWebSocketURL(service.Configuration.MattermostBaseURL),
		BotToken:    token,
		BotUserID:   botUser.ID,
		BotUsername: botUser.Username,
		BlueclawURL: strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/connectors/mattermost/events",
		HTTPClient:  service.httpClient(),
		EventBuilder: func(ctx context.Context, payload []byte) (platformInboundEvent, bool, error) {
			event, hasEvent, errorValue := normalizeMattermostWebSocketPayload(payload, botUser.ID, botUser.Username)
			if errorValue != nil || !hasEvent {
				return event, hasEvent, errorValue
			}
			event = service.enrichMattermostEvent(ctx, event)
			handled, errorValue := service.handleMattermostCompanionConnectCommand(ctx, event)
			if handled || errorValue != nil {
				return event, false, errorValue
			}
			return event, true, nil
		},
		AfterForward: func(ctx context.Context, payload []byte) {
			service.healthState().Update(func(state *platformHealthState) {
				state.LastSuccessfulForwardAt = time.Now().UTC()
				state.LastForwardError = ""
			})
			post, metadata, hasPost, errorValue := mattermostWebSocketPost(payload)
			if errorValue != nil || !hasPost || strings.TrimSpace(post.ChannelID) == "" || post.CreateAt <= 0 {
				return
			}
			lastSeenMutex.Lock()
			if post.CreateAt > pollState.LastSeenByChannel[post.ChannelID] {
				pollState.LastSeenByChannel[post.ChannelID] = post.CreateAt
			}
			lastSeenMutex.Unlock()
			go service.routeUploadedMattermostAttachments(context.Background(), post, metadata.ChannelType, metadata.ChannelName, botUser.ID)
		},
		AfterForwardError: func(errorValue error) {
			service.healthState().Update(func(state *platformHealthState) {
				state.LastForwardError = errorValue.Error()
			})
		},
		PollFallback: func(ctx context.Context) {
			service.pollMattermostFallback(ctx, &pollState, &lastSeenMutex)
		},
	}
	service.pollMattermostFallback(ctx, &pollState, &lastSeenMutex)
	go service.runMattermostPollingFallback(ctx, &pollState, &lastSeenMutex)
	go service.runMattermostImportCleanup(ctx)
	go listener.Start(ctx)
}

func (service Service) runMattermostPollingFallback(ctx context.Context, pollState *mattermostPollState, mutex *sync.Mutex) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.pollMattermostFallback(ctx, pollState, mutex)
		}
	}
}

func (service Service) pollMattermostFallback(ctx context.Context, pollState *mattermostPollState, mutex *sync.Mutex) {
	service.healthState().Update(func(state *platformHealthState) {
		state.MattermostFallbackActive = true
	})
	mutex.Lock()
	defer mutex.Unlock()
	wasInitialized := pollState.IsInitialized
	previousChannelCount := len(pollState.LastSeenByChannel)
	if errorValue := service.pollMattermost(ctx, pollState); errorValue != nil {
		service.healthState().Update(func(state *platformHealthState) {
			state.LastForwardError = errorValue.Error()
		})
		log.Printf("mattermost fallback poll failed: %v", errorValue)
		return
	}
	if !wasInitialized {
		log.Printf("mattermost fallback poll initialized: channels=%d", len(pollState.LastSeenByChannel))
		return
	}
	if len(pollState.LastSeenByChannel) > previousChannelCount {
		log.Printf("mattermost fallback poll discovered channels: previous=%d current=%d", previousChannelCount, len(pollState.LastSeenByChannel))
	}
}

func (service Service) pollMattermost(ctx context.Context, state *mattermostPollState) error {
	if state.LastSeenByChannel == nil {
		state.LastSeenByChannel = map[string]int64{}
	}
	previousPollStartedAt := state.LastPollStartedAt
	pollStartedAt := time.Now().UnixMilli()
	var botUser struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(botUser.ID) == "" {
		return errors.New("mattermost bot user id is empty")
	}

	channels, errorValue := service.mattermostBotChannels(ctx, botUser.ID)
	if errorValue != nil {
		return errorValue
	}

	for _, channel := range channels {
		if strings.TrimSpace(channel.ID) == "" {
			continue
		}
		if _, isFound := state.LastSeenByChannel[channel.ID]; !isFound {
			if state.IsInitialized {
				nextSeen, errorValue := service.forwardMattermostChannelPosts(ctx, botUser.ID, botUser.Username, channel.ID, channel.Type, channel.Name, mattermostNewChannelSince(previousPollStartedAt))
				if errorValue != nil {
					log.Printf("mattermost new channel poll failed: %s: %v", channel.ID, errorValue)
					continue
				}
				state.LastSeenByChannel[channel.ID] = nextSeen
				continue
			}
			state.LastSeenByChannel[channel.ID] = pollStartedAt
			continue
		}
		nextSeen, errorValue := service.forwardMattermostChannelPosts(ctx, botUser.ID, botUser.Username, channel.ID, channel.Type, channel.Name, state.LastSeenByChannel[channel.ID])
		if errorValue != nil {
			log.Printf("mattermost channel poll failed: %s: %v", channel.ID, errorValue)
			continue
		}
		if nextSeen > state.LastSeenByChannel[channel.ID] {
			state.LastSeenByChannel[channel.ID] = nextSeen
		}
	}
	state.IsInitialized = true
	state.LastPollStartedAt = pollStartedAt
	return nil
}

type mattermostBotChannel struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

func (service Service) mattermostBotChannels(ctx context.Context, botUserID string) ([]mattermostBotChannel, error) {
	const perPage = 200
	channels := []mattermostBotChannel{}
	for page := 0; page < 100; page++ {
		pageChannels := []mattermostBotChannel{}
		path := fmt.Sprintf("/api/v4/users/%s/channels?page=%d&per_page=%d", url.PathEscape(botUserID), page, perPage)
		if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, nil, &pageChannels); errorValue != nil {
			return nil, errorValue
		}
		channels = append(channels, pageChannels...)
		if len(pageChannels) < perPage {
			return channels, nil
		}
	}
	return channels, nil
}

func mattermostNewChannelSince(previousPollStartedAt int64) int64 {
	if previousPollStartedAt <= 1000 {
		return 0
	}
	return previousPollStartedAt - 1000
}

func (service Service) latestMattermostChannelPostCreateAt(ctx context.Context, channelID string) (int64, error) {
	var response mattermostPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?per_page=1"
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, nil, &response); errorValue != nil {
		return 0, errorValue
	}
	return latestMattermostPostCreateAt(response), nil
}

func latestMattermostPostCreateAt(response mattermostPostsResponse) int64 {
	latestCreateAt := int64(0)
	for _, post := range response.Posts {
		if post.CreateAt > latestCreateAt {
			latestCreateAt = post.CreateAt
		}
	}
	return latestCreateAt
}

func (service Service) forwardMattermostChannelPosts(ctx context.Context, botUserID string, botUsername string, channelID string, channelType string, channelName string, since int64) (int64, error) {
	var response mattermostPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?since=" + strconv.FormatInt(since, 10)
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, nil, &response); errorValue != nil {
		return since, errorValue
	}
	return service.forwardMattermostPosts(ctx, botUserID, botUsername, channelType, channelName, response, since)
}

func (service Service) forwardMattermostPosts(ctx context.Context, botUserID string, botUsername string, channelType string, channelName string, response mattermostPostsResponse, since int64) (int64, error) {
	nextSeen := since
	for index := len(response.Order) - 1; index >= 0; index-- {
		post := response.Posts[response.Order[index]]
		if strings.TrimSpace(post.ID) == "" || post.CreateAt <= since {
			continue
		}
		addressing := mattermostAddressingFromMessage(post.Message, botUsername)
		event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
			ID:        post.ID,
			UserID:    post.UserID,
			ChannelID: post.ChannelID,
			Message:   post.Message,
			RootID:    post.RootID,
			Type:      post.Type,
			CreateAt:  post.CreateAt,
			FileIDs:   post.FileIDs,
			Metadata:  post.Metadata,
		}, botUserID, channelType, channelName, addressing)
		if errorValue != nil {
			log.Printf("mattermost post normalize failed: %s: %v", post.ID, errorValue)
			if post.CreateAt > nextSeen {
				nextSeen = post.CreateAt
			}
			continue
		}
		if !hasEvent {
			if post.CreateAt > nextSeen {
				nextSeen = post.CreateAt
			}
			continue
		}
		event = service.enrichMattermostEvent(ctx, event)
		handled, errorValue := service.handleMattermostCompanionConnectCommand(ctx, event)
		if errorValue != nil {
			log.Printf("mattermost companion connect command failed: %s: %v", post.ID, errorValue)
			continue
		}
		if handled {
			if post.CreateAt > nextSeen {
				nextSeen = post.CreateAt
			}
			continue
		}
		if errorValue := service.forwardMattermostEvent(ctx, event); errorValue != nil {
			log.Printf("mattermost post forward failed: %s: %v", post.ID, errorValue)
			continue
		}
		log.Printf("mattermost post forwarded: postID=%s channelID=%s", post.ID, post.ChannelID)
		if post.CreateAt > nextSeen {
			nextSeen = post.CreateAt
		}
	}
	return nextSeen, nil
}

func (service Service) forwardMattermostEvent(ctx context.Context, event platformInboundEvent) error {
	errorValue := service.forwardPlatformEvent(ctx, "mattermost", event)
	service.healthState().Update(func(state *platformHealthState) {
		if errorValue != nil {
			state.LastForwardError = errorValue.Error()
			return
		}
		state.LastSuccessfulForwardAt = time.Now().UTC()
		state.LastForwardError = ""
	})
	return errorValue
}

func (service Service) forwardPlatformEvent(ctx context.Context, platform string, event platformInboundEvent) error {
	lockName := platform + ":" + event.ConversationID
	return service.eventLocker().WithLock(lockName, func() error {
		return service.forwardPlatformEventWithoutLock(ctx, platform, event)
	})
}

func (service Service) forwardPlatformEventWithoutLock(ctx context.Context, platform string, event platformInboundEvent) error {
	document, errorValue := json.Marshal(event)
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/connectors/" + url.PathEscape(platform) + "/events"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	responseDocument, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New(string(responseDocument))
	}
	log.Printf("platform event forward result: platform=%s messageID=%s response=%s", platform, event.MessageID, strings.TrimSpace(string(responseDocument)))
	return nil
}

func deriveMattermostWebSocketURL(baseURL string) string {
	parsedURL, errorValue := url.Parse(baseURL)
	if errorValue != nil || parsedURL.Host == "" {
		return ""
	}
	if parsedURL.Scheme == "https" {
		parsedURL.Scheme = "wss"
	} else {
		parsedURL.Scheme = "ws"
	}
	parsedURL.Path = "/api/v4/websocket"
	parsedURL.RawQuery = ""
	return parsedURL.String()
}

func (configuration Configuration) WithDefaults() Configuration {
	defaultConfiguration := DefaultConfiguration()
	if configuration.SocketPath == "" {
		configuration.SocketPath = defaultConfiguration.SocketPath
	}
	if configuration.OpenRouterKeyPath == "" {
		configuration.OpenRouterKeyPath = defaultConfiguration.OpenRouterKeyPath
	}
	if configuration.GoogleWorkspaceWebhookPath == "" {
		configuration.GoogleWorkspaceWebhookPath = defaultConfiguration.GoogleWorkspaceWebhookPath
	}
	if configuration.MattermostBaseURL == "" {
		configuration.MattermostBaseURL = defaultConfiguration.MattermostBaseURL
	}
	if configuration.MattermostTokenPath == "" {
		configuration.MattermostTokenPath = defaultConfiguration.MattermostTokenPath
	}
	if configuration.MattermostInteractiveTokenPath == "" {
		configuration.MattermostInteractiveTokenPath = defaultConfiguration.MattermostInteractiveTokenPath
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
	if configuration.ProviderAttemptTimeout <= 0 {
		configuration.ProviderAttemptTimeout = defaultConfiguration.ProviderAttemptTimeout
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
