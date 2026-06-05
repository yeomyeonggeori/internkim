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

func (service *Service) handleMattermostAttendancePostCreate(responseWriter http.ResponseWriter, request *http.Request) bool {
	payload, ok := mattermostPostCreatePayloadFromRequest(request)
	if !ok {
		return false
	}
	if strings.TrimSpace(payload.ChannelID) != strings.TrimSpace(readTrimmedFile(service.mattermostAttendanceChannelIDPath())) {
		return false
	}
	if !service.isMattermostAttendanceEntryPostID(payload.RootID) {
		return false
	}
	command, isCommand, errorValue := service.parseAttendancePostCommand(payload.Message)
	if !isCommand {
		return false
	}
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return true
	}
	postID, errorValue := service.applyMattermostAttendancePostCommand(request.Context(), request, payload, command)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return true
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(responseWriter).Encode(map[string]string{"id": postID})
	return true
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

func (service *Service) applyMattermostAttendancePostCommand(ctx context.Context, request *http.Request, payload mattermostPostCreatePayload, command attendancePostCommand) (string, error) {
	userRecord, found := service.mattermostPostCreateUser(request)
	if !found {
		return "", fmt.Errorf("Mattermost user session was not found")
	}
	if command.IsTimeUpdate {
		return service.updateAttendanceTimeFromMattermostPost(ctx, userRecord, command.TimeText)
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return "", errorValue
	}
	channelID, errorValue := service.mattermostAttendanceActionChannelID(ctx, adminToken, teamRecord.ID, payload.ChannelID)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.ensureMattermostChannelMembership(ctx, adminToken, channelID, userRecord.ID); errorValue != nil {
		return "", errorValue
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.applyAttendanceActionWithSource(ctx, userRecord, userToken, command.Kind, teamRecord.ID, channelID, payload.RootID, command.LocationID, attendanceSourceMattermostPost, payload.Message); errorValue != nil {
		return "", errorValue
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	event, found, errorValue := service.latestActiveAttendanceEventForUserAndKind(ctx, database, userRecord.ID, command.Kind)
	if errorValue != nil || !found {
		return "", errorValue
	}
	return event.ResultPostID, nil
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
	kind, locationName, found := service.parseAttendanceKindMessage(trimmedMessage)
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

func (service *Service) updateAttendanceTimeFromMattermostPost(ctx context.Context, userRecord mattermostUserRecord, timeText string) (string, error) {
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
	localTime, errorValue := service.attendanceLocalTimeForEvent(event, timeText)
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

func (service *Service) attendanceLocalTimeForEvent(event attendanceEvent, timeText string) (time.Time, error) {
	location, _ := service.workspaceTimeLocation()
	localTime, errorValue := time.ParseInLocation("2006-01-02 15:04", event.LocalDate+" "+strings.TrimSpace(timeText), location)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	return localTime, nil
}
