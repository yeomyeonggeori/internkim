package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAttendanceWorkMetricsSubtractBreaksAndReduceTargetByApprovedLeave(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	events := []attendanceEvent{
		workMetricEvent("in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
		workMetricEvent("out", attendanceKindClockOut, "2026-07-31T04:00:00Z"),
	}
	leaves := []attendanceApprovedLeaveOccurrence{{
		Email:              "kim@example.com",
		Date:               "2026-07-31",
		DeductionMilliDays: 500,
		Paid:               false,
	}}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-31",
		"2026-07-31",
		events,
		leaves,
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 12, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.TargetMinutes != 240 ||
		status.ActualMinutes != 180 ||
		status.LeaveMinutes != 240 ||
		status.FulfilledMinutes != 180 ||
		status.DifferenceMinutes != -60 ||
		status.RemainingMinutes != 60 ||
		status.OvertimeMinutes != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsCapLeaveToDailyTarget(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	events := []attendanceEvent{
		workMetricEvent("in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
		workMetricEvent("out", attendanceKindClockOut, "2026-07-31T11:00:00Z"),
	}
	leaves := []attendanceApprovedLeaveOccurrence{{
		Email:              "kim@example.com",
		Date:               "2026-07-31",
		DeductionMilliDays: 1000,
		Paid:               true,
	}}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		events,
		leaves,
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 21, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.ActualMinutes != 600 ||
		status.TargetMinutes != 0 ||
		status.LeaveMinutes != 480 ||
		status.FulfilledMinutes != 0 ||
		status.OvertimeMinutes != 0 ||
		status.RemainingMinutes != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsDistinguishShortfallOvertimeAndAdjustedTarget(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	testCases := []struct {
		name                     string
		clockIn                  string
		clockOut                 string
		deductionMilliDays       int
		expectedActualMinutes    int
		expectedLeaveMinutes     int
		expectedTargetMinutes    int
		expectedRemainingMinutes int
		expectedOvertimeMinutes  int
	}{
		{name: "work shortfall", clockIn: "2026-07-31T01:00:00Z", clockOut: "2026-07-31T08:00:00Z", expectedActualMinutes: 360, expectedTargetMinutes: 480, expectedRemainingMinutes: 120},
		{name: "work and leave shortfall", clockIn: "2026-07-31T01:00:00Z", clockOut: "2026-07-31T07:00:00Z", deductionMilliDays: 250, expectedActualMinutes: 300, expectedLeaveMinutes: 120, expectedTargetMinutes: 360, expectedRemainingMinutes: 60},
		{name: "work overtime", clockIn: "2026-07-31T00:00:00Z", clockOut: "2026-07-31T10:00:00Z", expectedActualMinutes: 540, expectedTargetMinutes: 480, expectedOvertimeMinutes: 60},
		{name: "work and leave above target", clockIn: "2026-07-31T00:00:00Z", clockOut: "2026-07-31T08:00:00Z", deductionMilliDays: 250, expectedActualMinutes: 420, expectedLeaveMinutes: 120, expectedTargetMinutes: 360, expectedOvertimeMinutes: 60},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leaves := []attendanceApprovedLeaveOccurrence{}
			if testCase.deductionMilliDays > 0 {
				leaves = append(leaves, attendanceApprovedLeaveOccurrence{
					Email:              "kim@example.com",
					Date:               "2026-07-31",
					DeductionMilliDays: testCase.deductionMilliDays,
					Paid:               true,
				})
			}
			status, calculateError := calculateAttendanceWorkStatus(
				"kim@example.com",
				"김철수",
				"2026-07-31",
				"2026-07-31",
				[]attendanceEvent{
					workMetricEvent("in", attendanceKindClockIn, testCase.clockIn),
					workMetricEvent("out", attendanceKindClockOut, testCase.clockOut),
				},
				leaves,
				defaultAttendanceWorkPolicy(),
				map[string]struct{}{},
				location,
				time.Date(2026, time.July, 31, 21, 0, 0, 0, location),
			)
			if calculateError != nil {
				t.Fatal(calculateError)
			}
			if status.ActualMinutes != testCase.expectedActualMinutes ||
				status.LeaveMinutes != testCase.expectedLeaveMinutes ||
				status.TargetMinutes != testCase.expectedTargetMinutes ||
				status.RemainingMinutes != testCase.expectedRemainingMinutes ||
				status.OvertimeMinutes != testCase.expectedOvertimeMinutes {
				t.Fatalf("status = %+v", status)
			}
		})
	}
}

func TestAttendanceWorkMetricsCountNightOverlap(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	events := []attendanceEvent{
		workMetricEvent("in", attendanceKindClockIn, "2026-07-31T13:00:00Z"),
		workMetricEvent("out", attendanceKindClockOut, "2026-07-31T14:30:00Z"),
	}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		events,
		nil,
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.ActualMinutes != 90 || status.NightMinutes != 90 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsAutonomousHasNoBaseline(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	policy.Revisions[0].WorkMode = attendanceWorkModeAutonomous
	policy.Revisions[0].WeeklyTargetMinutes = 0
	policy.Revisions[0].CoreTimeEnabled = false
	policy.Revisions[0].CoreStartTime = ""
	policy.Revisions[0].CoreEndTime = ""

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		nil,
		nil,
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 12, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.HasBaseline ||
		status.TargetMinutes != 0 ||
		status.RemainingMinutes != 0 ||
		status.OvertimeMinutes != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsJudgesEachDateByTheRevisionInForceThen(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	fixedRevision := defaultAttendanceWorkPolicyRevision()
	fixedRevision.WorkMode = attendanceWorkModeFixed
	fixedRevision.FixedStartTime = "09:00"
	fixedRevision.FixedEndTime = "18:00"
	fixedRevision.CoreTimeEnabled = false
	fixedRevision.CoreStartTime = ""
	fixedRevision.CoreEndTime = ""
	autonomousRevision := fixedRevision
	autonomousRevision.EffectiveDate = "2026-08-01"
	autonomousRevision.WorkMode = attendanceWorkModeAutonomous
	autonomousRevision.WeeklyTargetMinutes = 0
	autonomousRevision.FixedStartTime = ""
	autonomousRevision.FixedEndTime = ""
	policy := defaultAttendanceWorkPolicy()
	policy.Revisions = []attendanceWorkPolicyRevision{fixedRevision, autonomousRevision}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-31",
		"2026-08-01",
		[]attendanceEvent{
			workMetricEvent("fixed-in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
			workMetricEvent("fixed-out", attendanceKindClockOut, "2026-07-31T05:00:00Z"),
			workMetricEvent("in", attendanceKindClockIn, "2026-08-01T00:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-08-01T09:00:00Z"),
		},
		[]attendanceApprovedLeaveOccurrence{{
			Email:              "kim@example.com",
			Date:               "2026-08-01",
			DeductionMilliDays: 1000,
			Paid:               true,
		}},
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 2, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(status.Days) != 2 ||
		status.Days[0].WorkMode != attendanceWorkModeFixed ||
		status.Days[1].WorkMode != attendanceWorkModeAutonomous {
		t.Fatalf("each date must carry the mode in force on it: %+v", status.Days)
	}
	if !status.Days[0].HasBaseline ||
		status.Days[0].TargetMinutes != 480 ||
		status.Days[0].ActualMinutes != 240 {
		t.Fatalf("the day before the change must keep the fixed baseline: %+v", status.Days[0])
	}
	if status.Days[1].HasBaseline || status.Days[1].TargetMinutes != 0 {
		t.Fatalf("the day after the change must be autonomous: %+v", status.Days[1])
	}
	if !status.HasBaseline ||
		status.TargetMinutes != 480 ||
		status.ActualMinutes != 720 ||
		status.LeaveMinutes != 0 ||
		status.FulfilledMinutes != 240 ||
		status.DifferenceMinutes != -240 ||
		status.RemainingMinutes != 240 ||
		status.OvertimeMinutes != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsDeductTheBreakThatAppliedOnEachDate(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	shortBreak := defaultAttendanceWorkPolicyRevision()
	shortBreak.WorkMode = attendanceWorkModeFixed
	shortBreak.FixedStartTime = "09:00"
	shortBreak.FixedEndTime = "18:00"
	shortBreak.CoreTimeEnabled = false
	shortBreak.CoreStartTime = ""
	shortBreak.CoreEndTime = ""
	longBreak := shortBreak
	longBreak.EffectiveDate = "2026-08-03"
	longBreak.BreakPeriods = []attendanceWorkScheduleBreakPeriod{{StartTime: "12:00", EndTime: "14:00"}}
	longBreak.DailyTargetMinutes = 420
	longBreak.WeeklyTargetMinutes = 2100
	policy := defaultAttendanceWorkPolicy()
	policy.Revisions = []attendanceWorkPolicyRevision{shortBreak, longBreak}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-31",
		"2026-08-03",
		[]attendanceEvent{
			workMetricEvent("before-in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
			workMetricEvent("before-out", attendanceKindClockOut, "2026-07-31T09:00:00Z"),
			workMetricEvent("after-in", attendanceKindClockIn, "2026-08-03T00:00:00Z"),
			workMetricEvent("after-out", attendanceKindClockOut, "2026-08-03T09:00:00Z"),
		},
		[]attendanceApprovedLeaveOccurrence{},
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 4, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(status.Days) != 4 {
		t.Fatalf("days = %+v", status.Days)
	}
	if status.Days[0].Date != "2026-07-31" || status.Days[0].ActualMinutes != 480 {
		t.Fatalf("the day before the change keeps the one-hour break: %+v", status.Days[0])
	}
	if status.Days[3].Date != "2026-08-03" || status.Days[3].ActualMinutes != 420 {
		t.Fatalf("the day the change takes effect deducts the two-hour break: %+v", status.Days[3])
	}
}

func TestAttendanceWorkMetricsPreserveSecondsAndPeriodCapacities(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	policy.Revisions[0].BreakPeriods = nil
	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-27",
		"2026-08-02",
		[]attendanceEvent{
			workMetricEvent("in", attendanceKindClockIn, "2026-07-27T00:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-07-27T08:00:01Z"),
		},
		nil,
		policy,
		map[string]struct{}{"2026-07-29": {}},
		location,
		time.Date(2026, time.August, 3, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.ActualMinutes != 480 || status.ActualSeconds != 8*60*60+1 {
		t.Fatalf("actual duration = %d minutes, %d seconds", status.ActualMinutes, status.ActualSeconds)
	}
	if status.WorkingCapacitySeconds != 4*24*60*60 || status.CalendarCapacitySeconds != 7*24*60*60 {
		t.Fatalf("capacity = %d working, %d calendar", status.WorkingCapacitySeconds, status.CalendarCapacitySeconds)
	}
}

func TestAttendanceWorkMetricsAggregateOvertimeUsesPeriodTotal(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	events := []attendanceEvent{
		workMetricEvent("in-1", attendanceKindClockIn, "2026-07-30T00:00:00Z"),
		workMetricEvent("out-1", attendanceKindClockOut, "2026-07-30T11:00:00Z"),
		workMetricEvent("in-2", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
		workMetricEvent("out-2", attendanceKindClockOut, "2026-07-31T07:00:00Z"),
	}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-30",
		"2026-07-31",
		events,
		nil,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.TargetMinutes != 960 ||
		status.ActualMinutes != 960 ||
		status.OvertimeMinutes != 0 ||
		status.Days[0].OvertimeMinutes != 120 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsExcludeHolidaysFromBaseline(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		nil,
		[]attendanceApprovedLeaveOccurrence{{
			Email:              "kim@example.com",
			Date:               "2026-07-31",
			DeductionMilliDays: 1000,
			Paid:               true,
		}},
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{"2026-07-31": {}},
		location,
		time.Date(2026, time.August, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.TargetMinutes != 0 || status.LeaveMinutes != 0 || status.RemainingMinutes != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsSeparateInProgressAndIncompleteRecords(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	inProgress, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		[]attendanceEvent{workMetricEvent("in", attendanceKindClockIn, "2026-07-31T00:00:00Z")},
		nil,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 11, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if inProgress.ActualMinutes != 0 ||
		inProgress.ProvisionalMinutes != 120 ||
		!inProgress.IsWorking ||
		inProgress.Status != "working" {
		t.Fatalf("in progress = %+v", inProgress)
	}

	incomplete, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-30",
		"2026-07-30",
		[]attendanceEvent{workMetricEvent("in", attendanceKindClockIn, "2026-07-30T00:00:00Z")},
		nil,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 11, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !incomplete.NeedsReview ||
		!incomplete.HasIncompleteRecords ||
		incomplete.Status != "needsReview" {
		t.Fatalf("incomplete = %+v", incomplete)
	}
}

func TestAttendanceWorkMetricsFlagLeaveOverlapAndApplyScheduleExceptions(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	fixedPolicy := defaultAttendanceWorkPolicy()
	fixedPolicy.Revisions[0].WorkMode = attendanceWorkModeFixed
	fixedPolicy.Revisions[0].FixedStartTime = "09:00"
	fixedPolicy.Revisions[0].FixedEndTime = "18:00"
	fixedPolicy.Revisions[0].CoreTimeEnabled = false
	fixedPolicy.Revisions[0].CoreStartTime = ""
	fixedPolicy.Revisions[0].CoreEndTime = ""
	events := []attendanceEvent{
		workMetricEvent("in", attendanceKindClockIn, "2026-07-31T01:00:00Z"),
		workMetricEvent("out", attendanceKindClockOut, "2026-07-31T08:00:00Z"),
	}
	leave := []attendanceApprovedLeaveOccurrence{
		{Email: "kim@example.com", Date: "2026-07-31", StartTime: "09:00", EndTime: "10:00"},
		{Email: "kim@example.com", Date: "2026-07-31", StartTime: "17:00", EndTime: "18:00"},
	}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		events,
		leave,
		fixedPolicy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.Late || status.EarlyLeave || status.NeedsReview {
		t.Fatalf("fixed leave exception = %+v", status)
	}

	overlappingLeave := append(leave, attendanceApprovedLeaveOccurrence{
		Email: "kim@example.com", Date: "2026-07-31", StartTime: "11:00", EndTime: "12:00",
	})
	status, errorValue = calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		events,
		overlappingLeave,
		fixedPolicy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !status.NeedsReview || !status.HasLeaveWorkOverlap {
		t.Fatalf("leave overlap = %+v", status)
	}
}

func TestAttendanceWorkMetricsDetectCoreTimeMiss(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"김철수",
		"2026-07-31",
		"2026-07-31",
		[]attendanceEvent{
			workMetricEvent("in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-07-31T03:00:00Z"),
		},
		nil,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 1, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !status.CoreTimeMissed || status.Status != "coreTimeMissed" {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkStatusHTTPReturnsScopedWeeklyStatus(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.Configuration.TrustProxyForwardedEmail = true
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	eventsReader := func(
		context.Context,
		string,
		string,
	) ([]attendanceEvent, error) {
		return []attendanceEvent{
			workMetricEvent("in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-07-31T09:00:00Z"),
		}, nil
	}
	leaveReader := func(context.Context, string, string) ([]attendanceApprovedLeaveOccurrence, error) {
		return nil, nil
	}
	holidayReader := func(context.Context, time.Time, time.Time) (map[string]struct{}, error) {
		return map[string]struct{}{}, nil
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/attendance/api/work-status?period=week&anchor=2026-07-31",
		nil,
	)
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)

	service.writeAttendanceWorkStatusWithReadersAt(
		recorder,
		request,
		eventsReader,
		leaveReader,
		holidayReader,
		now,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceWorkStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.PeriodStart != "2026-07-27" ||
		response.PeriodEnd != "2026-08-02" ||
		response.Personal == nil ||
		response.Personal.Email != "staff@example.com" ||
		len(response.Employees) != 0 {
		t.Fatalf("response = %+v", response)
	}
}

func TestReadAttendanceWorkPeriodEventsIncludesAdjacentMonthBoundaries(t *testing.T) {
	requestedMonths := []string{}
	reader := func(_ context.Context, month string, _ string) ([]attendanceEvent, error) {
		requestedMonths = append(requestedMonths, month)
		if month == "2026-07" {
			event := workMetricEvent("in", attendanceKindClockIn, "2026-07-31T14:00:00Z")
			event.LocalDate = "2026-07-31"
			return []attendanceEvent{event}, nil
		}
		event := workMetricEvent("out", attendanceKindClockOut, "2026-07-31T16:00:00Z")
		event.LocalDate = "2026-08-01"
		return []attendanceEvent{event}, nil
	}

	events, errorValue := readAttendanceWorkPeriodEvents(
		t.Context(),
		"2026-07-31",
		"2026-07-31",
		reader,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 ||
		len(requestedMonths) != 2 ||
		requestedMonths[0] != "2026-07" ||
		requestedMonths[1] != "2026-08" {
		t.Fatalf("months = %v events = %+v", requestedMonths, events)
	}
}

func TestMergeAttendanceWorkMembersUsesEmailIdentityForDuplicateNames(t *testing.T) {
	members := []attendanceMember{{Email: "KIM@example.com", DisplayName: "김민지"}}
	events := []attendanceEvent{
		{Email: "kim@example.com", DisplayName: "변경된 이름"},
		{Email: "other@example.com", DisplayName: "김민지"},
	}

	merged := mergeAttendanceWorkMembers(members, events, "")
	if len(merged) != 2 {
		t.Fatalf("members = %+v", merged)
	}
	byEmail := map[string]attendanceMember{}
	for _, member := range merged {
		byEmail[member.Email] = member
	}
	if byEmail["kim@example.com"].DisplayName != "김민지" ||
		byEmail["other@example.com"].DisplayName != "김민지" {
		t.Fatalf("members = %+v", merged)
	}
}

func TestAttendanceWorkDisplayNameMatchesEmailCaseInsensitively(t *testing.T) {
	displayName := attendanceWorkDisplayName(
		[]attendanceEvent{{Email: "KIM@example.com", DisplayName: "김민지"}},
		"kim@example.com",
	)
	if displayName != "김민지" {
		t.Fatalf("display name = %q", displayName)
	}
}

func workMetricEvent(id string, kind string, occurredAt string) attendanceEvent {
	return attendanceEvent{
		ID:          id,
		Email:       "kim@example.com",
		DisplayName: "이샘플",
		Kind:        kind,
		OccurredAt:  occurredAt,
	}
}

func TestAttendanceWorkMetricsTreatLeaveCoveredDayAsLeaveNotOvertime(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	leaves := []attendanceApprovedLeaveOccurrence{{
		Email:              "kim@example.com",
		Date:               "2026-07-30",
		DeductionMilliDays: 1000,
		Paid:               true,
		StartTime:          "09:00",
		EndTime:            "18:00",
	}}

	restDay, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-30",
		"2026-07-30",
		nil,
		leaves,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if restDay.Days[0].TargetMinutes != 0 ||
		restDay.Days[0].LeaveMinutes != 480 ||
		restDay.Days[0].OvertimeMinutes != 0 ||
		restDay.Days[0].Status != "leaveCovered" ||
		restDay.Status != "leaveCovered" {
		t.Fatalf("rest day = %+v", restDay)
	}

	workedDay, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-30",
		"2026-07-30",
		[]attendanceEvent{
			workMetricEvent("in", attendanceKindClockIn, "2026-07-30T00:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-07-30T01:20:00Z"),
		},
		leaves,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if workedDay.Days[0].ActualMinutes != 80 ||
		workedDay.Days[0].OvertimeMinutes != 0 ||
		workedDay.Days[0].DifferenceMinutes != 0 ||
		workedDay.ActualMinutes != 80 ||
		workedDay.OvertimeMinutes != 0 {
		t.Fatalf("worked leave day = %+v", workedDay)
	}

	period, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-29",
		"2026-07-30",
		[]attendanceEvent{
			workMetricEvent("in-1", attendanceKindClockIn, "2026-07-29T00:00:00Z"),
			workMetricEvent("out-1", attendanceKindClockOut, "2026-07-29T09:00:00Z"),
			workMetricEvent("in-2", attendanceKindClockIn, "2026-07-30T00:00:00Z"),
			workMetricEvent("out-2", attendanceKindClockOut, "2026-07-30T01:20:00Z"),
		},
		leaves,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.July, 31, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if period.TargetMinutes != 480 ||
		period.ActualMinutes != 560 ||
		period.OvertimeMinutes != 0 ||
		period.DifferenceMinutes != 0 ||
		period.FulfilledMinutes != 480 {
		t.Fatalf("period = %+v", period)
	}
}

func TestAttendanceWorkMetricsKeepPeriodOvertimeWhenLeaveWeekHasNonWorkingDayWork(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	leaves := []attendanceApprovedLeaveOccurrence{}
	for _, date := range []string{"2026-07-27", "2026-07-28", "2026-07-29", "2026-07-30", "2026-07-31"} {
		leaves = append(leaves, attendanceApprovedLeaveOccurrence{
			Email:              "kim@example.com",
			Date:               date,
			DeductionMilliDays: 1000,
			Paid:               true,
			StartTime:          "09:00",
			EndTime:            "18:00",
		})
	}

	status, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-07-27",
		"2026-08-01",
		[]attendanceEvent{
			workMetricEvent("in", attendanceKindClockIn, "2026-08-01T00:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-08-01T03:00:00Z"),
		},
		leaves,
		defaultAttendanceWorkPolicy(),
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 2, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.TargetMinutes != 0 ||
		status.LeaveMinutes != 2400 ||
		status.ActualMinutes != 180 ||
		status.OvertimeMinutes != 180 ||
		status.Status != "overtime" {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsLeaveWithoutTimesCoversTheScheduledDay(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	policy.Revisions[0].WorkMode = attendanceWorkModeFixed
	policy.Revisions[0].FixedStartTime = "09:00"
	policy.Revisions[0].FixedEndTime = "18:00"
	policy.Revisions[0].CoreTimeEnabled = false
	policy.Revisions[0].CoreStartTime = ""
	policy.Revisions[0].CoreEndTime = ""
	timeless := []attendanceApprovedLeaveOccurrence{{
		Email:              "kim@example.com",
		Date:               "2026-08-20",
		DeductionMilliDays: 1000,
		Paid:               true,
	}}

	inside, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-08-20",
		"2026-08-20",
		[]attendanceEvent{
			workMetricEvent("in", attendanceKindClockIn, "2026-08-20T02:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-08-20T03:00:00Z"),
		},
		timeless,
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 21, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !inside.Days[0].HasLeaveWorkOverlap ||
		!inside.Days[0].NeedsReview ||
		inside.Days[0].Late ||
		inside.Days[0].EarlyLeave {
		t.Fatalf("work inside the scheduled day = %+v", inside.Days[0])
	}

	outside, errorValue := calculateAttendanceWorkStatus(
		"kim@example.com",
		"이샘플",
		"2026-08-20",
		"2026-08-20",
		[]attendanceEvent{
			workMetricEvent("in", attendanceKindClockIn, "2026-08-20T11:00:00Z"),
			workMetricEvent("out", attendanceKindClockOut, "2026-08-20T12:00:00Z"),
		},
		timeless,
		policy,
		map[string]struct{}{},
		location,
		time.Date(2026, time.August, 21, 0, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if outside.Days[0].HasLeaveWorkOverlap || outside.Days[0].NeedsReview {
		t.Fatalf("work after the scheduled day = %+v", outside.Days[0])
	}
}
