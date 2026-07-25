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

func (service *Service) repairMattermostPost(responseWriter http.ResponseWriter, request *http.Request, postID string) {
	if !service.authorizeMattermostPostMaintenance(request) {
		http.Error(responseWriter, "claimed admin or local request required", http.StatusForbidden)
		return
	}
	var body mattermostPostRepairRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	response, errorValue := service.repairMattermostPostRecord(request.Context(), postID, body)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, response)
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

func (service *Service) repairMattermostPostRecord(ctx context.Context, postID string, request mattermostPostRepairRequest) (mattermostPostRepairResponse, error) {
	trimmedPostID := strings.TrimSpace(postID)
	if trimmedPostID == "" {
		return mattermostPostRepairResponse{}, fmt.Errorf("postID is required")
	}
	before, errorValue := service.readMattermostDatabasePost(ctx, trimmedPostID)
	if errorValue != nil {
		return mattermostPostRepairResponse{}, errorValue
	}
	updates, after, errorValue := service.prepareMattermostPostRepair(ctx, before, request)
	if errorValue != nil {
		return mattermostPostRepairResponse{}, errorValue
	}
	if len(updates) == 0 {
		return mattermostPostRepairResponse{}, fmt.Errorf("no supported post fields were provided")
	}
	if !request.DryRun {
		if errorValue := service.updateMattermostDatabasePost(ctx, trimmedPostID, updates); errorValue != nil {
			return mattermostPostRepairResponse{}, errorValue
		}
	}
	labels := mattermostPostRepairUpdateLabels(updates)
	logMattermostPostRepair(request, trimmedPostID, labels)
	return mattermostPostRepairResponse{PostID: trimmedPostID, DryRun: request.DryRun, Before: before, After: after, Updates: labels}, nil
}

func (service *Service) prepareMattermostPostRepair(ctx context.Context, before mattermostDatabasePostSnapshot, request mattermostPostRepairRequest) ([]mattermostPostRepairUpdate, mattermostDatabasePostSnapshot, error) {
	after := before
	updates := []mattermostPostRepairUpdate{}
	if request.Message != nil {
		after.Message = *request.Message
		updates = append(updates, mattermostPostRepairUpdate{column: "message", value: sqlStringLiteral(*request.Message), label: "message"})
	}
	if request.RootID != nil {
		rootID := strings.TrimSpace(*request.RootID)
		if rootID == before.ID {
			return nil, mattermostDatabasePostSnapshot{}, fmt.Errorf("rootID cannot equal postID")
		}
		if rootID != "" {
			rootPost, errorValue := service.readMattermostDatabasePost(ctx, rootID)
			if errorValue != nil {
				return nil, mattermostDatabasePostSnapshot{}, fmt.Errorf("root post %s was not found: %w", rootID, errorValue)
			}
			if rootPost.DeleteAt != 0 {
				return nil, mattermostDatabasePostSnapshot{}, fmt.Errorf("root post %s is deleted", rootID)
			}
			if rootPost.ChannelID != before.ChannelID {
				return nil, mattermostDatabasePostSnapshot{}, fmt.Errorf("root post must be in the same channel")
			}
		}
		after.RootID = rootID
		updates = append(updates, mattermostPostRepairUpdate{column: "rootid", value: sqlStringLiteral(rootID), label: "rootID"})
	}
	timestampUpdates, timestampAfter, errorValue := prepareMattermostTimestampUpdates(request, after)
	if errorValue != nil {
		return nil, mattermostDatabasePostSnapshot{}, errorValue
	}
	updates = append(updates, timestampUpdates...)
	return updates, timestampAfter, nil
}

func prepareMattermostTimestampUpdates(request mattermostPostRepairRequest, after mattermostDatabasePostSnapshot) ([]mattermostPostRepairUpdate, mattermostDatabasePostSnapshot, error) {
	updates := []mattermostPostRepairUpdate{}
	values := []struct {
		name     string
		column   string
		document json.RawMessage
		assign   func(int64)
	}{
		{name: "createAt", column: "createat", document: request.CreateAt, assign: func(value int64) { after.CreateAt = value }},
		{name: "updateAt", column: "updateat", document: request.UpdateAt, assign: func(value int64) { after.UpdateAt = value }},
		{name: "editAt", column: "editat", document: request.EditAt, assign: func(value int64) { after.EditAt = value }},
	}
	for _, item := range values {
		if len(item.document) == 0 {
			continue
		}
		timestamp, errorValue := parseMattermostRepairTimestamp(item.document)
		if errorValue != nil {
			return nil, mattermostDatabasePostSnapshot{}, fmt.Errorf("%s is invalid: %w", item.name, errorValue)
		}
		item.assign(timestamp)
		updates = append(updates, mattermostPostRepairUpdate{column: item.column, value: strconv.FormatInt(timestamp, 10), label: item.name})
	}
	return updates, after, nil
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

func (service *Service) readMattermostDatabasePost(ctx context.Context, postID string) (mattermostDatabasePostSnapshot, error) {
	query := "SELECT row_to_json(post_row)::text FROM (SELECT id, channelid, rootid, message, createat, updateat, editat, deleteat FROM posts WHERE id = " + sqlStringLiteral(postID) + ") post_row"
	output, errorValue := service.runPostgresMattermostQuery(ctx, query)
	if errorValue != nil {
		return mattermostDatabasePostSnapshot{}, errorValue
	}
	document := strings.TrimSpace(string(output))
	if document == "" {
		return mattermostDatabasePostSnapshot{}, fmt.Errorf("Mattermost post %s was not found", postID)
	}
	var row struct {
		ID        string `json:"id"`
		ChannelID string `json:"channelid"`
		RootID    string `json:"rootid"`
		Message   string `json:"message"`
		CreateAt  int64  `json:"createat"`
		UpdateAt  int64  `json:"updateat"`
		EditAt    int64  `json:"editat"`
		DeleteAt  int64  `json:"deleteat"`
	}
	if errorValue := json.Unmarshal([]byte(document), &row); errorValue != nil {
		return mattermostDatabasePostSnapshot{}, errorValue
	}
	return mattermostDatabasePostSnapshot{
		ID:        row.ID,
		ChannelID: row.ChannelID,
		RootID:    row.RootID,
		Message:   row.Message,
		CreateAt:  row.CreateAt,
		UpdateAt:  row.UpdateAt,
		EditAt:    row.EditAt,
		DeleteAt:  row.DeleteAt,
	}, nil
}

func (service *Service) updateMattermostDatabasePost(ctx context.Context, postID string, updates []mattermostPostRepairUpdate) error {
	assignments := make([]string, 0, len(updates))
	for _, update := range updates {
		assignments = append(assignments, update.column+" = "+update.value)
	}
	query := "UPDATE posts SET " + strings.Join(assignments, ", ") + " WHERE id = " + sqlStringLiteral(postID)
	_, errorValue := service.runPostgresMattermostQuery(ctx, query)
	return errorValue
}

func (service *Service) runPostgresMattermostQuery(ctx context.Context, query string) ([]byte, error) {
	output, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql mattermost -tAc "+shellQuote(query))
	if errorValue != nil {
		return nil, fmt.Errorf("%w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func mattermostPostRepairUpdateLabels(updates []mattermostPostRepairUpdate) []string {
	labels := make([]string, 0, len(updates))
	for _, update := range updates {
		labels = append(labels, update.label)
	}
	return labels
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
