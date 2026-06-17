package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var attendanceTimeMessagePattern = regexp.MustCompile(`^([01]?\d|2[0-3]):([0-5]\d)$`)

type mattermostPostCreatePayload struct {
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"`
	Message   string `json:"message"`
}

type attendancePostCommand struct {
	Kind         string
	LocationID   string
	TimeText     string
	IsTimeUpdate bool
}

type attendancePostCommandResult struct {
	ActionResult            attendanceActionResult
	ShouldDeleteCommandPost bool
}

func (service *Service) mattermostAttendancePostCreateCommand(request *http.Request) (mattermostPostCreatePayload, attendancePostCommand, bool, error) {
	payload, ok := mattermostPostCreatePayloadFromRequest(request)
	if !ok {
		return mattermostPostCreatePayload{}, attendancePostCommand{}, false, nil
	}
	if strings.TrimSpace(payload.ChannelID) != strings.TrimSpace(readTrimmedFile(service.mattermostAttendanceChannelIDPath())) {
		return mattermostPostCreatePayload{}, attendancePostCommand{}, false, nil
	}
	if strings.TrimSpace(payload.RootID) != "" && !service.isMattermostAttendanceEntryPostID(payload.RootID) {
		return mattermostPostCreatePayload{}, attendancePostCommand{}, false, nil
	}
	command, isCommand, errorValue := service.parseAttendancePostCommand(payload.Message)
	if !isCommand {
		return mattermostPostCreatePayload{}, attendancePostCommand{}, false, errorValue
	}
	return payload, command, true, errorValue
}

func mattermostPostCreatePayloadFromRequest(request *http.Request) (mattermostPostCreatePayload, bool) {
	if request.Method != http.MethodPost || request.URL.Path != "/api/v4/posts" {
		return mattermostPostCreatePayload{}, false
	}
	document, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		return mattermostPostCreatePayload{}, false
	}
	request.Body = io.NopCloser(bytes.NewReader(document))
	var payload mattermostPostCreatePayload
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
		return mattermostPostCreatePayload{}, false
	}
	return payload, true
}

func (service *Service) syncMattermostAttendancePostCommand(ctx context.Context, request *http.Request, payload mattermostPostCreatePayload, command attendancePostCommand, responseBody []byte) error {
	commandPost := mattermostCreatedPost(responseBody)
	commandPostID := strings.TrimSpace(commandPost.ID)
	if commandPostID == "" {
		return fmt.Errorf("Mattermost attendance command post ID was not found")
	}
	commandPostCreatedAt := mattermostPostCreatedAt(commandPost, time.Now().UTC())
	result, errorValue := service.applyMattermostAttendancePostCommand(ctx, request, payload, command, commandPostID, commandPostCreatedAt)
	if errorValue != nil {
		return errorValue
	}
	if !result.ShouldDeleteCommandPost {
		return nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	return service.deleteMattermostPost(ctx, adminToken, commandPostID)
}

func mattermostCreatedPostID(responseBody []byte) string {
	return strings.TrimSpace(mattermostCreatedPost(responseBody).ID)
}

func mattermostCreatedPost(responseBody []byte) mattermostPostRecord {
	var postRecord mattermostPostRecord
	if errorValue := json.Unmarshal(responseBody, &postRecord); errorValue != nil {
		return mattermostPostRecord{}
	}
	return postRecord
}

func mattermostPostCreatedAt(postRecord mattermostPostRecord, fallback time.Time) time.Time {
	if postRecord.CreateAt <= 0 {
		return fallback
	}
	return time.UnixMilli(postRecord.CreateAt).UTC()
}

func (service *Service) applyMattermostAttendancePostCommand(ctx context.Context, request *http.Request, payload mattermostPostCreatePayload, command attendancePostCommand, commandPostID string, commandPostCreatedAt time.Time) (attendancePostCommandResult, error) {
	userRecord, found := service.mattermostPostCreateUser(request)
	if !found {
		return attendancePostCommandResult{}, fmt.Errorf("Mattermost user session was not found")
	}
	if command.IsTimeUpdate {
		resultPostID, errorValue := service.updateAttendanceTimeFromMattermostPost(ctx, userRecord, command.TimeText, commandPostCreatedAt)
		return attendancePostCommandResult{
			ActionResult:            updatedAttendanceActionResult(resultPostID),
			ShouldDeleteCommandPost: true,
		}, errorValue
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return attendancePostCommandResult{}, errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return attendancePostCommandResult{}, errorValue
	}
	channelID, errorValue := service.mattermostAttendanceActionChannelID(ctx, adminToken, teamRecord.ID, payload.ChannelID)
	if errorValue != nil {
		return attendancePostCommandResult{}, errorValue
	}
	if errorValue := service.ensureMattermostChannelMembership(ctx, adminToken, channelID, userRecord.ID); errorValue != nil {
		return attendancePostCommandResult{}, errorValue
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		return attendancePostCommandResult{}, errorValue
	}
	actionPostID := firstNonEmpty(payload.RootID, readTrimmedFile(service.mattermostAttendanceEntryPostIDPath()))
	if strings.TrimSpace(payload.RootID) != "" {
		actionResult, errorValue := service.applyMattermostAttendanceReplyPostCommand(ctx, userRecord, adminToken, userToken, command, teamRecord.ID, channelID, actionPostID, commandPostID)
		return attendancePostCommandResult{ActionResult: actionResult, ShouldDeleteCommandPost: actionResult.isIgnored()}, errorValue
	}
	actionResult, errorValue := service.applyAttendanceAction(ctx, userRecord, userToken, command.Kind, teamRecord.ID, channelID, actionPostID, command.LocationID)
	return attendancePostCommandResult{ActionResult: actionResult, ShouldDeleteCommandPost: true}, errorValue
}

func (service *Service) applyMattermostAttendanceReplyPostCommand(ctx context.Context, userRecord mattermostUserRecord, adminToken string, userToken string, command attendancePostCommand, teamID string, channelID string, actionPostID string, commandPostID string) (attendanceActionResult, error) {
	now := time.Now().UTC()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	defer database.Close()
	lastEvent, found, errorValue := service.latestAttendanceActionEvent(ctx, database, userRecord.ID, command.Kind, now)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	eventLocation := attendanceLocation{}
	if command.Kind == attendanceKindClockIn {
		eventLocation = service.defaultAttendanceLocation()
		if strings.TrimSpace(command.LocationID) != "" {
			eventLocation = service.attendanceLocationByID(command.LocationID)
		}
	}
	shouldIgnore, errorValue := service.shouldIgnoreMattermostAttendanceAction(ctx, command.Kind, eventLocation, lastEvent, found, channelID, actionPostID)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if shouldIgnore {
		return ignoredAttendanceActionResult(), nil
	}
	if errorValue := service.deleteLatestAttendanceResultPostForUserAndKind(ctx, database, adminToken, userRecord.ID, command.Kind); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	message := service.attendanceMessageForKindAndLocation(command.Kind, eventLocation)
	if errorValue := service.patchMattermostAttendanceResultPost(ctx, userToken, commandPostID, message); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	event := service.createAttendanceEvent(userRecord, command.Kind, now, teamID, channelID, actionPostID, commandPostID, eventLocation)
	return createdAttendanceActionResult(commandPostID), service.insertAttendanceEvent(ctx, database, event)
}

func (service *Service) mattermostPostCreateUser(request *http.Request) (mattermostUserRecord, bool) {
	token := strings.TrimPrefix(strings.TrimSpace(request.Header.Get("Authorization")), "Bearer ")
	if token != "" {
		var userRecord mattermostUserRecord
		if errorValue := service.mattermostRequest(request.Context(), http.MethodGet, "/api/v4/users/me", token, nil, &userRecord); errorValue == nil && strings.TrimSpace(userRecord.ID) != "" {
			return userRecord, true
		}
	}
	cookieHeader := mattermostSessionCookieHeader(request)
	if cookieHeader == "" {
		return mattermostUserRecord{}, false
	}
	return service.mattermostSessionUser(request, cookieHeader)
}

func (service *Service) parseAttendancePostCommand(message string) (attendancePostCommand, bool, error) {
	trimmedMessage := strings.TrimSpace(message)
	if matches := attendanceTimeMessagePattern.FindStringSubmatch(trimmedMessage); matches != nil {
		hour, _ := strconv.Atoi(matches[1])
		return attendancePostCommand{TimeText: fmt.Sprintf("%02d:%s", hour, matches[2]), IsTimeUpdate: true}, true, nil
	}
	if command, found, errorValue := service.parseAttendanceKindPostCommand(trimmedMessage); found || errorValue != nil {
		return command, found, errorValue
	}
	if command, found := service.parseAttendanceLocationPostCommand(trimmedMessage); found {
		return command, true, nil
	}
	return attendancePostCommand{}, false, nil
}

func (service *Service) parseAttendanceKindPostCommand(message string) (attendancePostCommand, bool, error) {
	kind, locationName, found := service.parseAttendanceKindMessage(message)
	if !found {
		return attendancePostCommand{}, false, nil
	}
	if kind == attendanceKindClockOut {
		return attendancePostCommand{Kind: kind}, true, nil
	}
	location := service.defaultAttendanceLocation()
	if strings.TrimSpace(locationName) != "" {
		var locationFound bool
		location, locationFound = service.attendanceLocationByName(locationName)
		if !locationFound {
			return attendancePostCommand{}, true, fmt.Errorf("attendance location %q was not found", strings.TrimSpace(locationName))
		}
	}
	return attendancePostCommand{Kind: kind, LocationID: location.ID}, true, nil
}

func (service *Service) parseAttendanceLocationPostCommand(message string) (attendancePostCommand, bool) {
	location, found := service.attendanceLocationByName(message)
	if !found {
		return attendancePostCommand{}, false
	}
	return attendancePostCommand{Kind: attendanceKindClockIn, LocationID: location.ID}, true
}

func (service *Service) parseAttendanceKindMessage(message string) (string, string, bool) {
	baseMessage, locationName := splitAttendanceLocationMessage(message)
	normalizedMessage := strings.ToLower(strings.TrimSpace(baseMessage))
	switch normalizedMessage {
	case "출근", "clock in", "clock-in", "clockin":
		return attendanceKindClockIn, locationName, true
	case "퇴근", "clock out", "clock-out", "clockout":
		return attendanceKindClockOut, "", true
	default:
		return "", "", false
	}
}

func splitAttendanceLocationMessage(message string) (string, string) {
	trimmedMessage := strings.TrimSpace(message)
	openIndex := strings.LastIndex(trimmedMessage, "(")
	if openIndex < 0 || !strings.HasSuffix(trimmedMessage, ")") {
		return trimmedMessage, ""
	}
	return strings.TrimSpace(trimmedMessage[:openIndex]), strings.TrimSpace(strings.TrimSuffix(trimmedMessage[openIndex+1:], ")"))
}

func (service *Service) updateAttendanceTimeFromMattermostPost(ctx context.Context, userRecord mattermostUserRecord, timeText string, commandPostCreatedAt time.Time) (string, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	event, found, errorValue := service.latestActiveAttendanceEvent(ctx, database, userRecord.ID)
	if errorValue != nil {
		return "", errorValue
	}
	if !found {
		return "", fmt.Errorf("active attendance event was not found")
	}
	localTime, errorValue := service.attendanceLocalTimeForEvent(event, timeText, commandPostCreatedAt)
	if errorValue != nil {
		return "", errorValue
	}
	updatedEvent := event
	updatedEvent.OccurredAt = localTime.UTC().Format(time.RFC3339Nano)
	updatedEvent.LocalDate = localTime.Format("2006-01-02")
	updatedEvent.LocalTime = localTime.Format("15:04:05")
	updatedEvent.TimeZoneAtEvent = localTime.Location().String()
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.patchMattermostAttendanceResultPost(ctx, userToken, event.ResultPostID, service.attendanceMessageForEvent(updatedEvent, true)); errorValue != nil {
		return "", errorValue
	}
	return event.ResultPostID, service.updateAttendanceEventTime(ctx, database, event, localTime)
}

func (service *Service) attendanceLocalTimeForEvent(event attendanceEvent, timeText string, commandPostCreatedAt time.Time) (time.Time, error) {
	location, _ := service.workspaceTimeLocation()
	localTime, errorValue := time.ParseInLocation("2006-01-02 15:04", event.LocalDate+" "+strings.TrimSpace(timeText), location)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	referenceTime := commandPostCreatedAt.In(location)
	if localTime.After(referenceTime) {
		return localTime.AddDate(0, 0, -1), nil
	}
	return localTime, nil
}
