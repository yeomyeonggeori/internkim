package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type mattermostPostRepairRequest struct {
	Message  *string         `json:"message,omitempty"`
	RootID   *string         `json:"rootID,omitempty"`
	CreateAt json.RawMessage `json:"createAt,omitempty"`
	UpdateAt json.RawMessage `json:"updateAt,omitempty"`
	EditAt   json.RawMessage `json:"editAt,omitempty"`
	DryRun   bool            `json:"dryRun,omitempty"`
}

type mattermostPostRepairResponse struct {
	PostID  string                         `json:"postID"`
	DryRun  bool                           `json:"dryRun"`
	Before  mattermostDatabasePostSnapshot `json:"before"`
	After   mattermostDatabasePostSnapshot `json:"after"`
	Updates []string                       `json:"updates"`
}

type mattermostDatabasePostSnapshot struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelID"`
	RootID    string `json:"rootID"`
	Message   string `json:"message"`
	CreateAt  int64  `json:"createAt"`
	UpdateAt  int64  `json:"updateAt"`
	EditAt    int64  `json:"editAt"`
	DeleteAt  int64  `json:"deleteAt"`
}

type mattermostPostRepairUpdate struct {
	column string
	value  string
	label  string
}

func (service *Service) authorizeMattermostPostMaintenance(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	callerEmail := service.authenticatedCallerEmail(request)
	if callerEmail == "" {
		return false
	}
	claimedAdminEmail := service.claimedAdminEmail()
	if claimedAdminEmail != "" {
		return strings.EqualFold(callerEmail, claimedAdminEmail)
	}
	return strings.EqualFold(callerEmail, service.seedAdminEmail())
}

func parseMattermostRepairTimestamp(document json.RawMessage) (int64, error) {
	var numberValue int64
	if errorValue := json.Unmarshal(document, &numberValue); errorValue == nil {
		return numberValue, nil
	}
	var stringValue string
	if errorValue := json.Unmarshal(document, &stringValue); errorValue != nil {
		return 0, errorValue
	}
	trimmedValue := strings.TrimSpace(stringValue)
	if trimmedValue == "" {
		return 0, fmt.Errorf("empty timestamp")
	}
	if numberValue, errorValue := strconv.ParseInt(trimmedValue, 10, 64); errorValue == nil {
		return numberValue, nil
	}
	parsedTime, errorValue := time.Parse(time.RFC3339Nano, trimmedValue)
	if errorValue != nil {
		return 0, errorValue
	}
	return parsedTime.UnixMilli(), nil
}

func (service *Service) runPostgresMattermostQuery(ctx context.Context, query string) ([]byte, error) {
	output, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql mattermost -tAc "+shellQuote(query))
	if errorValue != nil {
		return nil, fmt.Errorf("%w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func logMattermostPostRepair(request mattermostPostRepairRequest, postID string, updates []string) {
	logPrefix := "Mattermost post maintenance repair"
	if request.DryRun {
		logPrefix = "Mattermost post maintenance repair dry-run"
	}
	log.Printf("%s postID=%s updates=%s", logPrefix, postID, strings.Join(updates, ","))
}

func sqlStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
