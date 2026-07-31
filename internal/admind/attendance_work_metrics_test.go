package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAttendanceWorkMetricsSubtractBreaksAndCountPaidLeave(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	events := []attendanceEvent{
		workMetricEvent("in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
		workMetricEvent("out", attendanceKindClockOut, "2026-07-31T04:00:00Z"),
	}
	leaves := []attendancePaidLeaveOccurrence{{
		Email:              "kim@example.com",
		Date:               "2026-07-31",
		DeductionMilliDays: 500,
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
		time.Date(2026, time.July, 31, 12, 0, 0, 0, location),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if status.TargetMinutes != 480 ||
		status.ActualMinutes != 180 ||
		status.PaidLeaveMinutes != 240 ||
		status.CreditedLeaveMinutes != 240 ||
		status.FulfilledMinutes != 420 ||
		status.RemainingMinutes != 60 ||
		status.OvertimeMinutes != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsCapLeaveAndDeriveOvertimeFromActualWork(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceWorkPolicy()
	events := []attendanceEvent{
		workMetricEvent("in", attendanceKindClockIn, "2026-07-31T00:00:00Z"),
		workMetricEvent("out", attendanceKindClockOut, "2026-07-31T11:00:00Z"),
	}
	leaves := []attendancePaidLeaveOccurrence{{
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
		status.PaidLeaveMinutes != 480 ||
		status.CreditedLeaveMinutes != 0 ||
		status.FulfilledMinutes != 480 ||
		status.OvertimeMinutes != 120 ||
		status.RemainingMinutes != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestAttendanceWorkMetricsDistinguishShortfallOvertimeAndLeaveCredit(t *testing.T) {
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
		expectedPaidLeaveMinutes int
		expectedCreditedMinutes  int
		expectedRemainingMinutes int
		expectedOvertimeMinutes  int
	}{
		{name: "work shortfall", clockIn: "2026-07-31T01:00:00Z", clockOut: "2026-07-31T08:00:00Z", expectedActualMinutes: 360, expectedRemainingMinutes: 120},
		{name: "work and leave shortfall", clockIn: "2026-07-31T01:00:00Z", clockOut: "2026-07-31T07:00:00Z", deductionMilliDays: 250, expectedActualMinutes: 300, expectedPaidLeaveMinutes: 120, expectedCreditedMinutes: 120, expectedRemainingMinutes: 60},
		{name: "work overtime", clockIn: "2026-07-31T00:00:00Z", clockOut: "2026-07-31T10:00:00Z", expectedActualMinutes: 540, expectedOvertimeMinutes: 60},
		{name: "work and leave above target", clockIn: "2026-07-31T00:00:00Z", clockOut: "2026-07-31T08:00:00Z", deductionMilliDays: 250, expectedActualMinutes: 420, expectedPaidLeaveMinutes: 120, expectedCreditedMinutes: 60},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leaves := []attendancePaidLeaveOccurrence{}
			if testCase.deductionMilliDays > 0 {
				leaves = append(leaves, attendancePaidLeaveOccurrence{
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
				status.PaidLeaveMinutes != testCase.expectedPaidLeaveMinutes ||
				status.CreditedLeaveMinutes != testCase.expectedCreditedMinutes ||
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
		[]attendancePaidLeaveOccurrence{{
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
	if status.TargetMinutes != 0 || status.PaidLeaveMinutes != 0 || status.RemainingMinutes != 0 {
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
	leave := []attendancePaidLeaveOccurrence{
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

	overlappingLeave := append(leave, attendancePaidLeaveOccurrence{
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
	leaveReader := func(context.Context, string, string) ([]attendancePaidLeaveOccurrence, error) {
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
		DisplayName: "김철수",
		Kind:        kind,
		OccurredAt:  occurredAt,
	}
}
