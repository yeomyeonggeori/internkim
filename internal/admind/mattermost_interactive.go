package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/mattermostinteractive"
)

const mattermostInteractiveActionTokenFilename = "mattermost-interactive-action-token"

type mattermostInteractivePayload = mattermostinteractive.Payload
type mattermostInteractiveContext = mattermostinteractive.Context
type mattermostInteractiveResponse = mattermostinteractive.Response
type mattermostInteractiveError = mattermostinteractive.Error

type mattermostInteractiveActionHandler func(http.ResponseWriter, *http.Request, mattermostInteractivePayload)

type mattermostAttachment = mattermostinteractive.Attachment
type mattermostAction = mattermostinteractive.Action

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
	handler, isFound := service.mattermostInteractiveActionHandlers()[strings.TrimSpace(payload.Context.Action)]
	if !isFound {
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
	go service.forwardMattermostAskActionInBackground(payload)
	service.writeJSON(responseWriter, mattermostInteractiveResponse{Update: mattermostAskResolvedUpdate()})
}

func (service *Service) forwardMattermostAskActionInBackground(payload mattermostInteractivePayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if errorValue := service.forwardMattermostAskAction(ctx, payload); errorValue != nil {
		log.Printf("mattermost ask action forward failed: %v", errorValue)
	}
}

func (service *Service) forwardMattermostAskAction(ctx context.Context, payload mattermostInteractivePayload) error {
	document, errorValue := json.Marshal(payload)
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

func mattermostAskResolvedUpdate() map[string]any {
	return mattermostinteractive.ClearAttachmentsUpdate()
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
	return service.mattermostInteractiveButtonWithContext(actionID, name, tooltip, style, mattermostInteractiveContext{})
}

func (service *Service) mattermostInteractiveButtonWithContext(actionID string, name string, tooltip string, style string, context mattermostInteractiveContext) mattermostAction {
	context.Action = actionID
	context.Token = service.ensureMattermostInteractiveActionToken()
	return mattermostinteractive.Button(actionID, name, tooltip, style, service.mattermostInteractiveActionURL(), context)
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
