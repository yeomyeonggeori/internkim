package admind

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const mattermostConnectCommandTrigger = "connect"
const mattermostConnectCommandTokenFilename = "mattermost-connect-command-token"

type mattermostCommandRecord struct {
	ID               string `json:"id,omitempty"`
	Token            string `json:"token,omitempty"`
	TeamID           string `json:"team_id"`
	Trigger          string `json:"trigger"`
	Method           string `json:"method"`
	URL              string `json:"url"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	Autocomplete     bool   `json:"auto_complete"`
	AutocompleteDesc string `json:"auto_complete_desc"`
	AutocompleteHint string `json:"auto_complete_hint"`
}

type mattermostSlashCommandResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

func (service *Service) handleMattermostCommand(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if errorValue := request.ParseForm(); errorValue != nil {
		http.Error(responseWriter, "invalid command request", http.StatusBadRequest)
		return
	}
	if !service.isValidMattermostCommandToken(request) {
		http.Error(responseWriter, "invalid command token", http.StatusForbidden)
		return
	}
	if strings.TrimPrefix(request.FormValue("command"), "/") != mattermostConnectCommandTrigger {
		service.writeMattermostCommandResponse(responseWriter, "지원하지 않는 명령입니다. `/connect`로 Companion 앱을 연결하세요.")
		return
	}
	text := strings.ToLower(strings.TrimSpace(request.FormValue("text")))
	if text != "" && text != "help" && text != "도움말" {
		service.writeMattermostCommandResponse(responseWriter, "`/connect`는 Companion 앱 연결만 처리합니다.")
		return
	}
	service.handleMattermostConnectCommand(responseWriter, request)
}

func (service *Service) handleMattermostConnectCommand(responseWriter http.ResponseWriter, request *http.Request) {
	userRecord, errorValue := service.mattermostSlashCommandUser(request.Context(), request.FormValue("user_id"), request.FormValue("user_name"))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	owner := companionPairingCodeRequest{
		OwnerPlatform:       "mattermost",
		OwnerPlatformUserID: strings.TrimSpace(userRecord.ID),
		OwnerEmail:          strings.ToLower(strings.TrimSpace(userRecord.Email)),
		OwnerName:           mattermostDisplayName(userRecord),
		DeviceURL:           service.publicDeviceURL(request),
	}
	response := service.createCompanionPairingCodeForOwner(request, owner)
	service.writeMattermostCommandResponse(responseWriter, mattermostConnectCommandMessage(response))
}

func (service *Service) mattermostSlashCommandUser(ctx context.Context, userID string, username string) (mattermostUserRecord, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	if strings.TrimSpace(userID) != "" {
		userRecord, found, errorValue := service.findMattermostUserByID(ctx, token, userID)
		if errorValue != nil {
			return mattermostUserRecord{}, errorValue
		}
		if found {
			return userRecord, nil
		}
	}
	userRecord, found, errorValue := service.findMattermostUserByUsername(ctx, token, username)
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	if !found {
		return mattermostUserRecord{}, errors.New("Mattermost command user was not found")
	}
	return userRecord, nil
}

func (service *Service) writeMattermostCommandResponse(responseWriter http.ResponseWriter, text string) {
	service.writeJSON(responseWriter, mattermostSlashCommandResponse{ResponseType: "ephemeral", Text: text})
}

func (service *Service) isValidMattermostCommandToken(request *http.Request) bool {
	expectedToken := strings.TrimSpace(readTrimmedFile(service.mattermostConnectCommandTokenPath()))
	actualToken := strings.TrimSpace(request.FormValue("token"))
	if actualToken == "" {
		actualToken = strings.TrimPrefix(strings.TrimSpace(request.Header.Get("Authorization")), "Bearer ")
	}
	if expectedToken == "" || actualToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expectedToken), []byte(actualToken)) == 1
}

func (service *Service) ensureMattermostConnectCommand(ctx context.Context, token string) error {
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	commandRecord, found, errorValue := service.findMattermostConnectCommand(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return errorValue
	}
	if found {
		return service.ensureMattermostConnectCommandToken(ctx, token, commandRecord)
	}
	return service.createMattermostConnectCommand(ctx, token, teamRecord.ID)
}

func (service *Service) findMattermostConnectCommand(ctx context.Context, token string, teamID string) (mattermostCommandRecord, bool, error) {
	var records []mattermostCommandRecord
	path := "/api/v4/commands?team_id=" + url.QueryEscape(teamID)
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &records); errorValue != nil {
		return mattermostCommandRecord{}, false, errorValue
	}
	for _, record := range records {
		if record.Trigger == mattermostConnectCommandTrigger && record.TeamID == teamID {
			return record, true, nil
		}
	}
	return mattermostCommandRecord{}, false, nil
}

func (service *Service) ensureMattermostConnectCommandToken(ctx context.Context, token string, record mattermostCommandRecord) error {
	if strings.TrimSpace(readTrimmedFile(service.mattermostConnectCommandTokenPath())) != "" {
		return service.updateMattermostConnectCommand(ctx, token, record)
	}
	if strings.TrimSpace(record.Token) != "" {
		return service.writeMattermostConnectCommandToken(record.Token)
	}
	if errorValue := service.archiveMattermostConnectCommand(ctx, token, record.ID); errorValue != nil {
		return errorValue
	}
	return service.createMattermostConnectCommand(ctx, token, record.TeamID)
}

func (service *Service) createMattermostConnectCommand(ctx context.Context, token string, teamID string) error {
	var commandRecord mattermostCommandRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/commands", token, service.mattermostConnectCommandPayload(teamID, ""), &commandRecord); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(commandRecord.Token) == "" {
		return fmt.Errorf("Mattermost /connect command did not return a token")
	}
	return service.writeMattermostConnectCommandToken(commandRecord.Token)
}

func (service *Service) updateMattermostConnectCommand(ctx context.Context, token string, record mattermostCommandRecord) error {
	if strings.TrimSpace(record.ID) == "" {
		return nil
	}
	path := "/api/v4/commands/" + url.PathEscape(record.ID)
	return service.mattermostRequest(ctx, http.MethodPut, path, token, service.mattermostConnectCommandPayload(record.TeamID, record.ID), nil)
}

func (service *Service) archiveMattermostConnectCommand(ctx context.Context, token string, commandID string) error {
	if strings.TrimSpace(commandID) == "" {
		return nil
	}
	return service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/commands/"+url.PathEscape(commandID), token, nil, nil)
}

func (service *Service) mattermostConnectCommandPayload(teamID string, commandID string) mattermostCommandRecord {
	return mattermostCommandRecord{
		ID:               strings.TrimSpace(commandID),
		TeamID:           strings.TrimSpace(teamID),
		Trigger:          mattermostConnectCommandTrigger,
		Method:           "P",
		URL:              service.mattermostConnectCommandURL(),
		DisplayName:      "Connect Companion",
		Description:      "Connect your InternKim Companion app.",
		Autocomplete:     true,
		AutocompleteDesc: "Connect your Companion app",
	}
}

func (service *Service) mattermostConnectCommandURL() string {
	address := strings.TrimSpace(service.Configuration.ListenAddress)
	_, port, errorValue := net.SplitHostPort(address)
	if errorValue == nil && port != "" {
		return "http://127.0.0.1:" + port + "/_internkim/mattermost/commands"
	}
	return "http://" + strings.TrimRight(address, "/") + "/_internkim/mattermost/commands"
}

func (service *Service) mattermostConnectCommandTokenPath() string {
	return filepath.Join(service.Configuration.StateDirectory, mattermostConnectCommandTokenFilename)
}

func (service *Service) writeMattermostConnectCommandToken(token string) error {
	path := service.mattermostConnectCommandTokenPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o750); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(token)+"\n"), 0o600)
}

func (service *Service) publicDeviceURL(request *http.Request) string {
	deviceID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceIDPath)))
	if deviceID != "" {
		return "https://" + deviceID + ".example.test"
	}
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + request.Host
}

func mattermostDisplayName(userRecord mattermostUserRecord) string {
	return firstNonEmpty(userRecord.DisplayName, userRecord.Nickname, userRecord.FirstName, userRecord.Username)
}

func mattermostConnectCommandMessage(response companionPairingCodeResponse) string {
	expiresAt := response.ExpiresAt.Local().Format("15:04")
	return "Companion 연결 코드: `" + response.Code + "`\n" +
		"Companion 앱에서 이 링크를 열거나 코드를 입력하세요.\n" +
		response.DeepLink + "\n" +
		"만료: " + expiresAt
}
