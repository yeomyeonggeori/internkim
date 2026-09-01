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

	"gitlab.com/eastriver/internkim/internal/fleetdomain"
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

func (service *Service) archiveMattermostConnectCommand(ctx context.Context, token string, commandID string) error {
	if strings.TrimSpace(commandID) == "" {
		return nil
	}
	return service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/commands/"+url.PathEscape(commandID), token, nil, nil)
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
		return fleetdomain.Subdomain(fleetID, service.fleetZone())
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

func mattermostStopCommandMessage(response blueclawTaskStopResponse) string {
	if response.MultipleTargets {
		return "진행 중인 작업이 여러 개입니다. 모두 멈추려면 `/stop-all`을 사용해 주세요."
	}
	if response.CancelledTaskRunCount == 0 {
		return "현재 중단할 작업이 없습니다."
	}
	return fmt.Sprintf("진행 중인 작업 %d개를 중단했습니다. 예약된 반복 실행은 유지됩니다.", response.CancelledTaskRunCount)
}
