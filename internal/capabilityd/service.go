package capabilityd

import (
	"bytes"
	"context"
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
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	browserruntime "github.com/anthropic-lab/internkim/internal/browser"
)

type Configuration struct {
	SocketPath                 string
	OpenRouterKeyPath          string
	MattermostBaseURL          string
	MattermostTokenPath        string
	SlackTokenPath             string
	SlackAppTokenPath          string
	SignalJSONRPCURL           string
	SignalAccount              string
	SignalJSONRPCURLPath       string
	SignalAccountPath          string
	BlueclawBaseURL            string
	OpenRouterBaseURL          string
	OpenRouterModel            string
	OpenRouterEmbeddingBaseURL string
	OpenRouterEmbeddingModel   string
	SocketGroupName            string
	LiteRTModelPath            string
	LiteRTWrapperPath          string
	CompanionBaseURL           string
	PreferCompanionLLM         bool
	LocalOnly                  bool
	ProviderAttemptTimeout     time.Duration
	AgentBrowserPath           string
	DeviceBrowserPath          string
	CompanionFileDirectory     string
}

type Service struct {
	Configuration   Configuration
	HTTPClient      *http.Client
	RunCommand      func(context.Context, string, []string, []byte) ([]byte, error)
	EventLocker     *platformEventLocker
	ProgressManager *platformProgressManager
}

type embeddingRequest struct {
	Input any    `json:"input"`
	Model string `json:"model"`
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
	ReplyTargetID string             `json:"replyTargetID"`
	Message       string             `json:"message"`
	Attachments   []platformFileSpec `json:"attachments,omitempty"`
}

type progressRequest struct {
	ReplyTargetID string `json:"replyTargetID"`
}

type mattermostPolledPost struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id"`
	Type      string `json:"type"`
	CreateAt  int64  `json:"create_at"`
}

type mattermostPostsResponse struct {
	Order []string                        `json:"order"`
	Posts map[string]mattermostPolledPost `json:"posts"`
}

func DefaultConfiguration() Configuration {
	return Configuration{
		SocketPath:                 "/run/internkim/capability.sock",
		OpenRouterKeyPath:          "/root/.internkim/secrets/openrouter-api-key",
		MattermostBaseURL:          "http://localhost:8065",
		MattermostTokenPath:        "/root/.internkim/secrets/mattermost-bot-token",
		SlackTokenPath:             "/root/.internkim/secrets/slack-bot-token",
		SlackAppTokenPath:          "/root/.internkim/secrets/slack-app-token",
		SignalJSONRPCURLPath:       "/root/.internkim/config/signal-jsonrpc-url",
		SignalAccountPath:          "/root/.internkim/config/signal-account",
		BlueclawBaseURL:            "http://127.0.0.1:8080",
		OpenRouterBaseURL:          "https://openrouter.ai/api/v1/chat/completions",
		OpenRouterModel:            "google/gemini-3.1-flash-lite-preview",
		OpenRouterEmbeddingBaseURL: "https://openrouter.ai/api/v1/embeddings",
		OpenRouterEmbeddingModel:   "text-embedding-3-small",
		SocketGroupName:            "blueclaw",
		LiteRTModelPath:            "/root/.internkim/models/gemma-4-E4B-it.litertlm",
		LiteRTWrapperPath:          "/usr/local/bin/internkim-litert-wrapper",
		CompanionBaseURL:           "",
		PreferCompanionLLM:         false,
		LocalOnly:                  false,
		ProviderAttemptTimeout:     90 * time.Second,
		AgentBrowserPath:           "agent-browser",
		DeviceBrowserPath:          browserruntime.DeviceBrowserExecutablePath,
		CompanionFileDirectory:     "/tmp/internkim-companion-files",
	}
}

func (service Service) Run(ctx context.Context) error {
	listener, errorValue := service.listen()
	if errorValue != nil {
		return errorValue
	}
	defer listener.Close()

	go service.startMattermostForwarder(ctx)
	go service.startSlackSocketMode(ctx)
	go service.startSignalJSONRPCReceiver(ctx)

	server := &http.Server{Handler: service.router()}
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	multiplexer.HandleFunc("POST /v1/llm/structured", service.handleStructuredLLM)
	multiplexer.HandleFunc("POST /v1/llm/text", service.handleTextLLM)
	multiplexer.HandleFunc("POST /v1/embedding/create", service.handleEmbeddingCreate)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/identity.resolve", service.handleIdentityResolve)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/reply.send", service.handleReplySend)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/history.fetch", service.handleHistoryFetch)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/progress.start", service.handleProgressStart)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/progress.stop", service.handleProgressStop)
	multiplexer.HandleFunc("POST /v1/tools/{toolName}/invoke", service.handleToolInvoke)
	multiplexer.HandleFunc("GET /v1/capabilities", service.handleCapabilities)
	multiplexer.HandleFunc("GET /health", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok\n"))
	})
	return multiplexer
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
	var embeddingRequest embeddingRequest
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

func (service Service) createEmbedding(ctx context.Context, request embeddingRequest) (any, error) {
	apiKey := readSecretValue(service.Configuration.OpenRouterKeyPath)
	if apiKey == "" {
		return nil, errors.New("openrouter api key is not configured")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return nil, errors.New("openrouter api key is a simulation placeholder; set OPENROUTER_API_KEY or rerun setup --only openrouter --force")
	}

	requestDocument, errorValue := json.Marshal(map[string]any{
		"model": firstNonEmpty(request.Model, service.Configuration.OpenRouterEmbeddingModel, DefaultConfiguration().OpenRouterEmbeddingModel),
		"input": request.Input,
	})
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.Configuration.OpenRouterEmbeddingBaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, _ := io.ReadAll(httpResponse.Body)
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(string(responseDocument))
	}

	var parsedResponse struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Model string `json:"model"`
	}
	if errorValue := json.Unmarshal(responseDocument, &parsedResponse); errorValue != nil {
		return nil, errorValue
	}
	if len(parsedResponse.Data) == 0 {
		return nil, errors.New("openrouter embedding response did not include data")
	}
	if _, isBatch := request.Input.([]any); isBatch {
		embeddings := [][]float64{}
		for _, item := range parsedResponse.Data {
			embeddings = append(embeddings, item.Embedding)
		}
		return map[string]any{"provider": "openrouter", "model": parsedResponse.Model, "embeddings": embeddings}, nil
	}
	return map[string]any{"provider": "openrouter", "model": parsedResponse.Model, "embedding": parsedResponse.Data[0].Embedding}, nil
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
	return map[string]string{
		"platform":    "mattermost",
		"senderID":    response.ID,
		"userID":      response.ID,
		"email":       response.Email,
		"displayName": firstNonEmpty(strings.TrimSpace(response.FirstName+" "+response.LastName), response.Nickname, response.Username),
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
	service.stopMattermostProgress(request.ReplyTargetID)
	defer service.stopMattermostProgress(request.ReplyTargetID)
	fileIDs, errorValue := service.uploadMattermostAttachments(ctx, handle.ChannelID, request.Attachments)
	if errorValue != nil {
		return nil, errorValue
	}
	body := map[string]any{
		"channel_id": handle.ChannelID,
		"message":    request.Message,
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
	if errorValue != nil {
		log.Printf("mattermost reply failed: %v", errorValue)
	}
	return map[string]string{"dispatchID": response.ID}, errorValue
}

func (service Service) stopMattermostProgress(replyTargetID string) {
	service.progressManager().Stop("mattermost:" + replyTargetID)
}

func (service Service) mattermostStartProgressFromRequest(_ context.Context, reader io.Reader) (any, error) {
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
	service.progressManager().Start("mattermost:"+request.ReplyTargetID, mattermostProgressTTL, func(progressContext context.Context) {
		service.runMattermostTyping(progressContext, handle)
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

func (service Service) runMattermostTyping(ctx context.Context, handle platformHandle) {
	debounceTimer := time.NewTimer(mattermostTypingDebounce)
	defer debounceTimer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-debounceTimer.C:
		service.sendMattermostTyping(ctx, handle)
	}
	ticker := time.NewTicker(mattermostTypingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.sendMattermostTyping(ctx, handle)
		}
	}
}

func (service Service) sendMattermostTyping(ctx context.Context, handle platformHandle) {
	var botUser struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		log.Printf("mattermost typing bot lookup failed: %v", errorValue)
		return
	}
	if strings.TrimSpace(botUser.ID) == "" || strings.TrimSpace(handle.ChannelID) == "" {
		return
	}

	body := map[string]string{"channel_id": handle.ChannelID}
	if strings.TrimSpace(handle.RootID) != "" {
		body["parent_id"] = handle.RootID
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/users/"+url.PathEscape(botUser.ID)+"/typing", body, nil); errorValue != nil {
		log.Printf("mattermost typing failed: %v", errorValue)
	}
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
		return map[string]string{"dispatchID": dispatchID}, nil
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
	return map[string]string{"dispatchID": response.TS}, nil
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
	return service.authenticatedJSONRequest(ctx, method, strings.TrimRight(service.Configuration.MattermostBaseURL, "/")+path, token, body, responseValue)
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
		return errors.New(string(responseDocument))
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

func (service Service) httpClient() *http.Client {
	if service.HTTPClient != nil {
		return service.HTTPClient
	}
	return &http.Client{Timeout: 120 * time.Second}
}

func (service Service) runCommand(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
	if service.RunCommand != nil {
		return service.RunCommand(ctx, executablePath, arguments, standardInput)
	}

	commandContext, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	command := exec.CommandContext(commandContext, executablePath, arguments...)
	command.Stdin = bytes.NewReader(standardInput)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", executablePath, errorValue, strings.TrimSpace(string(output)))
	}
	return output, nil
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
const mattermostTypingDebounce = 750 * time.Millisecond
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
		log.Print("mattermost forwarder disabled: token missing")
		return
	}
	var botUser struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		log.Printf("mattermost websocket forwarder disabled: bot lookup failed: %v", errorValue)
		return
	}
	lastSeenByChannel := map[string]int64{}
	var lastSeenMutex sync.Mutex
	listener := MattermostWebSocketForwarder{
		URL:         deriveMattermostWebSocketURL(service.Configuration.MattermostBaseURL),
		BotToken:    token,
		BotUserID:   botUser.ID,
		BlueclawURL: strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/connectors/mattermost/events",
		HTTPClient:  service.httpClient(),
		EventBuilder: func(ctx context.Context, payload []byte) (platformInboundEvent, bool, error) {
			event, hasEvent, errorValue := normalizeMattermostWebSocketPayload(payload, botUser.ID)
			if errorValue != nil || !hasEvent {
				return event, hasEvent, errorValue
			}
			return service.enrichMattermostEvent(ctx, event), true, nil
		},
		AfterForward: func(_ context.Context, payload []byte) {
			post, _, hasPost, errorValue := mattermostWebSocketPost(payload)
			if errorValue != nil || !hasPost || strings.TrimSpace(post.ChannelID) == "" || post.CreateAt <= 0 {
				return
			}
			lastSeenMutex.Lock()
			if post.CreateAt > lastSeenByChannel[post.ChannelID] {
				lastSeenByChannel[post.ChannelID] = post.CreateAt
			}
			lastSeenMutex.Unlock()
		},
		PollFallback: func(ctx context.Context) {
			lastSeenMutex.Lock()
			defer lastSeenMutex.Unlock()
			if errorValue := service.pollMattermost(ctx, lastSeenByChannel); errorValue != nil {
				log.Printf("mattermost fallback poll failed: %v", errorValue)
			}
		},
	}
	go listener.Start(ctx)
}

func (service Service) pollMattermost(ctx context.Context, lastSeenByChannel map[string]int64) error {
	var botUser struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(botUser.ID) == "" {
		return errors.New("mattermost bot user id is empty")
	}

	var channels []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(botUser.ID)+"/channels", nil, &channels); errorValue != nil {
		return errorValue
	}

	for _, channel := range channels {
		if strings.TrimSpace(channel.ID) == "" {
			continue
		}
		if _, isFound := lastSeenByChannel[channel.ID]; !isFound {
			latestCreateAt, errorValue := service.latestMattermostChannelPostCreateAt(ctx, channel.ID)
			if errorValue != nil {
				log.Printf("mattermost channel watermark failed: %s: %v", channel.ID, errorValue)
				lastSeenByChannel[channel.ID] = time.Now().UnixMilli()
				continue
			}
			lastSeenByChannel[channel.ID] = latestCreateAt
			continue
		}
		nextSeen, errorValue := service.forwardMattermostChannelPosts(ctx, botUser.ID, channel.ID, channel.Type, lastSeenByChannel[channel.ID])
		if errorValue != nil {
			log.Printf("mattermost channel poll failed: %s: %v", channel.ID, errorValue)
			continue
		}
		if nextSeen > lastSeenByChannel[channel.ID] {
			lastSeenByChannel[channel.ID] = nextSeen
		}
	}
	return nil
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

func (service Service) forwardMattermostChannelPosts(ctx context.Context, botUserID string, channelID string, channelType string, since int64) (int64, error) {
	var response mattermostPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?since=" + strconv.FormatInt(since, 10)
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, nil, &response); errorValue != nil {
		return since, errorValue
	}

	nextSeen := since
	for index := len(response.Order) - 1; index >= 0; index-- {
		post := response.Posts[response.Order[index]]
		if strings.TrimSpace(post.ID) == "" || post.CreateAt <= since {
			continue
		}
		if post.CreateAt > nextSeen {
			nextSeen = post.CreateAt
		}
		event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
			ID:        post.ID,
			UserID:    post.UserID,
			ChannelID: post.ChannelID,
			Message:   post.Message,
			RootID:    post.RootID,
			Type:      post.Type,
			CreateAt:  post.CreateAt,
		}, botUserID, channelType)
		if errorValue != nil {
			log.Printf("mattermost post normalize failed: %s: %v", post.ID, errorValue)
			continue
		}
		if !hasEvent {
			continue
		}
		event = service.enrichMattermostEvent(ctx, event)
		if errorValue := service.forwardMattermostEvent(ctx, event); errorValue != nil {
			log.Printf("mattermost post forward failed: %s: %v", post.ID, errorValue)
		}
	}
	return nextSeen, nil
}

func (service Service) forwardMattermostEvent(ctx context.Context, event platformInboundEvent) error {
	return service.forwardPlatformEvent(ctx, "mattermost", event)
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
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseDocument, _ := io.ReadAll(response.Body)
		return errors.New(string(responseDocument))
	}
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
	if configuration.MattermostBaseURL == "" {
		configuration.MattermostBaseURL = defaultConfiguration.MattermostBaseURL
	}
	if configuration.MattermostTokenPath == "" {
		configuration.MattermostTokenPath = defaultConfiguration.MattermostTokenPath
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
	if configuration.OpenRouterBaseURL == "" {
		configuration.OpenRouterBaseURL = defaultConfiguration.OpenRouterBaseURL
	}
	if configuration.OpenRouterEmbeddingBaseURL == "" {
		configuration.OpenRouterEmbeddingBaseURL = defaultConfiguration.OpenRouterEmbeddingBaseURL
	}
	if configuration.OpenRouterEmbeddingModel == "" {
		configuration.OpenRouterEmbeddingModel = defaultConfiguration.OpenRouterEmbeddingModel
	}
	if configuration.SocketGroupName == "" {
		configuration.SocketGroupName = defaultConfiguration.SocketGroupName
	}
	if configuration.LiteRTModelPath == "" {
		configuration.LiteRTModelPath = defaultConfiguration.LiteRTModelPath
	}
	if configuration.LiteRTWrapperPath == "" {
		configuration.LiteRTWrapperPath = defaultConfiguration.LiteRTWrapperPath
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
	return configuration
}

func Run(configuration Configuration) error {
	ctx := context.Background()
	service := Service{Configuration: configuration.WithDefaults()}
	if errorValue := service.Run(ctx); errorValue != nil {
		return fmt.Errorf("capabilityd failed: %w", errorValue)
	}
	return nil
}
