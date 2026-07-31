package admind

type attendancePaidLeaveOccurrence struct {
	Email              string
	Date               string
	StartTime          string
	EndTime            string
	DeductionMilliDays int
	Paid               bool
}

type attendanceWorkIntervalDetail struct {
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Provisional bool   `json:"provisional"`
}

type attendanceLeaveIntervalDetail struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Paid      bool   `json:"paid"`
}

type attendanceWorkDayStatus struct {
	Date                    string                          `json:"date"`
	WorkMode                string                          `json:"workMode"`
	HasBaseline             bool                            `json:"hasBaseline"`
	TargetMinutes           int                             `json:"targetMinutes"`
	ActualMinutes           int                             `json:"actualMinutes"`
	ProvisionalMinutes      int                             `json:"provisionalMinutes"`
	PaidLeaveMinutes        int                             `json:"paidLeaveMinutes"`
	CreditedLeaveMinutes    int                             `json:"creditedLeaveMinutes"`
	FulfilledMinutes        int                             `json:"fulfilledMinutes"`
	DifferenceMinutes       int                             `json:"differenceMinutes"`
	RemainingMinutes        int                             `json:"remainingMinutes"`
	OvertimeMinutes         int                             `json:"overtimeMinutes"`
	NightMinutes            int                             `json:"nightMinutes"`
	IsWorking               bool                            `json:"isWorking"`
	NeedsReview             bool                            `json:"needsReview"`
	CoreTimeMissed          bool                            `json:"coreTimeMissed"`
	Late                    bool                            `json:"late"`
	EarlyLeave              bool                            `json:"earlyLeave"`
	HasLeaveWorkOverlap     bool                            `json:"hasLeaveWorkOverlap"`
	HasIncompleteWorkRecord bool                            `json:"hasIncompleteWorkRecord"`
	Status                  string                          `json:"status"`
	WorkSegments            []attendanceWorkIntervalDetail  `json:"workSegments"`
	LeaveSegments           []attendanceLeaveIntervalDetail `json:"leaveSegments"`
}

type attendanceWorkStatus struct {
	Email                string                    `json:"email"`
	DisplayName          string                    `json:"displayName"`
	PeriodStart          string                    `json:"periodStart"`
	PeriodEnd            string                    `json:"periodEnd"`
	WorkMode             string                    `json:"workMode"`
	HasBaseline          bool                      `json:"hasBaseline"`
	TargetMinutes        int                       `json:"targetMinutes"`
	ActualMinutes        int                       `json:"actualMinutes"`
	ProvisionalMinutes   int                       `json:"provisionalMinutes"`
	PaidLeaveMinutes     int                       `json:"paidLeaveMinutes"`
	CreditedLeaveMinutes int                       `json:"creditedLeaveMinutes"`
	FulfilledMinutes     int                       `json:"fulfilledMinutes"`
	DifferenceMinutes    int                       `json:"differenceMinutes"`
	RemainingMinutes     int                       `json:"remainingMinutes"`
	OvertimeMinutes      int                       `json:"overtimeMinutes"`
	NightMinutes         int                       `json:"nightMinutes"`
	IsWorking            bool                      `json:"isWorking"`
	NeedsReview          bool                      `json:"needsReview"`
	CoreTimeMissed       bool                      `json:"coreTimeMissed"`
	Late                 bool                      `json:"late"`
	EarlyLeave           bool                      `json:"earlyLeave"`
	HasLeaveWorkOverlap  bool                      `json:"hasLeaveWorkOverlap"`
	HasIncompleteRecords bool                      `json:"hasIncompleteRecords"`
	Status               string                    `json:"status"`
	Days                 []attendanceWorkDayStatus `json:"days"`
}
