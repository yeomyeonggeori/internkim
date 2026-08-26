package admind

import (
	"strings"
	"time"
)

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
