package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/mattermostinteractive"
)

const legacyMattermostInteractiveActionTokenFilename = "mattermost-interactive-action-token"

type mattermostInteractivePayload = mattermostinteractive.Payload
type mattermostInteractiveContext = mattermostinteractive.Context
type mattermostInteractiveResponse = mattermostinteractive.Response
type mattermostInteractiveError = mattermostinteractive.Error

type mattermostInteractiveActionHandler func(http.ResponseWriter, *http.Request, mattermostInteractivePayload)

type mattermostAttachment = mattermostinteractive.Attachment
type mattermostAction = mattermostinteractive.Action

type normalizedConnectorEventEnvelope struct {
	Event normalizedConnectorEvent `json:"event"`
}

type normalizedConnectorEvent struct {
	ConversationID   string                 `json:"conversationID"`
	MessageID        string                 `json:"messageID"`
	SenderID         string                 `json:"senderID"`
	ReplyTargetID    string                 `json:"replyTargetID"`
	Prompt           string                 `json:"prompt"`
	ResponseLanguage string                 `json:"responseLanguage,omitempty"`
	Context          normalizedEventContext `json:"context"`
	LegacyFields     map[string]any         `json:"legacyFields,omitempty"`
}

type normalizedEventContext struct {
	ChannelID        string `json:"channelID,omitempty"`
	ConversationType string `json:"conversationType,omitempty"`
}

type mattermostSelectedChoice struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

func (service *Service) handleMattermostInteractiveAction(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	var payload mattermostInteractivePayload
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		service.writeMattermostInteractiveError(responseWriter, "invalid action request")
		return
	}
	if !service.isValidMattermostInteractivePayload(payload) {
		log.Printf("Mattermost interactive action rejected: invalid token action=%q postID=%q channelID=%q", strings.TrimSpace(payload.Context.Action), strings.TrimSpace(payload.PostID), strings.TrimSpace(payload.ChannelID))
		service.writeMattermostInteractiveError(responseWriter, "invalid action token")
		return
	}
	handler, isFound := service.mattermostInteractiveActionHandlers()[strings.TrimSpace(payload.Context.Action)]
	if !isFound {
		log.Printf("Mattermost interactive action rejected: unsupported action=%q postID=%q channelID=%q", strings.TrimSpace(payload.Context.Action), strings.TrimSpace(payload.PostID), strings.TrimSpace(payload.ChannelID))
		service.writeMattermostInteractiveError(responseWriter, "unsupported action")
		return
	}
	handler(responseWriter, request, payload)
}

func (service *Service) mattermostInteractiveActionHandlers() map[string]mattermostInteractiveActionHandler {
	return map[string]mattermostInteractiveActionHandler{
		attendanceClockInAction: func(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload) {
			service.handleAttendanceClockAction(responseWriter, request, payload, attendanceKindClockIn)
		},
		attendanceClockOutAction: func(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload) {
			service.handleAttendanceClockAction(responseWriter, request, payload, attendanceKindClockOut)
		},
		attendanceToggleAction: service.handleAttendanceToggleAction,
		"ask.confirm":          service.handleAskInteractiveAction,
		"ask.cancel":           service.handleAskInteractiveAction,
		"ask.choice":           service.handleAskInteractiveAction,
	}
}

func (service *Service) handleAskInteractiveAction(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload) {
	if !isMattermostAskActionTarget(payload) {
		service.writeMattermostAskTargetMismatch(responseWriter)
		return
	}
	service.deleteMattermostAskControlPost(request.Context(), payload.PostID)
	go service.deleteMattermostAskEphemeralPostInBackground(payload)
	go service.forwardMattermostAskActionInBackground(payload)
	service.writeMattermostInteractiveSuccess(responseWriter)
}

func (service *Service) forwardMattermostAskActionInBackground(payload mattermostInteractivePayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if errorValue := service.forwardMattermostAskAction(ctx, payload); errorValue != nil {
		log.Printf("mattermost ask action forward failed: %v", errorValue)
	}
}

func (service *Service) forwardMattermostAskAction(ctx context.Context, payload mattermostInteractivePayload) error {
	document, errorValue := json.Marshal(normalizedMattermostAskEventEnvelope(payload))
	if errorValue != nil {
		return errorValue
	}
	endpoint := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/connectors/mattermost/events"
	forwardRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	forwardRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(forwardRequest)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("Blueclaw ask action returned %d", response.StatusCode)
}

func normalizedMattermostAskEventEnvelope(payload mattermostInteractivePayload) normalizedConnectorEventEnvelope {
	action := strings.TrimSpace(payload.Context.Action)
	choiceKey := firstNonEmpty(strings.TrimSpace(payload.Context.ChoiceKey), mattermostSelectedChoiceKey(payload.SelectedOption))
	messageIDParts := []string{"ask", strings.TrimSpace(payload.PostID), action, strings.TrimSpace(payload.Context.InteractionID), choiceKey}
	return normalizedConnectorEventEnvelope{Event: normalizedConnectorEvent{
		ConversationID:   strings.TrimSpace(payload.Context.ConversationID),
		MessageID:        strings.Join(messageIDParts, ":"),
		SenderID:         strings.TrimSpace(payload.UserID),
		ReplyTargetID:    strings.TrimSpace(payload.Context.ReplyTargetID),
		Prompt:           mattermostAskActionPrompt(action, choiceKey),
		ResponseLanguage: strings.TrimSpace(payload.Context.ResponseLanguage),
		Context: normalizedEventContext{
			ChannelID:        strings.TrimSpace(payload.ChannelID),
			ConversationType: "direct",
		},
		LegacyFields: map[string]any{
			"askAction":     strings.TrimPrefix(action, "ask."),
			"interactionID": strings.TrimSpace(payload.Context.InteractionID),
			"taskRunID":     strings.TrimSpace(payload.Context.TaskRunID),
			"choiceKey":     choiceKey,
			"postID":        strings.TrimSpace(payload.PostID),
			"ephemeralAsk":  true,
		},
	}}
}

func mattermostAskActionPrompt(action string, choiceKey string) string {
	switch strings.TrimSpace(action) {
	case "ask.confirm":
		return "approved"
	case "ask.cancel":
		return "rejected"
	case "ask.choice":
		return "selected " + strings.TrimSpace(choiceKey)
	default:
		return strings.TrimSpace(action)
	}
}

func mattermostSelectedChoiceKey(selectedOption string) string {
	selectedChoice, isFound := parseMattermostSelectedChoice(selectedOption)
	if isFound {
		return selectedChoice.Key
	}
	return strings.TrimSpace(selectedOption)
}

func parseMattermostSelectedChoice(selectedOption string) (mattermostSelectedChoice, bool) {
	var selectedChoice mattermostSelectedChoice
	if errorValue := json.Unmarshal([]byte(strings.TrimSpace(selectedOption)), &selectedChoice); errorValue != nil {
		return mattermostSelectedChoice{}, false
	}
	selectedChoice.Key = strings.TrimSpace(selectedChoice.Key)
	selectedChoice.Label = strings.TrimSpace(selectedChoice.Label)
	if selectedChoice.Key == "" {
		return mattermostSelectedChoice{}, false
	}
	return selectedChoice, true
}

func (service *Service) isValidMattermostInteractivePayload(payload mattermostInteractivePayload) bool {
	actualToken := strings.TrimSpace(payload.Context.Token)
	if actualToken == "" {
		return false
	}
	for _, expectedToken := range service.mattermostInteractiveActionTokens() {
		if expectedToken == actualToken {
			return true
		}
	}
	return false
}

func (service *Service) mattermostInteractiveButton(actionID string, name string, tooltip string, style string) mattermostAction {
	return service.mattermostInteractiveButtonWithContext(actionID, name, tooltip, style, mattermostInteractiveContext{})
}

func (service *Service) mattermostInteractiveButtonWithContext(actionID string, name string, tooltip string, style string, context mattermostInteractiveContext) mattermostAction {
	if strings.TrimSpace(context.Action) == "" {
		context.Action = actionID
	}
	context.Token = service.ensureMattermostInteractiveActionToken()
	return mattermostinteractive.Button(actionID, name, tooltip, style, service.mattermostInteractiveActionURL(), context)
}

func (service *Service) writeMattermostInteractiveSuccess(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, mattermostInteractiveResponse{})
}

func (service *Service) writeMattermostInteractiveError(responseWriter http.ResponseWriter, message string) {
	service.writeJSON(responseWriter, mattermostInteractiveResponse{Error: &mattermostInteractiveError{Message: message}})
}

func (service *Service) writeMattermostAskTargetMismatch(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, mattermostInteractiveResponse{EphemeralText: "이 선택지는 요청한 사용자만 사용할 수 있습니다."})
}

func (service *Service) deleteMattermostAskControlPost(ctx context.Context, postID string) {
	trimmedPostID := strings.TrimSpace(postID)
	if trimmedPostID == "" {
		return
	}
	token, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return
	}
	deleteContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	path := "/api/v4/posts/" + url.PathEscape(trimmedPostID)
	if errorValue := service.mattermostRequest(deleteContext, http.MethodDelete, path, token, nil, nil); errorValue != nil && !isMattermostNotFound(errorValue) {
		log.Printf("mattermost ask control delete failed: %v", errorValue)
	}
}

func isMattermostAskActionTarget(payload mattermostInteractivePayload) bool {
	targetUserID := strings.TrimSpace(payload.Context.TargetUserID)
	if targetUserID == "" {
		return false
	}
	return strings.TrimSpace(payload.UserID) == targetUserID
}

func (service *Service) mattermostInteractiveActionURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(service.Configuration.MattermostInteractiveBaseURL), "/")
	if baseURL != "" {
		return baseURL + "/_internkim/mattermost/actions"
	}
	address := strings.TrimSpace(service.Configuration.ListenAddress)
	_, port, errorValue := net.SplitHostPort(address)
	if errorValue == nil && port != "" {
		return "http://127.0.0.1:" + port + "/_internkim/mattermost/actions"
	}
	return "http://" + strings.TrimRight(address, "/") + "/_internkim/mattermost/actions"
}

func (service *Service) ensureMattermostInteractiveActionToken() string {
	path := service.mattermostInteractiveActionTokenPath()
	token := strings.TrimSpace(readTrimmedFile(path))
	if token != "" {
		return token
	}
	token = randomHex(32)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return token
	}
	if errorValue := os.WriteFile(path, []byte(token+"\n"), 0o600); errorValue != nil {
		return token
	}
	return token
}

func (service *Service) mattermostInteractiveActionTokenPath() string {
	path := strings.TrimSpace(service.Configuration.MattermostInteractiveTokenPath)
	if path != "" && !service.isDefaultMattermostInteractiveActionTokenPath(path) {
		return path
	}
	return filepath.Join(service.Configuration.StateDirectory, "mattermost-interactive-token")
}

func (service *Service) isDefaultMattermostInteractiveActionTokenPath(path string) bool {
	return path == DefaultConfiguration().MattermostInteractiveTokenPath && service.Configuration.StateDirectory != DefaultConfiguration().StateDirectory
}

func (service *Service) mattermostInteractiveActionTokens() []string {
	tokens := []string{}
	for _, path := range service.mattermostInteractiveActionTokenPaths() {
		token := strings.TrimSpace(readTrimmedFile(path))
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func (service *Service) mattermostInteractiveActionTokenPaths() []string {
	paths := []string{service.mattermostInteractiveActionTokenPath()}
	legacyPath := filepath.Join(service.Configuration.StateDirectory, legacyMattermostInteractiveActionTokenFilename)
	if legacyPath != paths[0] {
		paths = append(paths, legacyPath)
	}
	return paths
}
