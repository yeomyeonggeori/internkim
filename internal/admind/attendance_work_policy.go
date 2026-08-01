package admind

import "time"

const (
	attendanceWorkPolicyVersion              = 1
	attendanceWorkPolicyInitialEffectiveDate = "1970-01-01"
	attendanceWorkModeAutonomous             = "autonomous"
)

type attendanceWorkPolicy struct {
	Version   int                            `json:"version"`
	UpdatedAt string                         `json:"updatedAt"`
	Revisions []attendanceWorkPolicyRevision `json:"revisions"`
}

type attendanceWorkPolicyRevision struct {
	EffectiveDate       string                              `json:"effectiveDate"`
	WorkMode            string                              `json:"workMode"`
	WorkingWeekdays     []int                               `json:"workingWeekdays"`
	DailyTargetMinutes  int                                 `json:"dailyTargetMinutes"`
	WeeklyTargetMinutes int                                 `json:"weeklyTargetMinutes"`
	ReferenceStartTime  string                              `json:"referenceStartTime"`
	FixedStartTime      string                              `json:"fixedStartTime"`
	FixedEndTime        string                              `json:"fixedEndTime"`
	CoreTimeEnabled     bool                                `json:"coreTimeEnabled"`
	CoreStartTime       string                              `json:"coreStartTime"`
	CoreEndTime         string                              `json:"coreEndTime"`
	BreakPeriods        []attendanceWorkScheduleBreakPeriod `json:"breakPeriods"`
	NightStartTime      string                              `json:"nightStartTime"`
	NightEndTime        string                              `json:"nightEndTime"`
}

func defaultAttendanceWorkPolicy() attendanceWorkPolicy {
	return attendanceWorkPolicy{
		Version:   attendanceWorkPolicyVersion,
		UpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		Revisions: []attendanceWorkPolicyRevision{defaultAttendanceWorkPolicyRevision()},
	}
}

func defaultAttendanceWorkPolicyRevision() attendanceWorkPolicyRevision {
	return attendanceWorkPolicyRevision{
		EffectiveDate:       attendanceWorkPolicyInitialEffectiveDate,
		WorkMode:            attendanceWorkModeFlexible,
		WorkingWeekdays:     []int{1, 2, 3, 4, 5},
		DailyTargetMinutes:  8 * 60,
		WeeklyTargetMinutes: 40 * 60,
		ReferenceStartTime:  "09:00",
		FixedStartTime:      "",
		FixedEndTime:        "",
		CoreTimeEnabled:     true,
		CoreStartTime:       "11:00",
		CoreEndTime:         "16:00",
		BreakPeriods: []attendanceWorkScheduleBreakPeriod{{
			StartTime: "12:00",
			EndTime:   "13:00",
		}},
		NightStartTime: "22:00",
		NightEndTime:   "06:00",
	}
}
