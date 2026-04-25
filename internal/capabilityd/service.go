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
	"os/user"
	"strconv"
	"strings"
	"time"
)

type Configuration struct {
	SocketPath          string
	OpenRouterKeyPath   string
	MattermostBaseURL   string
	MattermostTokenPath string
	SlackTokenPath      string
	BlueclawBaseURL     string
	OpenRouterBaseURL   string
	SocketGroupName     string
}

type Service struct {
	Configuration Configuration
	HTTPClient    *http.Client
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmRequest struct {
	Model                  string        `json:"model"`
	ExecutionMode          string        `json:"executionMode"`
	Messages               []message     `json:"messages"`
	StructuredOutputSchema schemaRequest `json:"structuredOutputSchema"`
	RequireParameters      bool          `json:"requireParameters"`
	EnableResponseHealing  bool          `json:"enableResponseHealing"`
}

type schemaRequest struct {
	Name               string `json:"name"`
	Document           string `json:"document"`
	IsStrictlyEnforced bool   `json:"isStrictlyEnforced"`
}

type userLookupRequest struct {
	ExternalUserID string `json:"externalUserID"`
}

type conversationRequest struct {
	ConversationID string `json:"conversationID"`
}

type replyRequest struct {
	ConversationID string `json:"conversationID"`
	ParentID       string `json:"parentID"`
	Message        string `json:"message"`
}

type typingRequest struct {
	BotUserID      string `json:"botUserID"`
	ConversationID string `json:"conversationID"`
	ParentID       string `json:"parentID"`
}

func DefaultConfiguration() Configuration {
	return Configuration{
		SocketPath:          "/run/internkim/capability.sock",
		OpenRouterKeyPath:   "/root/.internkim/secrets/openrouter-api-key",
		MattermostBaseURL:   "http://localhost:8065",
		MattermostTokenPath: "/root/.internkim/secrets/mattermost-bot-token",
		SlackTokenPath:      "/root/.internkim/secrets/slack-bot-token",
		BlueclawBaseURL:     "http://127.0.0.1:8080",
		OpenRouterBaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		SocketGroupName:     "blueclaw",
	}
}

func (service Service) Run(ctx context.Context) error {
	listener, errorValue := service.listen()
	if errorValue != nil {
		return errorValue
	}
	defer listener.Close()

	go service.startMattermostForwarder(ctx)

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
	multiplexer.HandleFunc("POST /v1/platform/{platform}/identity.resolve", service.handleIdentityResolve)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/bot.resolve", service.handleBotResolve)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/conversation.kind", service.handleConversationKind)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/typing.publish", service.handleTypingPublish)
	multiplexer.HandleFunc("POST /v1/platform/{platform}/reply.send", service.handleReplySend)
	multiplexer.HandleFunc("GET /health", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = request
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok\n"))
	})
	return multiplexer
}

func (service Service) handleStructuredLLM(responseWriter http.ResponseWriter, request *http.Request) {
	var llmRequest llmRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&llmRequest); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	response, errorValue := service.completeStructured(request.Context(), llmRequest)
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
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleBotResolve(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "mattermost":
		response, errorValue = service.mattermostBotUser(request.Context())
	case "slack":
		response, errorValue = service.slackBotUser(request.Context())
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleConversationKind(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "mattermost":
		response, errorValue = service.mattermostChannelTypeFromRequest(request.Context(), request.Body)
	default:
		http.Error(responseWriter, "platform is not supported", http.StatusNotFound)
		return
	}
	service.writeResponse(responseWriter, response, errorValue)
}

func (service Service) handleTypingPublish(responseWriter http.ResponseWriter, request *http.Request) {
	var response any
	var errorValue error
	switch request.PathValue("platform") {
	case "mattermost":
		response, errorValue = service.mattermostTypingFromRequest(request.Context(), request.Body)
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

func (service Service) completeStructured(ctx context.Context, request llmRequest) (any, error) {
	executionMode := firstNonEmpty(request.ExecutionMode, "auto")
	switch executionMode {
	case "local":
		return nil, errors.New("local litert capability is not installed")
	case "remote", "auto":
		return service.completeRemote(ctx, request)
	default:
		return nil, errors.New("llm execution mode is not supported")
	}
}

func (service Service) completeRemote(ctx context.Context, request llmRequest) (any, error) {
	apiKey := readSecretValue(service.Configuration.OpenRouterKeyPath)
	if apiKey == "" {
		return nil, errors.New("openrouter api key is not configured")
	}

	requestDocument, errorValue := buildOpenRouterRequest(request)
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.Configuration.OpenRouterBaseURL, bytes.NewReader(requestDocument))
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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if errorValue := json.Unmarshal(responseDocument, &parsedResponse); errorValue != nil {
		return nil, errorValue
	}
	if len(parsedResponse.Choices) == 0 {
		return nil, errors.New("openrouter response did not include choices")
	}
	return map[string]string{
		"provider":        "openrouter",
		"model":           request.model(),
		"content":         parsedResponse.Choices[0].Message.Content,
		"selectedBackend": "remote",
	}, nil
}

func (service Service) mattermostBotUser(ctx context.Context) (any, error) {
	var response struct {
		ID string `json:"id"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &response)
	return map[string]string{"userID": response.ID}, errorValue
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
	var response struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(request.ExternalUserID), nil, &response)
	return map[string]string{
		"platform":       "mattermost",
		"externalUserID": response.ID,
		"userID":         response.ID,
		"email":          response.Email,
		"displayName":    firstNonEmpty(strings.TrimSpace(response.FirstName+" "+response.LastName), response.Nickname, response.Username),
	}, errorValue
}

func (service Service) mattermostChannelTypeFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.mattermostChannelType(ctx, payload)
}

func (service Service) mattermostChannelType(ctx context.Context, payload json.RawMessage) (any, error) {
	var request conversationRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	var response struct {
		Type string `json:"type"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/channels/"+url.PathEscape(request.ConversationID), nil, &response)
	return map[string]string{"channelType": response.Type, "conversationKind": response.Type}, errorValue
}

func (service Service) mattermostTypingFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.mattermostTyping(ctx, payload)
}

func (service Service) mattermostTyping(ctx context.Context, payload json.RawMessage) (any, error) {
	var request typingRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	body := map[string]string{"channel_id": request.ConversationID}
	if strings.TrimSpace(request.ParentID) != "" {
		body["parent_id"] = request.ParentID
	}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/users/"+url.PathEscape(request.BotUserID)+"/typing", body, nil)
	return map[string]string{"ok": "true"}, errorValue
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
	body := map[string]string{
		"channel_id": request.ConversationID,
		"message":    request.Message,
	}
	if strings.TrimSpace(request.ParentID) != "" {
		body["root_id"] = request.ParentID
	}
	var response struct {
		ID string `json:"id"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", body, &response)
	return map[string]string{"dispatchID": response.ID}, errorValue
}

func (service Service) slackBotUser(ctx context.Context) (any, error) {
	var response struct {
		IsOK   bool   `json:"ok"`
		UserID string `json:"user_id"`
		Error  string `json:"error"`
	}
	errorValue := service.slackRequest(ctx, http.MethodPost, "/auth.test", nil, &response)
	if errorValue != nil {
		return nil, errorValue
	}
	if !response.IsOK {
		return nil, errors.New("slack auth failed: " + response.Error)
	}
	return map[string]string{"userID": response.UserID}, nil
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
	errorValue := service.slackRequest(ctx, http.MethodGet, "/users.info?user="+url.QueryEscape(request.ExternalUserID), nil, &response)
	if errorValue != nil {
		return nil, errorValue
	}
	if !response.IsOK {
		return nil, errors.New("slack user lookup failed: " + response.Error)
	}
	return map[string]string{
		"platform":       "slack",
		"externalUserID": response.User.ID,
		"userID":         response.User.ID,
		"email":          response.User.Profile.Email,
		"displayName":    firstNonEmpty(response.User.RealName, response.User.Name),
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
	body := map[string]string{
		"channel": request.ConversationID,
		"text":    request.Message,
	}
	if strings.TrimSpace(request.ParentID) != "" {
		body["thread_ts"] = request.ParentID
	}
	var response struct {
		IsOK  bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	errorValue := service.slackRequest(ctx, http.MethodPost, "/chat.postMessage", body, &response)
	if errorValue != nil {
		return nil, errorValue
	}
	if !response.IsOK {
		return nil, errors.New("slack reply failed: " + response.Error)
	}
	return map[string]string{"dispatchID": response.TS}, nil
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

func buildOpenRouterRequest(request llmRequest) ([]byte, error) {
	var schema json.RawMessage
	if strings.TrimSpace(request.StructuredOutputSchema.Document) != "" {
		schema = json.RawMessage(request.StructuredOutputSchema.Document)
	}
	document := map[string]any{
		"model":    request.model(),
		"messages": request.Messages,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   request.StructuredOutputSchema.Name,
				"strict": request.StructuredOutputSchema.IsStrictlyEnforced,
				"schema": schema,
			},
		},
		"stream": false,
	}
	if request.RequireParameters {
		document["provider"] = map[string]bool{"require_parameters": true}
	}
	if request.EnableResponseHealing {
		document["plugins"] = []map[string]string{{"id": "response-healing"}}
	}
	return json.Marshal(document)
}

func readSecretValue(path string) string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	value := strings.TrimSpace(string(document))
	return strings.TrimPrefix(value, "OPENROUTER_API_KEY=")
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

func (request llmRequest) model() string {
	return strings.TrimSpace(request.Model)
}

func (service Service) startMattermostForwarder(ctx context.Context) {
	token := readSecretValue(service.Configuration.MattermostTokenPath)
	if token == "" {
		log.Print("mattermost forwarder disabled: token missing")
		return
	}
	listener := MattermostWebSocketForwarder{
		URL:         deriveMattermostWebSocketURL(service.Configuration.MattermostBaseURL),
		BotToken:    token,
		BlueclawURL: strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/connectors/mattermost/events",
		HTTPClient:  service.httpClient(),
	}
	go listener.Start(ctx)
	go service.startMattermostPoller(ctx)
}

func (service Service) startMattermostPoller(ctx context.Context) {
	lastSeenByChannel := map[string]int64{}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for ctx.Err() == nil {
		if errorValue := service.pollMattermost(ctx, lastSeenByChannel); errorValue != nil {
			log.Printf("mattermost poll failed: %v", errorValue)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
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
			lastSeenByChannel[channel.ID] = time.Now().Add(-60 * time.Second).UnixMilli()
		}
		nextSeen, errorValue := service.forwardMattermostChannelPosts(ctx, channel.ID, channel.Type, lastSeenByChannel[channel.ID])
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

func (service Service) forwardMattermostChannelPosts(ctx context.Context, channelID string, channelType string, since int64) (int64, error) {
	var response struct {
		Order []string `json:"order"`
		Posts map[string]struct {
			ID        string `json:"id"`
			UserID    string `json:"user_id"`
			ChannelID string `json:"channel_id"`
			Message   string `json:"message"`
			RootID    string `json:"root_id"`
			Type      string `json:"type"`
			CreateAt  int64  `json:"create_at"`
		} `json:"posts"`
	}
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
		event := map[string]string{
			"event":        "posted",
			"user_id":      post.UserID,
			"channel_id":   post.ChannelID,
			"channel_type": channelType,
			"post_id":      post.ID,
			"message":      post.Message,
			"root_id":      post.RootID,
			"type":         post.Type,
		}
		if errorValue := service.forwardMattermostEvent(ctx, event); errorValue != nil {
			log.Printf("mattermost post forward failed: %s: %v", post.ID, errorValue)
		}
	}
	return nextSeen, nil
}

func (service Service) forwardMattermostEvent(ctx context.Context, event map[string]string) error {
	document, errorValue := json.Marshal(event)
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/connectors/mattermost/events"
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
	if configuration.BlueclawBaseURL == "" {
		configuration.BlueclawBaseURL = defaultConfiguration.BlueclawBaseURL
	}
	if configuration.OpenRouterBaseURL == "" {
		configuration.OpenRouterBaseURL = defaultConfiguration.OpenRouterBaseURL
	}
	if configuration.SocketGroupName == "" {
		configuration.SocketGroupName = defaultConfiguration.SocketGroupName
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
