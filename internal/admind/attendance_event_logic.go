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
	lastEvent, found, errorValue := service.latestAttendanceActionEvent(ctx, database, userRecord.ID, "", now)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if found && attendanceEventOccurredWithin(lastEvent, now, attendanceDuplicateWindow) {
		return service.handleRepeatedAttendanceClick(ctx, database, userToken, lastEvent, now)
	}
	return service.createNextAttendanceEvent(ctx, database, userRecord, userToken, lastEvent, found, teamID, channelID, actionPostID, now)
}

func (service *Service) applyAttendanceAction(ctx context.Context, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, locationID string) (attendanceActionResult, error) {
	return service.applyAttendanceActionWithResultPost(ctx, userRecord, userToken, kind, teamID, channelID, actionPostID, locationID, "")
}

func (service *Service) applyAttendanceActionWithResultPost(ctx context.Context, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, locationID string, resultPostID string) (attendanceActionResult, error) {
	now := time.Now().UTC()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	defer database.Close()
	lastEvent, found, errorValue := service.latestAttendanceActionEvent(ctx, database, userRecord.ID, kind, now)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	eventLocation := attendanceLocation{}
	if kind == attendanceKindClockIn {
		eventLocation = service.defaultAttendanceLocation()
		if strings.TrimSpace(locationID) != "" {
			eventLocation = service.attendanceLocationByID(locationID)
		}
	}
	shouldIgnore, errorValue := service.shouldIgnoreMattermostAttendanceAction(ctx, kind, eventLocation, lastEvent, found, channelID, actionPostID)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if shouldIgnore {
		return ignoredAttendanceActionResult(), nil
	}
	accidentalResult, handled, errorValue := service.handleAccidentalAttendanceSequence(ctx, database, userRecord.ID, userToken, kind, eventLocation, lastEvent, found, now)
	if errorValue != nil || handled {
		return accidentalResult, errorValue
	}
	if strings.TrimSpace(resultPostID) != "" {
		return service.createAttendanceEventFromExistingResultPostForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, now, eventLocation, resultPostID)
	}
	return service.createAttendanceEventForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, now, eventLocation)
}

func (service *Service) latestActiveAttendanceEventForToday(ctx context.Context, database *sql.DB, mattermostUserID string, now time.Time) (attendanceEvent, bool, error) {
	location, _ := service.workspaceTimeLocation()
	localDate := now.In(location).Format("2006-01-02")
	return service.latestActiveAttendanceEventForLocalDateAtOrBefore(ctx, database, mattermostUserID, localDate, now)
}

func (service *Service) latestAttendanceActionEvent(ctx context.Context, database *sql.DB, mattermostUserID string, kind string, now time.Time) (attendanceEvent, bool, error) {
	event, found, errorValue := service.latestActiveAttendanceEventForToday(ctx, database, mattermostUserID, now)
	if errorValue != nil || found || kind == attendanceKindClockIn {
		return event, found, errorValue
	}
	return service.openClockInFromYesterday(ctx, database, mattermostUserID, now)
}

func (service *Service) openClockInFromYesterday(ctx context.Context, database *sql.DB, mattermostUserID string, now time.Time) (attendanceEvent, bool, error) {
	location, _ := service.workspaceTimeLocation()
	yesterdayDate := now.In(location).AddDate(0, 0, -1).Format("2006-01-02")
	yesterdayEvent, found, errorValue := service.latestActiveAttendanceEventForLocalDateAtOrBefore(ctx, database, mattermostUserID, yesterdayDate, now)
	if errorValue != nil || !found || yesterdayEvent.Kind != attendanceKindClockIn {
		return attendanceEvent{}, false, errorValue
	}
	return yesterdayEvent, true, nil
}

func (service *Service) shouldIgnoreMattermostAttendanceAction(ctx context.Context, kind string, location attendanceLocation, event attendanceEvent, hasEvent bool, channelID string, actionPostID string) (bool, error) {
	if !shouldIgnoreAttendanceAction(kind, event, hasEvent) {
		return false, nil
	}
	if kind == attendanceKindClockIn && hasEvent && !attendanceEventLocationMatches(event, location) {
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

func attendanceEventLocationMatches(event attendanceEvent, location attendanceLocation) bool {
	eventLocationID := strings.TrimSpace(event.LocationID)
	locationID := strings.TrimSpace(location.ID)
	if eventLocationID != "" || locationID != "" {
		return eventLocationID == locationID
	}
	return strings.TrimSpace(event.LocationName) == strings.TrimSpace(location.Name)
}

func (service *Service) handleRepeatedAttendanceClick(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, occurredAt time.Time) (attendanceActionResult, error) {
	if strings.TrimSpace(event.RepeatedClickAt) == "" {
		return ignoredAttendanceActionResult(), service.markAttendanceRepeatedClick(ctx, database, event.ID, occurredAt)
	}
	return updatedAttendanceActionResult(event.ResultPostID), service.cancelAttendanceEvent(ctx, database, userToken, event, occurredAt)
}

func (service *Service) handleAccidentalAttendanceSequence(ctx context.Context, database *sql.DB, mattermostUserID string, userToken string, kind string, eventLocation attendanceLocation, lastEvent attendanceEvent, hasLastEvent bool, occurredAt time.Time) (attendanceActionResult, bool, error) {
	if kind == attendanceKindClockOut && hasLastEvent && lastEvent.Kind == attendanceKindClockIn && attendanceEventOccurredWithin(lastEvent, occurredAt, attendanceAccidentalSequenceWindow) {
		errorValue := service.cancelAttendanceEventWithReason(ctx, database, userToken, lastEvent, occurredAt, attendanceAccidentalShortSegmentCancelReason, "")
		return updatedAttendanceActionResult(lastEvent.ResultPostID), true, errorValue
	}
	if kind != attendanceKindClockIn {
		return attendanceActionResult{}, false, nil
	}
	clockOutEvent, foundClockOut, errorValue := service.latestAccidentalClockOutEvent(ctx, database, mattermostUserID, lastEvent, hasLastEvent, occurredAt)
	if errorValue != nil || !foundClockOut {
		return attendanceActionResult{}, false, errorValue
	}
	clockOutOccurredAt, errorValue := parseAttendanceEventTime(clockOutEvent.OccurredAt)
	if errorValue != nil {
		return attendanceActionResult{}, false, errorValue
	}
	clockInEvent, foundClockIn, errorValue := service.latestActiveAttendanceEventBefore(ctx, database, mattermostUserID, attendanceKindClockIn, clockOutOccurredAt)
	if errorValue != nil || !foundClockIn || !attendanceEventLocationMatches(clockInEvent, eventLocation) {
		return attendanceActionResult{}, false, errorValue
	}
	errorValue = service.cancelAttendanceEventWithReason(ctx, database, userToken, clockOutEvent, occurredAt, attendanceSameLocationResumeCancelReason, "")
	return updatedAttendanceActionResult(clockOutEvent.ResultPostID), true, errorValue
}

func (service *Service) latestAccidentalClockOutEvent(ctx context.Context, database *sql.DB, mattermostUserID string, lastEvent attendanceEvent, hasLastEvent bool, occurredAt time.Time) (attendanceEvent, bool, error) {
	if hasLastEvent && lastEvent.Kind == attendanceKindClockOut && attendanceEventOccurredWithin(lastEvent, occurredAt, attendanceAccidentalSequenceWindow) {
		return lastEvent, true, nil
	}
	event, found, errorValue := service.latestActiveAttendanceEvent(ctx, database, mattermostUserID)
	if errorValue != nil || !found || event.Kind != attendanceKindClockOut || !attendanceEventOccurredWithin(event, occurredAt, attendanceAccidentalSequenceWindow) {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
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
	return service.insertAttendanceEventForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, occurredAt, eventLocation, resultPostID)
}

func (service *Service) createAttendanceEventFromExistingResultPostForKind(ctx context.Context, database *sql.DB, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, occurredAt time.Time, eventLocation attendanceLocation, resultPostID string) (attendanceActionResult, error) {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if errorValue := service.deleteLatestAttendanceResultPostForUserAndKind(ctx, database, adminToken, userRecord.ID, kind); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if errorValue := service.patchMattermostAttendanceResultPost(ctx, userToken, resultPostID, service.attendanceMessageForKindAndLocation(kind, eventLocation)); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	return service.insertAttendanceEventForKind(ctx, database, userRecord, userToken, kind, teamID, channelID, actionPostID, occurredAt, eventLocation, resultPostID)
}

func (service *Service) insertAttendanceEventForKind(ctx context.Context, database *sql.DB, userRecord mattermostUserRecord, userToken string, kind string, teamID string, channelID string, actionPostID string, occurredAt time.Time, eventLocation attendanceLocation, resultPostID string) (attendanceActionResult, error) {
	event := service.createAttendanceEvent(userRecord, kind, occurredAt, teamID, channelID, actionPostID, resultPostID, eventLocation)
	if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	service.syncMattermostCustomStatusToAttendance(ctx, userToken, kind, eventLocation)
	return createdAttendanceActionResult(resultPostID), nil
}

func (service *Service) deleteLatestAttendanceResultPostForUserAndKind(ctx context.Context, database *sql.DB, adminToken string, mattermostUserID string, kind string) error {
	event, found, errorValue := service.latestActiveAttendanceEventForUserAndKind(ctx, database, mattermostUserID, kind)
	if errorValue != nil || !found {
		return errorValue
	}
	return service.deleteMattermostPost(ctx, adminToken, event.ResultPostID)
}

func (service *Service) cancelAttendanceEvent(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, canceledAt time.Time) error {
	return service.cancelAttendanceEventWithReason(ctx, database, userToken, event, canceledAt, attendanceCancelReason, canceledAt.Format(time.RFC3339))
}

func (service *Service) cancelAttendanceEventWithReason(ctx context.Context, database *sql.DB, userToken string, event attendanceEvent, canceledAt time.Time, reason string, repeatedClickAt string) error {
	if errorValue := service.cancelAttendanceEventRecord(ctx, database, event, canceledAt, reason, repeatedClickAt); errorValue != nil {
		return errorValue
	}
	message := service.attendanceCancelMessage(event.Kind, attendanceLocation{ID: event.LocationID, Name: event.LocationName})
	_, errorValue := service.postMattermostUserAttendanceMessage(ctx, userToken, event.ChannelID, event.ActionPostID, message)
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
