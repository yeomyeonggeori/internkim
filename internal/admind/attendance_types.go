package admind

import (
	"time"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

const (
	attendanceKindClockIn            = "clock_in"
	attendanceKindClockOut           = "clock_out"
	attendanceSourceMattermostButton = "mattermost_button"
	attendanceSourceMattermostPost   = "mattermost_post"
	attendanceChannelName            = mattermostdefaults.AttendanceChannelName
	attendanceChannelDisplayName     = mattermostdefaults.AttendanceChannelDisplayName
	attendanceClockInAction          = "attendanceClockIn"
	attendanceClockOutAction         = "attendanceClockOut"
	attendanceToggleAction           = "attendance.toggle"
	attendanceEntryPostProperty      = "internkim_attendance_entry"
	attendanceEntryPostIDFilename    = "mattermost-attendance-entry-post-id"
	attendanceChannelIDFilename      = "mattermost-attendance-channel-id"
	attendanceCancelReason           = "repeated click confirmed"
	attendanceDuplicateWindow        = 5 * time.Minute
)

type attendanceEvent struct {
	ID                   string                          `json:"id"`
	MattermostUserID     string                          `json:"mattermostUserID"`
	MattermostUsername   string                          `json:"mattermostUsername"`
	Email                string                          `json:"email"`
	DisplayName          string                          `json:"displayName"`
	Kind                 string                          `json:"kind"`
	OccurredAt           string                          `json:"occurredAt"`
	LocalDate            string                          `json:"localDate"`
	LocalTime            string                          `json:"localTime"`
	TimeZoneAtEvent      string                          `json:"timeZoneAtEvent"`
	Source               string                          `json:"source"`
	TeamID               string                          `json:"teamID"`
	ChannelID            string                          `json:"channelID"`
	ActionPostID         string                          `json:"actionPostID"`
	ResultPostID         string                          `json:"resultPostID"`
	LocationID           string                          `json:"locationID,omitempty"`
	LocationName         string                          `json:"locationName,omitempty"`
	CanceledAt           string                          `json:"canceledAt,omitempty"`
	CancelReason         string                          `json:"cancelReason,omitempty"`
	RepeatedClickAt      string                          `json:"repeatedClickAt,omitempty"`
	SourceMessage        string                          `json:"sourceMessage,omitempty"`
	OriginalLocationID   string                          `json:"originalLocationID,omitempty"`
	OriginalLocationName string                          `json:"originalLocationName,omitempty"`
	OverriddenBy         string                          `json:"overriddenBy,omitempty"`
	OverriddenAt         string                          `json:"overriddenAt,omitempty"`
	ParsedAs             *attendanceParsedClassification `json:"parsedAs,omitempty"`
}

type attendanceParsedClassification struct {
	Kind       string `json:"kind,omitempty"`
	LocationID string `json:"locationID,omitempty"`
}

type attendanceSummaryResponse struct {
	Month                string               `json:"month"`
	CurrentUserEmail     string               `json:"currentUserEmail"`
	IsAdmin              bool                 `json:"isAdmin"`
	TimeZone             string               `json:"timeZone"`
	Events               []attendanceEvent    `json:"events"`
	TodayStatus          string               `json:"todayStatus"`
	Locations            []attendanceLocation `json:"locations"`
	TeamViewVisibleToAll bool                 `json:"teamViewVisibleToAll"`
	TeamViewBlocked      bool                 `json:"teamViewBlocked"`
}

type attendanceSettingsRequest struct {
	TeamViewVisibleToAll *bool `json:"teamViewVisibleToAll"`
}

type attendanceEventLocationRequest struct {
	LocationID string `json:"locationID"`
}

type attendanceUserTokenRecord struct {
	UserID    string `json:"userID"`
	Token     string `json:"token"`
	UpdatedAt string `json:"updatedAt"`
}

type mattermostTokenResponse struct {
	Token string `json:"token"`
}
