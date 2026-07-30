package admind

import (
	"time"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

const (
	attendanceKindClockIn                        = "clock_in"
	attendanceKindClockOut                       = "clock_out"
	attendanceSourceMattermostButton             = "mattermost_button"
	attendanceChannelName                        = mattermostdefaults.AttendanceChannelName
	attendanceChannelDisplayName                 = mattermostdefaults.AttendanceChannelDisplayName
	attendanceClockInAction                      = "attendanceClockIn"
	attendanceClockOutAction                     = "attendanceClockOut"
	attendanceToggleAction                       = "attendance.toggle"
	attendanceEntryPostProperty                  = "internkim_attendance_entry"
	attendanceEntryPostFingerprintProperty       = "internkim_attendance_entry_fingerprint"
	attendanceEntryPostIDFilename                = "mattermost-attendance-entry-post-id"
	attendanceChannelIDFilename                  = "mattermost-attendance-channel-id"
	attendanceCancelReason                       = "repeated_click_confirmed"
	attendanceAccidentalShortSegmentCancelReason = "accidental_short_segment"
	attendanceSameLocationResumeCancelReason     = "same_location_resume"
	attendanceDuplicateWindow                    = 5 * time.Minute
	attendanceAccidentalSequenceWindow           = 30 * time.Second
	attendanceOvernightShiftWindow               = 12 * time.Hour
)

type attendanceEvent struct {
	ID                   string                    `json:"id"`
	MattermostUserID     string                    `json:"mattermostUserID"`
	MattermostUsername   string                    `json:"mattermostUsername"`
	Email                string                    `json:"email"`
	DisplayName          string                    `json:"displayName"`
	Kind                 string                    `json:"kind"`
	OccurredAt           string                    `json:"occurredAt"`
	LocalDate            string                    `json:"localDate"`
	LocalTime            string                    `json:"localTime"`
	TimeZoneAtEvent      string                    `json:"timeZoneAtEvent"`
	Source               string                    `json:"source"`
	TeamID               string                    `json:"teamID"`
	ChannelID            string                    `json:"channelID"`
	ActionPostID         string                    `json:"actionPostID"`
	ResultPostID         string                    `json:"resultPostID"`
	LocationID           string                    `json:"locationID,omitempty"`
	LocationName         string                    `json:"locationName,omitempty"`
	CanceledAt           string                    `json:"canceledAt,omitempty"`
	CancelReason         string                    `json:"cancelReason,omitempty"`
	RepeatedClickAt      string                    `json:"repeatedClickAt,omitempty"`
	OriginalOccurredAt   string                    `json:"originalOccurredAt,omitempty"`
	OriginalLocalDate    string                    `json:"originalLocalDate,omitempty"`
	OriginalLocalTime    string                    `json:"originalLocalTime,omitempty"`
	OriginalLocationID   string                    `json:"originalLocationID,omitempty"`
	OriginalLocationName string                    `json:"originalLocationName,omitempty"`
	OverrideReason       string                    `json:"overrideReason,omitempty"`
	OverriddenBy         string                    `json:"overriddenBy,omitempty"`
	OverriddenAt         string                    `json:"overriddenAt,omitempty"`
	OverrideHistory      []attendanceEventOverride `json:"overrideHistory,omitempty"`
}

type attendanceEventOverride struct {
	ID                   string `json:"id"`
	EventID              string `json:"eventID"`
	EditedBy             string `json:"editedBy"`
	EditedAt             string `json:"editedAt"`
	Reason               string `json:"reason"`
	OriginalOccurredAt   string `json:"originalOccurredAt"`
	OriginalLocalDate    string `json:"originalLocalDate"`
	OriginalLocalTime    string `json:"originalLocalTime"`
	OriginalLocationID   string `json:"originalLocationID"`
	OriginalLocationName string `json:"originalLocationName"`
	OverrideOccurredAt   string `json:"overrideOccurredAt"`
	OverrideLocalDate    string `json:"overrideLocalDate"`
	OverrideLocalTime    string `json:"overrideLocalTime"`
	OverrideLocationID   string `json:"overrideLocationID"`
	OverrideLocationName string `json:"overrideLocationName"`
}

type attendanceAbsence struct {
	ID           string `json:"id"`
	RangeID      string `json:"rangeID,omitempty"`
	Email        string `json:"email"`
	Kind         string `json:"kind"`
	LabelKey     string `json:"labelKey"`
	Date         string `json:"date"`
	StartDate    string `json:"startDate,omitempty"`
	EndDate      string `json:"endDate,omitempty"`
	StartTime    string `json:"startTime,omitempty"`
	EndTime      string `json:"endTime,omitempty"`
	Reason       string `json:"reason,omitempty"`
	CreatedBy    string `json:"createdBy,omitempty"`
	CreatedAt    string `json:"createdAt"`
	CanceledAt   string `json:"canceledAt,omitempty"`
	IsRangeStart bool   `json:"isRangeStart,omitempty"`
	IsRangeEnd   bool   `json:"isRangeEnd,omitempty"`
	IsChunkStart bool   `json:"isChunkStart,omitempty"`
	IsChunkEnd   bool   `json:"isChunkEnd,omitempty"`
}

type attendanceAbsenceRange struct {
	ID         string
	Email      string
	Kind       string
	StartDate  string
	EndDate    string
	Reason     string
	CreatedBy  string
	CreatedAt  string
	UpdatedAt  string
	CanceledAt string
	ReplacedBy string
}

type attendanceSummaryResponse struct {
	Month                 string                     `json:"month"`
	ServerTime            string                     `json:"serverTime"`
	CurrentUserEmail      string                     `json:"currentUserEmail"`
	IsAdmin               bool                       `json:"isAdmin"`
	TimeZone              string                     `json:"timeZone"`
	TimeZoneAuthoritative bool                       `json:"timeZoneAuthoritative"`
	Events                []attendanceEvent          `json:"events"`
	Absences              []attendanceAbsence        `json:"absences"`
	Members               []attendanceMember         `json:"members"`
	TodayStatus           string                     `json:"todayStatus"`
	ActiveLeave           *attendanceActiveLeaveView `json:"activeLeave,omitempty"`
	Locations             []attendanceLocation       `json:"locations"`
	TeamViewVisibleToAll  bool                       `json:"teamViewVisibleToAll"`
	TeamViewBlocked       bool                       `json:"teamViewBlocked"`
}

type attendanceMember struct {
	Email              string `json:"email"`
	DisplayName        string `json:"displayName"`
	Image              string `json:"image,omitempty"`
	MattermostUsername string `json:"mattermostUsername"`
	UserID             string `json:"-"`
	HireDate           string `json:"-"`
}

type attendanceAbsenceRequest struct {
	Email     string `json:"email"`
	Kind      string `json:"kind"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Reason    string `json:"reason"`
}

type attendanceEventOverrideRequest struct {
	LocalDate  string `json:"localDate"`
	LocalTime  string `json:"localTime"`
	LocationID string `json:"locationID"`
	Reason     string `json:"reason"`
}

type attendanceAbsencesResponse struct {
	Absences []attendanceAbsence `json:"absences"`
}

type attendanceSettingsRequest struct {
	TeamViewVisibleToAll *bool `json:"teamViewVisibleToAll"`
}

type attendanceUserTokenRecord struct {
	UserID    string `json:"userID"`
	Token     string `json:"token"`
	UpdatedAt string `json:"updatedAt"`
}

type mattermostTokenResponse struct {
	Token string `json:"token"`
}
