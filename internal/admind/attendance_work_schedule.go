package admind

const (
	attendanceWorkScheduleVersion        = 1
	attendanceWorkModeFixed              = "fixed"
	attendanceWorkModeFlexible           = "flexible"
	attendanceWorkScheduleFullDay        = "fullDay"
	attendanceWorkScheduleHalfDay        = "halfDay"
	attendanceWorkScheduleQuarterDay     = "quarterDay"
	attendanceWorkScheduleMinutesPerDay  = 24 * 60
	attendanceWorkScheduleMaximumMinutes = attendanceWorkScheduleMinutesPerDay - 1
)

type attendanceWorkSchedule struct {
	Version              int                                 `json:"version"`
	WorkMode             string                              `json:"workMode"`
	WorkingWeekdays      []int                               `json:"workingWeekdays"`
	ScheduledWorkMinutes int                                 `json:"scheduledWorkMinutes"`
	ReferenceStartTime   string                              `json:"referenceStartTime"`
	FixedStartTime       string                              `json:"fixedStartTime"`
	FixedEndTime         string                              `json:"fixedEndTime"`
	BreakPeriods         []attendanceWorkScheduleBreakPeriod `json:"breakPeriods"`
	UpdatedAt            string                              `json:"updatedAt"`
}

type attendanceWorkScheduleBreakPeriod struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

func defaultAttendanceWorkSchedule() attendanceWorkSchedule {
	return attendanceWorkSchedule{
		Version:              attendanceWorkScheduleVersion,
		WorkMode:             attendanceWorkModeFlexible,
		WorkingWeekdays:      []int{1, 2, 3, 4, 5},
		ScheduledWorkMinutes: 8 * 60,
		ReferenceStartTime:   "09:00",
		FixedStartTime:       "",
		FixedEndTime:         "",
		BreakPeriods: []attendanceWorkScheduleBreakPeriod{{
			StartTime: "12:00",
			EndTime:   "13:00",
		}},
		UpdatedAt: "",
	}
}
