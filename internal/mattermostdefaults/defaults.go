package mattermostdefaults

import "strings"

const (
	AdminResourcePath      = "admin"
	AttendanceResourcePath = "attendance"
	CalendarResourcePath   = "calendar"
	FlowResourcePath       = "flow"
	MailResourcePath       = "mail"

	TownSquareChannelName = "town-square"

	FlowChannelName        = "flow"
	FlowChannelDisplayName = "업무"

	CalendarChannelName        = "calendar"
	CalendarChannelDisplayName = "Calendar"

	AttendanceChannelName        = "attendance"
	AttendanceChannelDisplayName = "Attendance"
)

type PublicChannel struct {
	Name        string
	DisplayName string
	Header      string
	Purpose     string
}

func ManagedResourcePaths() []string {
	return []string{
		AdminResourcePath,
		AttendanceResourcePath,
		CalendarResourcePath,
		FlowResourcePath,
		MailResourcePath,
	}
}

func ManagedResourcePathSetting() string {
	return strings.Join(ManagedResourcePaths(), ",")
}

func DefaultPublicChannels() []PublicChannel {
	return []PublicChannel{
		{Name: FlowChannelName, DisplayName: FlowChannelDisplayName, Header: "[Flow 열기](/flow/)"},
		{Name: CalendarChannelName, DisplayName: CalendarChannelDisplayName, Header: "[Calendar 열기](/calendar/)"},
		{Name: AttendanceChannelName, DisplayName: AttendanceChannelDisplayName, Header: "[출결 열기](/attendance/)", Purpose: "[출결 열기](/attendance/)"},
	}
}
