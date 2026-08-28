package centralplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAttendanceReconciliationCarriesTheActualWorkMode(t *testing.T) {
	var carried map[string]any
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if errorValue := json.NewDecoder(request.Body).Decode(&carried); errorValue != nil {
			t.Fatal(errorValue)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"added":0,"removed":0,"refused":[]}`))
	}))
	defer plane.Close()

	client := New(Settings{AppURL: plane.URL, AgentAPIKey: "agent-key"})
	_, errorValue := client.ReconcileAttendance(context.Background(), ReconcileWindow{
		Platform: "mattermost",
		WorkMode: "fixed",
		From:     time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		To:       time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if carried["workMode"] != "fixed" {
		t.Fatalf("work mode = %v", carried["workMode"])
	}
}

func TestAttendanceReconciliationCarriesEveryPolicyRevision(t *testing.T) {
	var carried map[string]json.RawMessage
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if errorValue := json.NewDecoder(request.Body).Decode(&carried); errorValue != nil {
			t.Fatal(errorValue)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"added":0,"removed":0,"refused":[]}`))
	}))
	defer plane.Close()

	policy := ReconciledWorkPolicy{
		Version: 1,
		Revisions: []ReconciledWorkPolicyRevision{
			{
				EffectiveDate:       "1970-01-01",
				WorkMode:            "flexible",
				WorkingWeekdays:     []int{1, 2, 3, 4, 5},
				DailyTargetMinutes:  480,
				WeeklyTargetMinutes: 2400,
				NightStartTime:      "22:00",
				NightEndTime:        "06:00",
			},
			{
				EffectiveDate:       "2026-08-01",
				WorkMode:            "fixed",
				WorkingWeekdays:     []int{1, 2, 3, 4, 5},
				DailyTargetMinutes:  480,
				WeeklyTargetMinutes: 2400,
				NightStartTime:      "22:00",
				NightEndTime:        "06:00",
			},
		},
	}
	client := New(Settings{AppURL: plane.URL, AgentAPIKey: "agent-key"})
	_, errorValue := client.ReconcileAttendance(context.Background(), ReconcileWindow{
		Platform:   "mattermost",
		WorkPolicy: &policy,
		From:       time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	var decoded ReconciledWorkPolicy
	if errorValue := json.Unmarshal(carried["workPolicy"], &decoded); errorValue != nil {
		t.Fatal(errorValue)
	}
	if decoded.Version != 1 || len(decoded.Revisions) != 2 {
		t.Fatalf("work policy = %+v", decoded)
	}
	if decoded.Revisions[0].EffectiveDate != "1970-01-01" ||
		decoded.Revisions[0].WorkMode != "flexible" {
		t.Fatalf("the revision that covers the past did not travel: %+v", decoded.Revisions[0])
	}
	if decoded.Revisions[1].EffectiveDate != "2026-08-01" ||
		decoded.Revisions[1].WorkMode != "fixed" ||
		decoded.Revisions[1].NightStartTime != "22:00" {
		t.Fatalf("the current revision did not travel: %+v", decoded.Revisions[1])
	}
}

func TestAttendanceReconciliationCarriesTheCompanyHolidays(t *testing.T) {
	var carried map[string]json.RawMessage
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if errorValue := json.NewDecoder(request.Body).Decode(&carried); errorValue != nil {
			t.Fatal(errorValue)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"added":0,"removed":0,"refused":[]}`))
	}))
	defer plane.Close()

	client := New(Settings{AppURL: plane.URL, AgentAPIKey: "agent-key"})
	_, errorValue := client.ReconcileAttendance(context.Background(), ReconcileWindow{
		Platform: "mattermost",
		From:     time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		To:       time.Date(2027, time.January, 2, 0, 0, 0, 0, time.UTC),
		CompanyHolidays: []ReconciledCompanyHoliday{{
			ID: "company-holiday-1", Title: "창립기념일", Date: "2027-01-02", RecursAnnually: true,
		}},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, carriedCalendar := carried["workCalendar"]; carriedCalendar {
		t.Fatal("a day per date is derived now and must not be sent")
	}
	var holidays []map[string]any
	if errorValue := json.Unmarshal(carried["companyHolidays"], &holidays); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(holidays) != 1 ||
		holidays[0]["id"] != "company-holiday-1" ||
		holidays[0]["title"] != "창립기념일" ||
		holidays[0]["date"] != "2027-01-02" ||
		holidays[0]["recursAnnually"] != true {
		t.Fatalf("company holidays = %#v", holidays)
	}
}

func TestSettingsNeedEveryPart(t *testing.T) {
	complete := Settings{AppURL: "https://app", AgentAPIKey: "key", ProjectURL: "https://plane", PublishableKey: "publishable"}
	if !complete.Configured() {
		t.Fatal("a complete setting should be usable")
	}
	for _, missing := range []Settings{
		{AgentAPIKey: "key", ProjectURL: "https://plane", PublishableKey: "publishable"},
		{AppURL: "https://app", ProjectURL: "https://plane", PublishableKey: "publishable"},
		{AppURL: "https://app", AgentAPIKey: "key", PublishableKey: "publishable"},
		{AppURL: "https://app", AgentAPIKey: "key", ProjectURL: "https://plane"},
	} {
		if missing.Configured() {
			t.Fatalf("a setting missing a part must not be usable: %+v", missing)
		}
	}
}

func TestRecordAttendanceAsksForASessionThenWrites(t *testing.T) {
	var sessionRequests int
	var wroteBody string
	var carriedToken string

	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/api/agent/session"):
			sessionRequests++
			if request.Header.Get("Authorization") != "Bearer agent-key" {
				t.Errorf("the agent key must prove the company, got %q", request.Header.Get("Authorization"))
			}
			writer.Header().Set("Content-Type", "application/json")
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"member-token","expiresAt":` +
				timestampIn(time.Hour) + `}`))
		case strings.HasSuffix(request.URL.Path, "/rest/v1/attendance"):
			carriedToken = request.Header.Get("Authorization")
			body := make([]byte, request.ContentLength)
			request.Body.Read(body)
			wroteBody = string(body)
			writer.WriteHeader(http.StatusCreated)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer plane.Close()

	client := New(Settings{AppURL: plane.URL, AgentAPIKey: "agent-key", ProjectURL: plane.URL, PublishableKey: "publishable"})
	record := AttendanceRecord{Platform: "mattermost", ExternalID: "U1", Kind: "clock_in", Location: "사무실", OccurredAt: time.Now()}

	if errorValue := client.RecordAttendance(context.Background(), record); errorValue != nil {
		t.Fatalf("recording should have worked: %v", errorValue)
	}
	if carriedToken != "Bearer member-token" {
		t.Fatalf("the write must be made as the member, got %q", carriedToken)
	}
	if !strings.Contains(wroteBody, `"member_id":"member-1"`) || !strings.Contains(wroteBody, `"location":"사무실"`) {
		t.Fatalf("the record should carry the member and where they are, got %s", wroteBody)
	}

	if errorValue := client.RecordAttendance(context.Background(), record); errorValue != nil {
		t.Fatalf("the second record should have worked: %v", errorValue)
	}
	if sessionRequests != 1 {
		t.Fatalf("a session should be asked for once and reused, asked %d times", sessionRequests)
	}
}

func TestClockOutCarriesNoPlace(t *testing.T) {
	var wroteBody string
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/api/agent/session") {
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"member-token","expiresAt":` + timestampIn(time.Hour) + `}`))
			return
		}
		body := make([]byte, request.ContentLength)
		request.Body.Read(body)
		wroteBody = string(body)
		writer.WriteHeader(http.StatusCreated)
	}))
	defer plane.Close()

	client := New(Settings{AppURL: plane.URL, AgentAPIKey: "agent-key", ProjectURL: plane.URL, PublishableKey: "publishable"})
	errorValue := client.RecordAttendance(context.Background(), AttendanceRecord{
		Platform: "mattermost", ExternalID: "U1", Kind: "clock_out", Location: "사무실", OccurredAt: time.Now(),
	})
	if errorValue != nil {
		t.Fatalf("recording should have worked: %v", errorValue)
	}
	if strings.Contains(wroteBody, "location") {
		t.Fatalf("leaving has no place, got %s", wroteBody)
	}
}

func TestARefusedIdentityIsReportedNotHidden(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer plane.Close()

	client := New(Settings{AppURL: plane.URL, AgentAPIKey: "agent-key", ProjectURL: plane.URL, PublishableKey: "publishable"})
	errorValue := client.RecordAttendance(context.Background(), AttendanceRecord{
		Platform: "mattermost", ExternalID: "nobody", Kind: "clock_in", OccurredAt: time.Now(),
	})
	if errorValue == nil {
		t.Fatal("a refusal must be reported to the caller")
	}
}

func timestampIn(ahead time.Duration) string {
	return strconv.FormatInt(time.Now().Add(ahead).Unix(), 10)
}
