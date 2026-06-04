package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (service *Service) ensureMattermostUserAccessToken(ctx context.Context, adminToken string, userID string) (string, error) {
	records := service.readMattermostUserTokenRecords()
	if token := strings.TrimSpace(records[userID].Token); token != "" && service.isValidMattermostUserToken(ctx, token, userID) {
		return token, nil
	}
	token, errorValue := service.createMattermostUserAccessToken(ctx, adminToken, userID)
	if errorValue != nil {
		return "", errorValue
	}
	records[userID] = attendanceUserTokenRecord{
		UserID:    userID,
		Token:     token,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return token, service.writeMattermostUserTokenRecords(records)
}

func (service *Service) createMattermostUserAccessToken(ctx context.Context, adminToken string, userID string) (string, error) {
	var response mattermostTokenResponse
	body := map[string]string{"description": "internkim-attendance"}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/users/"+url.PathEscape(userID)+"/tokens", adminToken, body, &response); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(response.Token) == "" {
		return "", fmt.Errorf("Mattermost user token was not returned")
	}
	return strings.TrimSpace(response.Token), nil
}

func (service *Service) isValidMattermostUserToken(ctx context.Context, token string, userID string) bool {
	var userRecord mattermostUserRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", token, nil, &userRecord)
	return errorValue == nil && userRecord.ID == userID
}

func (service *Service) readMattermostUserTokenRecords() map[string]attendanceUserTokenRecord {
	document, errorValue := os.ReadFile(service.mattermostUserTokenPath())
	if errorValue != nil {
		return map[string]attendanceUserTokenRecord{}
	}
	var records map[string]attendanceUserTokenRecord
	if errorValue := json.Unmarshal(document, &records); errorValue != nil || records == nil {
		return map[string]attendanceUserTokenRecord{}
	}
	return records
}

func (service *Service) writeMattermostUserTokenRecords(records map[string]attendanceUserTokenRecord) error {
	document, errorValue := json.MarshalIndent(records, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.mattermostUserTokenPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func (service *Service) mattermostUserTokenPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "mattermost-user-tokens.json")
}
