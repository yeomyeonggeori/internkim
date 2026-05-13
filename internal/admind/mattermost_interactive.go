package admind

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const mattermostInteractiveActionTokenFilename = "mattermost-interactive-action-token"

type mattermostInteractivePayload struct {
	UserID    string                       `json:"user_id"`
	PostID    string                       `json:"post_id"`
	ChannelID string                       `json:"channel_id"`
	TeamID    string                       `json:"team_id"`
	Context   mattermostInteractiveContext `json:"context"`
}

type mattermostInteractiveContext struct {
	Action string `json:"action"`
	Token  string `json:"token"`
}

type mattermostInteractiveResponse struct {
	Update        any                         `json:"update,omitempty"`
	EphemeralText string                      `json:"ephemeral_text,omitempty"`
	Error         *mattermostInteractiveError `json:"error,omitempty"`
}

type mattermostInteractiveError struct {
	Message string `json:"message"`
}

type mattermostAttachment struct {
	Fallback string             `json:"fallback"`
	Text     string             `json:"text"`
	Actions  []mattermostAction `json:"actions"`
}

type mattermostAction struct {
	ID          string                      `json:"id"`
	Type        string                      `json:"type"`
	Name        string                      `json:"name"`
	Tooltip     string                      `json:"tooltip,omitempty"`
	Style       string                      `json:"style,omitempty"`
	Integration mattermostActionIntegration `json:"integration"`
}

type mattermostActionIntegration struct {
	URL     string                       `json:"url"`
	Context mattermostInteractiveContext `json:"context"`
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
		service.writeMattermostInteractiveError(responseWriter, "invalid action token")
		return
	}
	switch strings.TrimSpace(payload.Context.Action) {
	case attendanceClockInAction:
		service.handleAttendanceClockAction(responseWriter, request, payload, attendanceKindClockIn)
	case attendanceClockOutAction:
		service.handleAttendanceClockAction(responseWriter, request, payload, attendanceKindClockOut)
	case attendanceToggleAction:
		service.handleAttendanceToggleAction(responseWriter, request, payload)
	default:
		service.writeMattermostInteractiveError(responseWriter, "unsupported action")
	}
}

func (service *Service) isValidMattermostInteractivePayload(payload mattermostInteractivePayload) bool {
	expectedToken := strings.TrimSpace(readTrimmedFile(service.mattermostInteractiveActionTokenPath()))
	actualToken := strings.TrimSpace(payload.Context.Token)
	if expectedToken == "" || actualToken == "" {
		return false
	}
	return expectedToken == actualToken
}

func (service *Service) mattermostInteractiveButton(actionID string, name string, tooltip string, style string) mattermostAction {
	return mattermostAction{
		ID:      actionID,
		Type:    "button",
		Name:    name,
		Tooltip: tooltip,
		Style:   style,
		Integration: mattermostActionIntegration{
			URL: service.mattermostInteractiveActionURL(),
			Context: mattermostInteractiveContext{
				Action: actionID,
				Token:  service.ensureMattermostInteractiveActionToken(),
			},
		},
	}
}

func (service *Service) writeMattermostInteractiveSuccess(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, mattermostInteractiveResponse{})
}

func (service *Service) writeMattermostInteractiveError(responseWriter http.ResponseWriter, message string) {
	service.writeJSON(responseWriter, mattermostInteractiveResponse{Error: &mattermostInteractiveError{Message: message}})
}

func (service *Service) mattermostInteractiveActionURL() string {
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
	return filepath.Join(service.Configuration.StateDirectory, mattermostInteractiveActionTokenFilename)
}
