package admind

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"
)

const attendanceWorkConformanceEmail = "이샘플@example.com"

type attendanceWorkConformancePolicy struct {
	WorkMode           string                              `json:"workMode"`
	WorkingWeekdays    []int                               `json:"workingWeekdays"`
	DailyTargetMinutes int                                 `json:"dailyTargetMinutes"`
	ReferenceStartTime string                              `json:"referenceStartTime"`
	FixedStartTime     string                              `json:"fixedStartTime"`
	FixedEndTime       string                              `json:"fixedEndTime"`
	CoreTimeEnabled    bool                                `json:"coreTimeEnabled"`
	CoreStartTime      string                              `json:"coreStartTime"`
	CoreEndTime        string                              `json:"coreEndTime"`
	BreakPeriods       []attendanceWorkScheduleBreakPeriod `json:"breakPeriods"`
	NightStartTime     string                              `json:"nightStartTime"`
	NightEndTime       string                              `json:"nightEndTime"`
}

type attendanceWorkConformanceEvent struct {
	Kind       string `json:"kind"`
	OccurredAt string `json:"occurredAt"`
	Location   string `json:"location,omitempty"`
}

type attendanceWorkConformanceLeave struct {
	StartDate string  `json:"startDate"`
	EndDate   string  `json:"endDate"`
	Days      float64 `json:"days"`
}

type attendanceWorkConformanceExpected struct {
	ActualSeconds        int  `json:"actualSeconds"`
	ActualMinutes        int  `json:"actualMinutes"`
	ProvisionalSeconds   int  `json:"provisionalSeconds"`
	ProvisionalMinutes   int  `json:"provisionalMinutes"`
	TargetMinutes        int  `json:"targetMinutes"`
	LeaveMinutes         int  `json:"leaveMinutes"`
	NightMinutes         int  `json:"nightMinutes"`
	HasBaseline          bool `json:"hasBaseline"`
	HasIncompleteRecords bool `json:"hasIncompleteRecords"`
	NeedsReview          bool `json:"needsReview"`
}

type attendanceWorkConformanceScenario struct {
	Name        string                            `json:"name"`
	TimeZone    string                            `json:"timeZone"`
	Now         string                            `json:"now"`
	PeriodStart string                            `json:"periodStart"`
	PeriodEnd   string                            `json:"periodEnd"`
	Policy      attendanceWorkConformancePolicy   `json:"policy"`
	Events      []attendanceWorkConformanceEvent  `json:"events"`
	Leave       []attendanceWorkConformanceLeave  `json:"leave"`
	Expected    attendanceWorkConformanceExpected `json:"expected"`
}

type attendanceWorkConformanceFile struct {
	Scenarios []attendanceWorkConformanceScenario `json:"scenarios"`
}

func TestAttendanceWorkConformanceMatchesSharedScenarios(t *testing.T) {
	raw, errorValue := os.ReadFile("testdata/attendance-work-conformance.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var file attendanceWorkConformanceFile
	if errorValue := json.Unmarshal(raw, &file); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(file.Scenarios) == 0 {
		t.Fatal("conformance file has no scenarios")
	}

	executed := 0
	for _, scenario := range file.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			executed++
			location, errorValue := time.LoadLocation(scenario.TimeZone)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			now, errorValue := parseAttendanceEventTime(scenario.Now)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			events, errorValue := attendanceWorkConformanceEvents(scenario)
			if errorValue != nil {
				t.Fatalf("scenario %s: %v", scenario.Name, errorValue)
			}
			leaveOccurrences, errorValue := attendanceWorkConformanceLeaveOccurrences(scenario)
			if errorValue != nil {
				t.Fatalf("scenario %s: %v", scenario.Name, errorValue)
			}
			policy := attendanceWorkConformanceGoPolicy(scenario.Policy)

			status, errorValue := calculateAttendanceWorkStatus(
				attendanceWorkConformanceEmail,
				"이샘플",
				scenario.PeriodStart,
				scenario.PeriodEnd,
				events,
				leaveOccurrences,
				policy,
				map[string]struct{}{},
				location,
				now,
			)
			if errorValue != nil {
				t.Fatalf("scenario %s: %v", scenario.Name, errorValue)
			}

			assertAttendanceWorkConformanceField(t, scenario.Name, "actualSeconds", status.ActualSeconds, scenario.Expected.ActualSeconds)
			assertAttendanceWorkConformanceField(t, scenario.Name, "actualMinutes", status.ActualMinutes, scenario.Expected.ActualMinutes)
			assertAttendanceWorkConformanceField(t, scenario.Name, "provisionalSeconds", status.ProvisionalSeconds, scenario.Expected.ProvisionalSeconds)
			assertAttendanceWorkConformanceField(t, scenario.Name, "provisionalMinutes", status.ProvisionalMinutes, scenario.Expected.ProvisionalMinutes)
			assertAttendanceWorkConformanceField(t, scenario.Name, "targetMinutes", status.TargetMinutes, scenario.Expected.TargetMinutes)
			assertAttendanceWorkConformanceField(t, scenario.Name, "leaveMinutes", status.LeaveMinutes, scenario.Expected.LeaveMinutes)
			assertAttendanceWorkConformanceField(t, scenario.Name, "nightMinutes", status.NightMinutes, scenario.Expected.NightMinutes)
			assertAttendanceWorkConformanceBoolField(t, scenario.Name, "hasBaseline", status.HasBaseline, scenario.Expected.HasBaseline)
			assertAttendanceWorkConformanceBoolField(t, scenario.Name, "hasIncompleteRecords", status.HasIncompleteRecords, scenario.Expected.HasIncompleteRecords)
			assertAttendanceWorkConformanceBoolField(t, scenario.Name, "needsReview", status.NeedsReview, scenario.Expected.NeedsReview)
		})
	}

	if executed != len(file.Scenarios) {
		t.Fatalf("executed %d scenarios, want %d", executed, len(file.Scenarios))
	}
}

func attendanceWorkConformanceEvents(
	scenario attendanceWorkConformanceScenario,
) ([]attendanceEvent, error) {
	events := make([]attendanceEvent, 0, len(scenario.Events))
	for index, event := range scenario.Events {
		switch event.Kind {
		case attendanceKindClockIn, attendanceKindClockOut:
		default:
			return nil, errAttendanceWorkConformanceUnknownEventKind(event.Kind)
		}
		events = append(events, attendanceEvent{
			ID:          scenario.Name + "-event-" + strconv.Itoa(index),
			Email:       attendanceWorkConformanceEmail,
			DisplayName: "이샘플",
			Kind:        event.Kind,
			OccurredAt:  event.OccurredAt,
			LocationID:  event.Location,
		})
	}
	return events, nil
}

func attendanceWorkConformanceLeaveOccurrences(
	scenario attendanceWorkConformanceScenario,
) ([]attendanceApprovedLeaveOccurrence, error) {
	occurrences := make([]attendanceApprovedLeaveOccurrence, 0, len(scenario.Leave))
	for _, leave := range scenario.Leave {
		startDate, errorValue := attendanceWorkScheduleDate(leave.StartDate, time.UTC)
		if errorValue != nil {
			return nil, errorValue
		}
		endDate, errorValue := attendanceWorkScheduleDate(leave.EndDate, time.UTC)
		if errorValue != nil {
			return nil, errorValue
		}
		if leave.Days > 0.5 {
			for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
				occurrences = append(occurrences, attendanceApprovedLeaveOccurrence{
					Email:              attendanceWorkConformanceEmail,
					Date:               date.Format(time.DateOnly),
					StartTime:          "00:00",
					EndTime:            "23:59",
					DeductionMilliDays: 1000,
					Paid:               true,
				})
			}
			continue
		}
		occurrences = append(occurrences, attendanceApprovedLeaveOccurrence{
			Email:              attendanceWorkConformanceEmail,
			Date:               leave.StartDate,
			StartTime:          "00:00",
			EndTime:            "23:59",
			DeductionMilliDays: int(leave.Days*1000 + 0.5),
			Paid:               true,
		})
	}
	return occurrences, nil
}

func attendanceWorkConformanceGoPolicy(
	source attendanceWorkConformancePolicy,
) attendanceWorkPolicy {
	weeklyTargetMinutes := 0
	if source.WorkMode != attendanceWorkModeAutonomous {
		weeklyTargetMinutes = source.DailyTargetMinutes * len(source.WorkingWeekdays)
	}
	revision := attendanceWorkPolicyRevision{
		EffectiveDate:       attendanceWorkPolicyInitialEffectiveDate,
		WorkMode:            source.WorkMode,
		WorkingWeekdays:     source.WorkingWeekdays,
		DailyTargetMinutes:  source.DailyTargetMinutes,
		WeeklyTargetMinutes: weeklyTargetMinutes,
		ReferenceStartTime:  source.ReferenceStartTime,
		FixedStartTime:      source.FixedStartTime,
		FixedEndTime:        source.FixedEndTime,
		CoreTimeEnabled:     source.CoreTimeEnabled,
		CoreStartTime:       source.CoreStartTime,
		CoreEndTime:         source.CoreEndTime,
		BreakPeriods:        source.BreakPeriods,
		NightStartTime:      source.NightStartTime,
		NightEndTime:        source.NightEndTime,
	}
	return attendanceWorkPolicy{
		Version:   attendanceWorkPolicyVersion,
		UpdatedAt: time.Unix(0, 0).UTC().Format(time.RFC3339),
		Revisions: []attendanceWorkPolicyRevision{revision},
	}
}

func assertAttendanceWorkConformanceField(
	t *testing.T,
	scenarioName string,
	fieldName string,
	got int,
	want int,
) {
	t.Helper()
	if got != want {
		t.Errorf("scenario %s: field %s = %d, want %d", scenarioName, fieldName, got, want)
	}
}

func assertAttendanceWorkConformanceBoolField(
	t *testing.T,
	scenarioName string,
	fieldName string,
	got bool,
	want bool,
) {
	t.Helper()
	if got != want {
		t.Errorf("scenario %s: field %s = %t, want %t", scenarioName, fieldName, got, want)
	}
}

func errAttendanceWorkConformanceUnknownEventKind(kind string) error {
	return &attendanceWorkConformanceUnknownEventKindError{Kind: kind}
}

type attendanceWorkConformanceUnknownEventKindError struct {
	Kind string
}

func (errorValue *attendanceWorkConformanceUnknownEventKindError) Error() string {
	return "unknown conformance event kind: " + errorValue.Kind
}
