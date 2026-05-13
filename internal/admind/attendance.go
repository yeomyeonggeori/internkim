package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"

	_ "modernc.org/sqlite"
)

const (
	attendanceKindClockIn            = "clock_in"
	attendanceKindClockOut           = "clock_out"
	attendanceSourceMattermostButton = "mattermost_button"
	attendanceChannelName            = mattermostdefaults.AttendanceChannelName
	attendanceChannelDisplayName     = mattermostdefaults.AttendanceChannelDisplayName
	attendanceToggleAction           = "attendance.toggle"
	attendanceEntryPostMessage       = "출퇴근 기록"
	attendanceEntryPostProperty      = "internkim_attendance_entry"
	attendanceCancelReason           = "repeated click confirmed"
	attendanceDuplicateWindow        = 5 * time.Minute
)

const attendanceEventSelectColumns = `
id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, canceled_at, cancel_reason, repeated_click_at`

var errAttendanceDuplicateIgnored = errors.New("attendance duplicate ignored")

type attendanceEvent struct {
	ID                 string `json:"id"`
	MattermostUserID   string `json:"mattermostUserID"`
	MattermostUsername string `json:"mattermostUsername"`
	Email              string `json:"email"`
	DisplayName        string `json:"displayName"`
	Kind               string `json:"kind"`
	OccurredAt         string `json:"occurredAt"`
	LocalDate          string `json:"localDate"`
	LocalTime          string `json:"localTime"`
	TimeZoneAtEvent    string `json:"timeZoneAtEvent"`
	Source             string `json:"source"`
	TeamID             string `json:"teamID"`
	ChannelID          string `json:"channelID"`
	ActionPostID       string `json:"actionPostID"`
	ResultPostID       string `json:"resultPostID"`
	CanceledAt         string `json:"canceledAt,omitempty"`
	CancelReason       string `json:"cancelReason,omitempty"`
	RepeatedClickAt    string `json:"repeatedClickAt,omitempty"`
}

type attendanceSummaryResponse struct {
	Month            string            `json:"month"`
	CurrentUserEmail string            `json:"currentUserEmail"`
	IsAdmin          bool              `json:"isAdmin"`
	TimeZone         string            `json:"timeZone"`
	Events           []attendanceEvent `json:"events"`
	TodayStatus      string            `json:"todayStatus"`
}

type attendanceUserTokenRecord struct {
	UserID    string `json:"userID"`
	Token     string `json:"token"`
	UpdatedAt string `json:"updatedAt"`
}

type mattermostTokenResponse struct {
	Token string `json:"token"`
}

func (service *Service) serveAttendancePage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/attendance" {
		http.Redirect(responseWriter, request, "/attendance/", http.StatusFound)
		return
	}
	if service.serveAttendanceStaticFile(responseWriter, request) {
		return
	}
	service.serveAttendanceIndex(responseWriter, request)
}

func (service *Service) serveAttendanceStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/attendance/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "attendance", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveAttendanceIndex(responseWriter http.ResponseWriter, request *http.Request) {
	attendanceIndexPath := filepath.Join(service.Configuration.AdminUIPath, "attendance", "index.html")
	if fileInformation, errorValue := os.Stat(attendanceIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, attendanceIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleAttendance(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeAttendanceRequest(request) {
		http.Error(responseWriter, "attendance access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/attendance/api")
	switch {
	case request.Method == http.MethodGet && path == "/summary":
		service.writeAttendanceSummary(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeAttendanceRequest(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	actorEmail := authenticatedCallerEmail(request)
	if actorEmail == "" {
		return false
	}
	return service.isFlowStaffActor(request.Context(), actorEmail)
}

func (service *Service) writeAttendanceSummary(responseWriter http.ResponseWriter, request *http.Request) {
	location, timeZoneName := service.workspaceTimeLocation()
	month := normalizeAttendanceMonth(request.URL.Query().Get("month"), time.Now().In(location))
	actorEmail := strings.ToLower(strings.TrimSpace(authenticatedCallerEmail(request)))
	isAdmin := service.isAuthorized(request)
	targetEmail := actorEmail
	if isAdmin {
		targetEmail = strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	}
	events, errorValue := service.readAttendanceEvents(request.Context(), month, targetEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	statusEvents := events
	if actorEmail != "" && targetEmail != actorEmail {
		if actorEvents, actorError := service.readAttendanceEvents(request.Context(), month, actorEmail); actorError == nil {
			statusEvents = actorEvents
		}
	}
	service.writeJSON(responseWriter, attendanceSummaryResponse{
		Month:            month,
		CurrentUserEmail: actorEmail,
		IsAdmin:          isAdmin,
		TimeZone:         timeZoneName,
		Events:           events,
		TodayStatus:      attendanceStatusForEvents(statusEvents, time.Now().In(location).Format("2006-01-02")),
	})
}

func (service *Service) handleAttendanceToggleAction(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload) {
	errorValue := service.toggleAttendanceFromMattermost(request.Context(), payload)
	if errorValue == nil || errors.Is(errorValue, errAttendanceDuplicateIgnored) {
		service.writeMattermostInteractiveSuccess(responseWriter)
		return
	}
	service.writeMattermostInteractiveError(responseWriter, errorValue.Error())
}

func (service *Service) toggleAttendanceFromMattermost(ctx context.Context, payload mattermostInteractivePayload) error {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	userRecord, found, errorValue := service.findMattermostUserByID(ctx, adminToken, payload.UserID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return fmt.Errorf("Mattermost user %s was not found", payload.UserID)
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return errorValue
	}
	channelID, errorValue := service.ensureMattermostAttendanceChannel(ctx, adminToken, teamRecord.ID)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := service.ensureMattermostChannelMembership(ctx, adminToken, channelID, userRecord.ID); errorValue != nil {
		return errorValue
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		return errorValue
	}
	return service.applyAttendanceToggle(ctx, userRecord, userToken, firstNonEmpty(payload.TeamID, teamRecord.ID), channelID, payload.PostID)
}

func (service *Service) applyAttendanceToggle(ctx context.Context, userRecord mattermostUserRecord, userToken string, teamID string, channelID string, actionPostID string) error {
	now := time.Now().UTC()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	lastEvent, found, errorValue := service.latestActiveAttendanceEvent(ctx, database, userRecord.ID)
	if errorValue != nil {
		return errorValue
	}
	if found && attendanceEventOccurredWithin(lastEvent, now, attendanceDuplicateWindow) {
		return service.handleRepeatedAttendanceClick(ctx, database, userToken, lastEvent, now)
	}
	return service.createNextAttendanceEvent(ctx, database, userRecord, userToken, lastEvent, found, teamID, channelID, actionPostID, now)
}

func (service *Service) handleRepeatedAttendanceClick(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, occurredAt time.Time) error {
	if strings.TrimSpace(event.RepeatedClickAt) == "" {
		return service.markAttendanceRepeatedClick(ctx, database, event.ID, occurredAt)
	}
	return service.cancelAttendanceEvent(ctx, database, userToken, event, occurredAt)
}

func (service *Service) createNextAttendanceEvent(ctx context.Context, database *sql.DB, userRecord mattermostUserRecord, userToken string, lastEvent attendanceEvent, hasLastEvent bool, teamID string, channelID string, actionPostID string, occurredAt time.Time) error {
	kind := nextAttendanceKind(lastEvent, hasLastEvent)
	resultPostID, errorValue := service.postMattermostUserAttendanceMessage(ctx, userToken, channelID, attendanceMessageForKind(kind))
	if errorValue != nil {
		return errorValue
	}
	event := service.createAttendanceEvent(userRecord, kind, occurredAt, teamID, channelID, actionPostID, resultPostID)
	return service.insertAttendanceEvent(ctx, database, event)
}

func (service *Service) cancelAttendanceEvent(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, canceledAt time.Time) error {
	message := attendanceMessageForKind(event.Kind) + " 취소"
	if _, errorValue := service.postMattermostUserAttendanceMessage(ctx, userToken, event.ChannelID, message); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx, `
UPDATE attendance_events
SET canceled_at = ?, cancel_reason = ?, repeated_click_at = ?
WHERE id = ?`,
		canceledAt.Format(time.RFC3339),
		attendanceCancelReason,
		canceledAt.Format(time.RFC3339),
		event.ID,
	)
	return errorValue
}

func (service *Service) createAttendanceEvent(userRecord mattermostUserRecord, kind string, occurredAt time.Time, teamID string, channelID string, actionPostID string, resultPostID string) attendanceEvent {
	location, timeZoneName := service.workspaceTimeLocation()
	localTime := occurredAt.In(location)
	return attendanceEvent{
		ID:                 randomHex(16),
		MattermostUserID:   strings.TrimSpace(userRecord.ID),
		MattermostUsername: strings.TrimSpace(userRecord.Username),
		Email:              strings.ToLower(strings.TrimSpace(userRecord.Email)),
		DisplayName:        mattermostDisplayName(userRecord),
		Kind:               kind,
		OccurredAt:         occurredAt.Format(time.RFC3339),
		LocalDate:          localTime.Format("2006-01-02"),
		LocalTime:          localTime.Format("15:04:05"),
		TimeZoneAtEvent:    timeZoneName,
		Source:             attendanceSourceMattermostButton,
		TeamID:             strings.TrimSpace(teamID),
		ChannelID:          strings.TrimSpace(channelID),
		ActionPostID:       strings.TrimSpace(actionPostID),
		ResultPostID:       strings.TrimSpace(resultPostID),
	}
}

func (service *Service) openAttendanceDatabase(ctx context.Context) (*sql.DB, error) {
	if errorValue := os.MkdirAll(filepath.Dir(service.Configuration.AttendanceDatabasePath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	database, errorValue := sql.Open("sqlite", service.Configuration.AttendanceDatabasePath)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := ensureAttendanceSchema(ctx, database); errorValue != nil {
		_ = database.Close()
		return nil, errorValue
	}
	return database, nil
}

func ensureAttendanceSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS attendance_events (
	id TEXT PRIMARY KEY,
	mattermost_user_id TEXT NOT NULL,
	mattermost_username TEXT NOT NULL,
	email TEXT NOT NULL,
	display_name TEXT NOT NULL,
	kind TEXT NOT NULL,
	occurred_at TEXT NOT NULL,
	local_date TEXT NOT NULL,
	local_time TEXT NOT NULL,
	time_zone_at_event TEXT NOT NULL,
	source TEXT NOT NULL,
	team_id TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	action_post_id TEXT NOT NULL,
	result_post_id TEXT NOT NULL,
	canceled_at TEXT NOT NULL,
	cancel_reason TEXT NOT NULL,
	repeated_click_at TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_events_user_date ON attendance_events(email, local_date)")
	return errorValue
}

func (service *Service) insertAttendanceEvent(ctx context.Context, database *sql.DB, event attendanceEvent) error {
	_, errorValue := database.ExecContext(ctx, `
INSERT INTO attendance_events (
	id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, canceled_at, cancel_reason, repeated_click_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		event.MattermostUserID,
		event.MattermostUsername,
		event.Email,
		event.DisplayName,
		event.Kind,
		event.OccurredAt,
		event.LocalDate,
		event.LocalTime,
		event.TimeZoneAtEvent,
		event.Source,
		event.TeamID,
		event.ChannelID,
		event.ActionPostID,
		event.ResultPostID,
		"",
		"",
		"",
	)
	return errorValue
}

func (service *Service) latestActiveAttendanceEvent(ctx context.Context, database *sql.DB, mattermostUserID string) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE mattermost_user_id = ? AND canceled_at = ''
ORDER BY occurred_at DESC
LIMIT 1`, mattermostUserID)
	event, errorValue := scanAttendanceEvent(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func (service *Service) markAttendanceRepeatedClick(ctx context.Context, database *sql.DB, eventID string, occurredAt time.Time) error {
	_, errorValue := database.ExecContext(ctx, "UPDATE attendance_events SET repeated_click_at = ? WHERE id = ?", occurredAt.Format(time.RFC3339), eventID)
	if errorValue != nil {
		return errorValue
	}
	return errAttendanceDuplicateIgnored
}

func (service *Service) readAttendanceEvents(ctx context.Context, month string, email string) ([]attendanceEvent, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	startDate := month + "-01"
	endDate := attendanceNextMonth(month) + "-01"
	query := `SELECT ` + attendanceEventSelectColumns + `
FROM attendance_events
WHERE local_date >= ? AND local_date < ?`
	arguments := []any{startDate, endDate}
	if strings.TrimSpace(email) != "" {
		query += " AND email = ?"
		arguments = append(arguments, strings.ToLower(strings.TrimSpace(email)))
	}
	query += " ORDER BY occurred_at DESC"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	events := []attendanceEvent{}
	for rows.Next() {
		event, errorValue := scanAttendanceEvent(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

type attendanceEventScanner interface {
	Scan(dest ...any) error
}

func scanAttendanceEvent(scanner attendanceEventScanner) (attendanceEvent, error) {
	var event attendanceEvent
	errorValue := scanner.Scan(
		&event.ID,
		&event.MattermostUserID,
		&event.MattermostUsername,
		&event.Email,
		&event.DisplayName,
		&event.Kind,
		&event.OccurredAt,
		&event.LocalDate,
		&event.LocalTime,
		&event.TimeZoneAtEvent,
		&event.Source,
		&event.TeamID,
		&event.ChannelID,
		&event.ActionPostID,
		&event.ResultPostID,
		&event.CanceledAt,
		&event.CancelReason,
		&event.RepeatedClickAt,
	)
	return event, errorValue
}

func (service *Service) ensureMattermostAttendanceChannel(ctx context.Context, token string, teamID string) (string, error) {
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, attendanceChannelName, attendanceChannelDisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.updateMattermostAttendanceChannelText(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.ensureMattermostAttendanceEntryPost(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	return channelID, nil
}

func (service *Service) updateMattermostAttendanceChannelText(ctx context.Context, token string, channelID string) error {
	link := service.mattermostAttendanceLink()
	body := map[string]string{
		"display_name": attendanceChannelDisplayName,
		"header":       link,
		"purpose":      link,
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", token, body, nil)
}

func (service *Service) ensureMattermostAttendanceEntryPost(ctx context.Context, token string, channelID string) error {
	expectedProps := service.mattermostAttendanceEntryPostProps()
	postRecord, found := service.mattermostAttendanceEntryPost(ctx, token, channelID)
	body := map[string]any{
		"message": attendanceEntryPostMessage,
		"props":   expectedProps,
	}
	if found {
		return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/posts/"+url.PathEscape(postRecord.ID)+"/patch", token, body, nil)
	}
	body["channel_id"] = channelID
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", token, body, nil)
}

func (service *Service) mattermostAttendanceEntryPost(ctx context.Context, token string, channelID string) (mattermostPostRecord, bool) {
	var response mattermostPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?per_page=50"
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &response); errorValue != nil {
		return mattermostPostRecord{}, false
	}
	for _, postID := range response.Order {
		postRecord := response.Posts[postID]
		if postRecord.Props[attendanceEntryPostProperty] == true {
			return postRecord, true
		}
	}
	return mattermostPostRecord{}, false
}

func (service *Service) mattermostAttendanceEntryPostProps() map[string]any {
	return map[string]any{
		attendanceEntryPostProperty: true,
		"attachments": []mattermostAttachment{
			{
				Fallback: attendanceEntryPostMessage,
				Text:     "버튼을 누르면 현재 상태에 따라 출근 또는 퇴근이 기록됩니다.",
				Actions: []mattermostAction{
					service.mattermostInteractiveButton(attendanceToggleAction, attendanceEntryPostMessage, "출근 또는 퇴근을 기록합니다.", "primary"),
				},
			},
		},
	}
}

func (service *Service) mattermostAttendanceLink() string {
	baseURL := strings.TrimRight(strings.TrimSpace(service.mattermostFlowBaseURL()), "/")
	if baseURL == "" {
		return "[출결 열기](/attendance/)"
	}
	return "[출결 열기](" + baseURL + "/attendance/)"
}

func (service *Service) ensureMattermostChannelMembership(ctx context.Context, token string, channelID string, userID string) error {
	body := map[string]string{"user_id": userID}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", token, body, nil)
	if errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	return nil
}

func (service *Service) postMattermostUserAttendanceMessage(ctx context.Context, userToken string, channelID string, message string) (string, error) {
	body := map[string]string{
		"channel_id": channelID,
		"message":    message,
	}
	var response struct {
		ID string `json:"id"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", userToken, body, &response)
	return strings.TrimSpace(response.ID), errorValue
}

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

func attendanceEventOccurredWithin(event attendanceEvent, now time.Time, window time.Duration) bool {
	occurredAt, errorValue := time.Parse(time.RFC3339, event.OccurredAt)
	return errorValue == nil && now.Sub(occurredAt) >= 0 && now.Sub(occurredAt) <= window
}

func attendanceMessageForKind(kind string) string {
	if kind == attendanceKindClockOut {
		return "퇴근"
	}
	return "출근"
}

func nextAttendanceKind(event attendanceEvent, hasEvent bool) string {
	if hasEvent && event.Kind == attendanceKindClockIn {
		return attendanceKindClockOut
	}
	return attendanceKindClockIn
}

func attendanceStatusForEvents(events []attendanceEvent, localDate string) string {
	for _, event := range events {
		if event.LocalDate != localDate || event.CanceledAt != "" {
			continue
		}
		if event.Kind == attendanceKindClockIn {
			return "clocked_in"
		}
		if event.Kind == attendanceKindClockOut {
			return "clocked_out"
		}
	}
	return "not_clocked_in"
}

func normalizeAttendanceMonth(value string, fallback time.Time) string {
	trimmedValue := strings.TrimSpace(value)
	if _, errorValue := time.Parse("2006-01", trimmedValue); errorValue == nil {
		return trimmedValue
	}
	return fallback.Format("2006-01")
}

func attendanceNextMonth(month string) string {
	parsedTime, errorValue := time.Parse("2006-01", month)
	if errorValue != nil {
		return month
	}
	return parsedTime.AddDate(0, 1, 0).Format("2006-01")
}
