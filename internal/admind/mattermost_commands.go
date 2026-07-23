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
const mattermostStopCommandTrigger = "stop"
const mattermostStopAllCommandTrigger = "stop-all"
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
	Username     string `json:"username,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
}

type blueclawTaskStopResponse struct {
	CancelledTaskRunCount int  `json:"cancelledTaskRunCount"`
	MultipleTargets       bool `json:"multipleTargets"`
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
	command := strings.TrimPrefix(strings.TrimSpace(request.FormValue("command")), "/")
	text := strings.ToLower(strings.TrimSpace(request.FormValue("text")))
	switch command {
	case mattermostConnectCommandTrigger:
		if text != "" && text != "help" && text != "도움말" {
			service.writeMattermostCommandResponse(responseWriter, request, "`/connect`는 Companion 앱 연결만 처리합니다.")
			return
		}
		service.handleMattermostConnectCommand(responseWriter, request)
	case mattermostStopCommandTrigger, mattermostStopAllCommandTrigger:
		service.handleMattermostStopCommand(responseWriter, request, command)
	default:
		service.writeMattermostCommandResponse(responseWriter, request, "지원하지 않는 명령입니다. `/connect`, `/stop`, `/stop-all`을 사용할 수 있습니다.")
		return
	}
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
	service.writeMattermostCommandResponse(responseWriter, request, mattermostConnectCommandMessage(response))
}

func (service *Service) handleMattermostStopCommand(responseWriter http.ResponseWriter, request *http.Request, command string) {
	userRecord, errorValue := service.mattermostSlashCommandUser(request.Context(), request.FormValue("user_id"), request.FormValue("user_name"))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	mode := "stop"
	if command == mattermostStopAllCommandTrigger {
		mode = "stop_all"
	}
	stopResponse := blueclawTaskStopResponse{}
	body := map[string]any{
		"mode":                  mode,
		"requesterEmail":        strings.ToLower(strings.TrimSpace(userRecord.Email)),
		"originConversationIDs": mattermostSlashCommandConversationIDs(request),
		"reason":                "mattermost slash command /" + command,
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/task/cancel", body, &stopResponse); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeMattermostCommandResponse(responseWriter, request, mattermostStopCommandMessage(stopResponse))
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

func (service *Service) writeMattermostCommandResponse(responseWriter http.ResponseWriter, request *http.Request, text string) {
	service.writeJSON(responseWriter, mattermostSlashCommandResponse{
		ResponseType: "ephemeral",
		Text:         text,
		Username:     service.mattermostCommandDisplayName(),
		IconURL:      strings.TrimRight(service.publicDeviceURL(request), "/") + "/logo.svg",
	})
}

func (service *Service) mattermostCommandDisplayName() string {
	profile, found := service.loadBotProfile()
	if found {
		return profile.DisplayName
	}
	return defaultBotProfile().DisplayName
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
	for _, token := range strings.Fields(expectedToken) {
		if subtle.ConstantTimeCompare([]byte(token), []byte(actualToken)) == 1 {
			return true
		}
	}
	return false
}

func (service *Service) ensureMattermostConnectCommand(ctx context.Context, token string) error {
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	for _, trigger := range mattermostManagedCommandTriggers() {
		commandRecord, found, errorValue := service.findMattermostCommand(ctx, token, teamRecord.ID, trigger)
		if errorValue != nil {
			return errorValue
		}
		if found {
			if errorValue := service.ensureMattermostCommandToken(ctx, token, commandRecord, trigger); errorValue != nil {
				return errorValue
			}
			continue
		}
		if errorValue := service.createMattermostCommand(ctx, token, teamRecord.ID, trigger); errorValue != nil {
			return errorValue
		}
	}
	for _, trigger := range mattermostDeprecatedCommandTriggers() {
		commandRecord, found, errorValue := service.findMattermostCommand(ctx, token, teamRecord.ID, trigger)
		if errorValue != nil {
			return errorValue
		}
		if found {
			if errorValue := service.archiveMattermostConnectCommand(ctx, token, commandRecord.ID); errorValue != nil {
				return errorValue
			}
		}
	}
	return nil
}

func (service *Service) findMattermostCommand(ctx context.Context, token string, teamID string, trigger string) (mattermostCommandRecord, bool, error) {
	var records []mattermostCommandRecord
	path := "/api/v4/commands?team_id=" + url.QueryEscape(teamID)
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &records); errorValue != nil {
		return mattermostCommandRecord{}, false, errorValue
	}
	for _, record := range records {
		if record.Trigger == trigger && record.TeamID == teamID {
			return record, true, nil
		}
	}
	return mattermostCommandRecord{}, false, nil
}

func (service *Service) ensureMattermostCommandToken(ctx context.Context, token string, record mattermostCommandRecord, trigger string) error {
	if strings.TrimSpace(readTrimmedFile(service.mattermostConnectCommandTokenPath())) != "" {
		return service.updateMattermostCommand(ctx, token, record, trigger)
	}
	if strings.TrimSpace(record.Token) != "" {
		return service.writeMattermostCommandToken(record.Token)
	}
	if errorValue := service.archiveMattermostConnectCommand(ctx, token, record.ID); errorValue != nil {
		return errorValue
	}
	return service.createMattermostCommand(ctx, token, record.TeamID, trigger)
}

func (service *Service) createMattermostCommand(ctx context.Context, token string, teamID string, trigger string) error {
	var commandRecord mattermostCommandRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/commands", token, service.mattermostCommandPayload(teamID, "", trigger), &commandRecord); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(commandRecord.Token) == "" {
		return fmt.Errorf("Mattermost /%s command did not return a token", trigger)
	}
	return service.writeMattermostCommandToken(commandRecord.Token)
}

func (service *Service) updateMattermostCommand(ctx context.Context, token string, record mattermostCommandRecord, trigger string) error {
	if strings.TrimSpace(record.ID) == "" {
		return nil
	}
	path := "/api/v4/commands/" + url.PathEscape(record.ID)
	return service.mattermostRequest(ctx, http.MethodPut, path, token, service.mattermostCommandPayload(record.TeamID, record.ID, trigger), nil)
}

func (service *Service) archiveMattermostConnectCommand(ctx context.Context, token string, commandID string) error {
	if strings.TrimSpace(commandID) == "" {
		return nil
	}
	return service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/commands/"+url.PathEscape(commandID), token, nil, nil)
}

func (service *Service) mattermostCommandPayload(teamID string, commandID string, trigger string) mattermostCommandRecord {
	commandRecord := mattermostCommandRecord{
		ID:           strings.TrimSpace(commandID),
		TeamID:       strings.TrimSpace(teamID),
		Trigger:      trigger,
		Method:       "P",
		URL:          service.mattermostConnectCommandURL(),
		Autocomplete: true,
	}
	switch trigger {
	case mattermostStopCommandTrigger:
		commandRecord.DisplayName = "Stop InternKim task"
		commandRecord.Description = "Stop your current InternKim task."
		commandRecord.AutocompleteDesc = "Stop your current task"
	case mattermostStopAllCommandTrigger:
		commandRecord.DisplayName = "Stop all InternKim tasks"
		commandRecord.Description = "Stop all of your active InternKim tasks."
		commandRecord.AutocompleteDesc = "Stop all active tasks"
	default:
		commandRecord.DisplayName = "Connect Companion"
		commandRecord.Description = "Connect your InternKim Companion app."
		commandRecord.AutocompleteDesc = "Connect your Companion app"
	}
	return commandRecord
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

func (service *Service) writeMattermostCommandToken(token string) error {
	path := service.mattermostConnectCommandTokenPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o750); errorValue != nil {
		return errorValue
	}
	tokenSet := map[string]bool{}
	tokens := []string{}
	for _, value := range strings.Fields(readTrimmedFile(path)) {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" || tokenSet[trimmedValue] {
			continue
		}
		tokenSet[trimmedValue] = true
		tokens = append(tokens, trimmedValue)
	}
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken != "" && !tokenSet[trimmedToken] {
		tokens = append(tokens, trimmedToken)
	}
	return os.WriteFile(path, []byte(strings.Join(tokens, "\n")+"\n"), 0o600)
}

func (service *Service) publicDeviceURL(request *http.Request) string {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	if fleetID != "" {
		return "https://" + fleetID + ".intern.kim"
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
		"[Companion 앱 열기](" + response.DeepLink + ")\n" +
		"만료: " + expiresAt
}

func mattermostManagedCommandTriggers() []string {
	return []string{
		mattermostConnectCommandTrigger,
		mattermostStopCommandTrigger,
		mattermostStopAllCommandTrigger,
	}
}

func mattermostDeprecatedCommandTriggers() []string {
	return []string{"중단", "중단-전부"}
}

func mattermostSlashCommandConversationIDs(request *http.Request) []string {
	channelID := strings.TrimSpace(request.FormValue("channel_id"))
	if channelID == "" {
		return nil
	}
	return []string{"channel:" + channelID, "dm:" + channelID, "group:" + channelID}
}

func mattermostStopCommandMessage(response blueclawTaskStopResponse) string {
	if response.MultipleTargets {
		return "진행 중인 작업이 여러 개입니다. 모두 멈추려면 `/stop-all`을 사용해 주세요."
	}
	if response.CancelledTaskRunCount == 0 {
		return "현재 중단할 작업이 없습니다."
	}
	return fmt.Sprintf("진행 중인 작업 %d개를 중단했습니다. 예약된 반복 실행은 유지됩니다.", response.CancelledTaskRunCount)
}
