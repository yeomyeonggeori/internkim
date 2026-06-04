package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

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

func (service *Service) applyAttendanceAction(ctx context.Context, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, locationID string) error {
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
	if shouldIgnoreAttendanceAction(kind, lastEvent, found) {
		return nil
	}
	eventLocation := attendanceLocation{}
	if kind == attendanceKindClockIn && strings.TrimSpace(locationID) != "" {
		eventLocation = service.attendanceLocationByID(locationID)
	}
	return service.createAttendanceEventForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, now, eventLocation)
}

func (service *Service) handleRepeatedAttendanceClick(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, occurredAt time.Time) error {
	if strings.TrimSpace(event.RepeatedClickAt) == "" {
		return service.markAttendanceRepeatedClick(ctx, database, event.ID, occurredAt)
	}
	return service.cancelAttendanceEvent(ctx, database, userToken, event, occurredAt)
}

func (service *Service) createNextAttendanceEvent(ctx context.Context, database *sql.DB, userRecord mattermostUserRecord, userToken string, lastEvent attendanceEvent, hasLastEvent bool, teamID string, channelID string, actionPostID string, occurredAt time.Time) error {
	kind := nextAttendanceKind(lastEvent, hasLastEvent)
	eventLocation := attendanceLocation{}
	if kind == attendanceKindClockIn {
		eventLocation = service.defaultAttendanceLocation()
	}
	return service.createAttendanceEventForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, occurredAt, eventLocation)
}

func (service *Service) createAttendanceEventForKind(ctx context.Context, database *sql.DB, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, occurredAt time.Time, eventLocation attendanceLocation) error {
	resultPostID, errorValue := service.postMattermostUserAttendanceMessage(ctx, userToken, channelID, actionPostID, service.attendanceMessageForKindAndLocation(kind, eventLocation.Name))
	if errorValue != nil {
		return errorValue
	}
	event := service.createAttendanceEvent(userRecord, kind, occurredAt, teamID, channelID, actionPostID, resultPostID, eventLocation)
	return service.insertAttendanceEvent(ctx, database, event)
}

func (service *Service) cancelAttendanceEvent(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, canceledAt time.Time) error {
	message := service.attendanceCancelMessage(event.Kind, event.LocationName)
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

func (service *Service) attendanceMessageForKindAndLocation(kind string, locationName string) string {
	message := attendanceMessageForKind(service.adminText(), kind)
	trimmedLocationName := strings.TrimSpace(locationName)
	if kind != attendanceKindClockIn || trimmedLocationName == "" || !service.hasMultipleAttendanceLocations() {
		return message
	}
	return message + "(" + trimmedLocationName + ")"
}

func (service *Service) attendanceCancelMessage(kind string, locationName string) string {
	return service.attendanceMessageForKindAndLocation(kind, locationName) + " " + service.adminText().AttendanceCanceled
}

func (service *Service) hasMultipleAttendanceLocations() bool {
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		locations = defaultAttendanceLocations()
	}
	return len(locations) > 1
}
