package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAttendanceWorkPolicyDefaultsUseFlexibleWeek(t *testing.T) {
	policy := defaultAttendanceWorkPolicy()

	if policy.Version != attendanceWorkPolicyVersion || len(policy.Revisions) != 1 {
		t.Fatalf("default policy = %+v", policy)
	}
	revision := policy.Revisions[0]
	if revision.EffectiveDate != attendanceWorkPolicyInitialEffectiveDate ||
		revision.WorkMode != attendanceWorkModeFlexible ||
		revision.DailyTargetMinutes != 480 ||
		revision.WeeklyTargetMinutes != 2400 ||
		revision.NightStartTime != "22:00" ||
		revision.NightEndTime != "06:00" {
		t.Fatalf("default revision = %+v", revision)
	}
}

func TestAttendanceWorkPolicySavePreservesLeavePolicy(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	leavePolicy := defaultAttendanceLeavePolicy()
	leavePolicy.FiscalYearStartMonth = 4
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), leavePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}

	revision := defaultAttendanceWorkPolicyRevision()
	revision.WorkMode = attendanceWorkModeFixed
	revision.FixedStartTime = "09:00"
	revision.FixedEndTime = "18:00"
	now := time.Date(2026, time.July, 31, 2, 30, 0, 0, time.UTC)
	if _, errorValue := service.saveAttendanceWorkPolicyRevision(
		t.Context(),
		revision,
		"2026-07-31",
		now,
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	document, errorValue := service.readAttendanceSettingsDocument(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if document.LeavePolicy.FiscalYearStartMonth != 4 {
		t.Fatalf("leave policy changed = %+v", document.LeavePolicy)
	}
	if len(document.WorkPolicy.Revisions) != 2 {
		t.Fatalf("work policy revisions = %+v", document.WorkPolicy.Revisions)
	}
	encodedWorkPolicy, errorValue := json.Marshal(document.WorkPolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(encodedWorkPolicy), `"holidays"`) {
		t.Fatalf("work policy duplicates company holidays = %s", encodedWorkPolicy)
	}
}

func TestAttendanceWorkPolicyReplacesSameDateAndKeepsPastRevision(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	now := time.Date(2026, time.July, 31, 3, 0, 0, 0, time.UTC)

	flexible := defaultAttendanceWorkPolicyRevision()
	if _, errorValue := service.saveAttendanceWorkPolicyRevision(
		t.Context(),
		flexible,
		"2026-07-31",
		now,
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	fixed := defaultAttendanceWorkPolicyRevision()
	fixed.WorkMode = attendanceWorkModeFixed
	fixed.FixedStartTime = "08:00"
	fixed.FixedEndTime = "17:00"
	policy, errorValue := service.saveAttendanceWorkPolicyRevision(
		t.Context(),
		fixed,
		"2026-07-31",
		now.Add(time.Hour),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(policy.Revisions) != 2 {
		t.Fatalf("revisions = %+v", policy.Revisions)
	}
	current, errorValue := attendanceWorkPolicyRevisionForDate(policy, "2026-07-31")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if current.WorkMode != attendanceWorkModeFixed || current.FixedStartTime != "08:00" {
		t.Fatalf("current revision = %+v", current)
	}
	past, errorValue := attendanceWorkPolicyRevisionForDate(policy, "2026-07-30")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if past.WorkMode != attendanceWorkModeFlexible ||
		past.EffectiveDate != attendanceWorkPolicyInitialEffectiveDate {
		t.Fatalf("past revision = %+v", past)
	}
}

func TestAttendanceWorkPolicyRejectsInvalidModeSpecificValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*attendanceWorkPolicyRevision)
	}{
		{
			name: "unknown mode",
			mutate: func(revision *attendanceWorkPolicyRevision) {
				revision.WorkMode = "hybrid"
			},
		},
		{
			name: "flexible weekly mismatch",
			mutate: func(revision *attendanceWorkPolicyRevision) {
				revision.WeeklyTargetMinutes = 2399
			},
		},
		{
			name: "fixed missing times",
			mutate: func(revision *attendanceWorkPolicyRevision) {
				revision.WorkMode = attendanceWorkModeFixed
			},
		},
		{
			name: "invalid core time",
			mutate: func(revision *attendanceWorkPolicyRevision) {
				revision.CoreTimeEnabled = true
				revision.CoreStartTime = "16:00"
				revision.CoreEndTime = "11:00"
			},
		},
		{
			name: "same night boundary",
			mutate: func(revision *attendanceWorkPolicyRevision) {
				revision.NightEndTime = revision.NightStartTime
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			revision := defaultAttendanceWorkPolicyRevision()
			testCase.mutate(&revision)
			if errorValue := validateAndNormalizeAttendanceWorkPolicyRevision(&revision); errorValue == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestAttendanceWorkPolicyHTTPRoundtripUsesCompanyDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	revision := defaultAttendanceWorkPolicyRevision()
	revision.EffectiveDate = ""
	revision.WorkMode = attendanceWorkModeFixed
	revision.FixedStartTime = "09:00"
	revision.FixedEndTime = "18:00"
	revision.CoreTimeEnabled = false
	encodedRevision, errorValue := json.Marshal(revision)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	putRecorder := httptest.NewRecorder()
	putRequest := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/attendance-work-policy",
		bytes.NewReader(encodedRevision),
	)
	now := time.Date(2026, time.July, 31, 16, 30, 0, 0, time.UTC)
	holidayReader := func(context.Context, time.Time, time.Time) (map[string]struct{}, error) {
		return map[string]struct{}{"2026-08-17": {}}, nil
	}
	service.handleAttendanceWorkPolicyAt(putRecorder, putRequest, now, holidayReader)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("put status = %d body = %s", putRecorder.Code, putRecorder.Body.String())
	}

	var savedResponse attendanceWorkPolicyResponse
	if errorValue = json.Unmarshal(putRecorder.Body.Bytes(), &savedResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	saved := savedResponse.Policy
	expectedDate := now.In(service.workspaceTimeZone().location).Format(time.DateOnly)
	current, errorValue := attendanceWorkPolicyRevisionForDate(saved, expectedDate)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if current.WorkMode != attendanceWorkModeFixed || current.EffectiveDate != expectedDate {
		t.Fatalf("current revision = %+v", current)
	}

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/admin/api/attendance-work-policy", nil)
	service.handleAttendanceWorkPolicyAt(getRecorder, getRequest, now, holidayReader)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d body = %s", getRecorder.Code, getRecorder.Body.String())
	}
	var loadedResponse attendanceWorkPolicyResponse
	if errorValue = json.Unmarshal(getRecorder.Body.Bytes(), &loadedResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	loaded := loadedResponse.Policy
	if len(loaded.Revisions) != len(saved.Revisions) {
		t.Fatalf("loaded = %+v saved = %+v", loaded, saved)
	}
	if loadedResponse.CurrentMonth != "2026-08" ||
		loadedResponse.TimeZone != service.workspaceTimeZone().name ||
		len(loadedResponse.HolidayDates) != 1 ||
		loadedResponse.HolidayDates[0] != "2026-08-17" {
		t.Fatalf("response = %+v", loadedResponse)
	}
}
