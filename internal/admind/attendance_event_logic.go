package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type attendanceActionResult struct {
	Status       string
	ResultPostID string
}

const (
	attendanceActionStatusCreated = "created"
	attendanceActionStatusIgnored = "ignored"
	attendanceActionStatusUpdated = "updated"
)

func createdAttendanceActionResult(resultPostID string) attendanceActionResult {
	return attendanceActionResult{Status: attendanceActionStatusCreated, ResultPostID: strings.TrimSpace(resultPostID)}
}

func ignoredAttendanceActionResult() attendanceActionResult {
	return attendanceActionResult{Status: attendanceActionStatusIgnored}
}

func updatedAttendanceActionResult(resultPostID string) attendanceActionResult {
	return attendanceActionResult{Status: attendanceActionStatusUpdated, ResultPostID: strings.TrimSpace(resultPostID)}
}

func (result attendanceActionResult) isCreated() bool {
	return result.Status == attendanceActionStatusCreated
}

func (result attendanceActionResult) isIgnored() bool {
	return result.Status == attendanceActionStatusIgnored
}

func (service *Service) applyAttendanceToggle(ctx context.Context, userRecord mattermostUserRecord, userToken string, teamID string, channelID string, actionPostID string) (attendanceActionResult, error) {
	now := time.Now().UTC()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	defer database.Close()
	lastEvent, found, errorValue := service.latestActiveAttendanceEventForToday(ctx, database, userRecord.ID, now)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if found && attendanceEventOccurredWithin(lastEvent, now, attendanceDuplicateWindow) {
		return service.handleRepeatedAttendanceClick(ctx, database, userToken, lastEvent, now)
	}
	return service.createNextAttendanceEvent(ctx, database, userRecord, userToken, lastEvent, found, teamID, channelID, actionPostID, now)
}

func (service *Service) applyAttendanceAction(ctx context.Context, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, locationID string) (attendanceActionResult, error) {
	now := time.Now().UTC()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	defer database.Close()
	lastEvent, found, errorValue := service.latestActiveAttendanceEventForToday(ctx, database, userRecord.ID, now)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	shouldIgnore, errorValue := service.shouldIgnoreMattermostAttendanceAction(ctx, kind, lastEvent, found, channelID, actionPostID)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if shouldIgnore {
		return ignoredAttendanceActionResult(), nil
	}
	eventLocation := attendanceLocation{}
	if kind == attendanceKindClockIn {
		eventLocation = service.defaultAttendanceLocation()
		if strings.TrimSpace(locationID) != "" {
			eventLocation = service.attendanceLocationByID(locationID)
		}
	}
	return service.createAttendanceEventForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, now, eventLocation)
}

func (service *Service) latestActiveAttendanceEventForToday(ctx context.Context, database *sql.DB, mattermostUserID string, now time.Time) (attendanceEvent, bool, error) {
	location, _ := service.workspaceTimeLocation()
	localDate := now.In(location).Format("2006-01-02")
	return service.latestActiveAttendanceEventForLocalDateAtOrBefore(ctx, database, mattermostUserID, localDate, now)
}

func (service *Service) shouldIgnoreMattermostAttendanceAction(ctx context.Context, kind string, event attendanceEvent, hasEvent bool, channelID string, actionPostID string) (bool, error) {
	if !shouldIgnoreAttendanceAction(kind, event, hasEvent) {
		return false, nil
	}
	if kind != attendanceKindClockIn || !hasEvent {
		return true, nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	postExists, errorValue := service.mattermostAttendanceResultPostExists(ctx, adminToken, event.ResultPostID, channelID, actionPostID)
	if errorValue != nil {
		return false, errorValue
	}
	return postExists, nil
}

func (service *Service) handleRepeatedAttendanceClick(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, occurredAt time.Time) (attendanceActionResult, error) {
	if strings.TrimSpace(event.RepeatedClickAt) == "" {
		return ignoredAttendanceActionResult(), service.markAttendanceRepeatedClick(ctx, database, event.ID, occurredAt)
	}
	return updatedAttendanceActionResult(event.ResultPostID), service.cancelAttendanceEvent(ctx, database, userToken, event, occurredAt)
}

func (service *Service) createNextAttendanceEvent(ctx context.Context, database *sql.DB, userRecord mattermostUserRecord, userToken string, lastEvent attendanceEvent, hasLastEvent bool, teamID string, channelID string, actionPostID string, occurredAt time.Time) (attendanceActionResult, error) {
	kind := nextAttendanceKind(lastEvent, hasLastEvent)
	eventLocation := attendanceLocation{}
	if kind == attendanceKindClockIn {
		eventLocation = service.defaultAttendanceLocation()
	}
	return service.createAttendanceEventForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, occurredAt, eventLocation)
}

func (service *Service) createAttendanceEventForKind(ctx context.Context, database *sql.DB, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, occurredAt time.Time, eventLocation attendanceLocation) (attendanceActionResult, error) {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if errorValue := service.deleteLatestAttendanceResultPostForUserAndKind(ctx, database, adminToken, userRecord.ID, kind); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	resultPostID, errorValue := service.postMattermostUserAttendanceMessage(ctx, userToken, channelID, actionPostID, service.attendanceMessageForKindAndLocation(kind, eventLocation))
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	event := service.createAttendanceEvent(userRecord, kind, occurredAt, teamID, channelID, actionPostID, resultPostID, eventLocation)
	return createdAttendanceActionResult(resultPostID), service.insertAttendanceEvent(ctx, database, event)
}

func (service *Service) deleteLatestAttendanceResultPostForUserAndKind(ctx context.Context, database *sql.DB, adminToken string, mattermostUserID string, kind string) error {
	event, found, errorValue := service.latestActiveAttendanceEventForUserAndKind(ctx, database, mattermostUserID, kind)
	if errorValue != nil || !found {
		return errorValue
	}
	return service.deleteMattermostAttendanceResultPost(ctx, adminToken, event.ResultPostID)
}

func (service *Service) cancelAttendanceEvent(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, canceledAt time.Time) error {
	message := service.attendanceCancelMessage(event.Kind, attendanceLocation{ID: event.LocationID, Name: event.LocationName})
	if _, errorValue := service.postMattermostUserAttendanceMessage(ctx, userToken, event.ChannelID, event.ActionPostID, message); errorValue != nil {
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

func (service *Service) createAttendanceEvent(userRecord mattermostUserRecord, kind string, occurredAt time.Time, teamID string, channelID string, actionPostID string, resultPostID string, eventLocation attendanceLocation) attendanceEvent {
	location, timeZoneName := service.workspaceTimeLocation()
	localTime := occurredAt.In(location)
	return attendanceEvent{
		ID:                 randomHex(16),
		MattermostUserID:   strings.TrimSpace(userRecord.ID),
		MattermostUsername: strings.TrimSpace(userRecord.Username),
		Email:              strings.ToLower(strings.TrimSpace(userRecord.Email)),
		DisplayName:        mattermostDisplayName(userRecord),
		Kind:               kind,
		OccurredAt:         occurredAt.Format(time.RFC3339Nano),
		LocalDate:          localTime.Format("2006-01-02"),
		LocalTime:          localTime.Format("15:04:05"),
		TimeZoneAtEvent:    timeZoneName,
		Source:             attendanceSourceMattermostButton,
		TeamID:             strings.TrimSpace(teamID),
		ChannelID:          strings.TrimSpace(channelID),
		ActionPostID:       strings.TrimSpace(actionPostID),
		ResultPostID:       strings.TrimSpace(resultPostID),
		LocationID:         strings.TrimSpace(eventLocation.ID),
		LocationName:       strings.TrimSpace(eventLocation.Name),
	}
}

func (service *Service) attendanceMessageForKindAndLocation(kind string, location attendanceLocation) string {
	message := attendanceMessageForKind(service.adminText(), kind)
	locationName := strings.TrimSpace(service.attendanceDisplayLocationName(location))
	if kind != attendanceKindClockIn || locationName == "" {
		return message
	}
	return message + "(" + locationName + ")"
}

func (service *Service) attendanceCancelMessage(kind string, location attendanceLocation) string {
	return service.attendanceMessageForKindAndLocation(kind, location) + " " + service.adminText().AttendanceCanceled
}

func (service *Service) attendanceMessageForEvent(event attendanceEvent, includeTime bool) string {
	message := service.attendanceMessageForKindAndLocation(event.Kind, attendanceLocation{ID: event.LocationID, Name: event.LocationName})
	if !includeTime || len(event.LocalTime) < 5 {
		return message
	}
	return message + " " + event.LocalTime[:5]
}

func (service *Service) attendanceDisplayLocationName(location attendanceLocation) string {
	locationName := strings.TrimSpace(location.Name)
	if service.adminLocale() == "en" && location.ID == "office" && locationName == "사무실" {
		return "Office"
	}
	return locationName
}
