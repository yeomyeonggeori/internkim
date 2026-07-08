package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServeCalendarAccountStatusReportsDisconnected(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.Connected {
		t.Error("Connected should be false without account")
	}
	if body.NeedsReauth {
		t.Error("NeedsReauth should be false when disconnected")
	}
	if body.GoogleOAuthConfigured {
		t.Error("GoogleOAuthConfigured should be false without client configuration")
	}
}

func TestServeCalendarAccountStatusReportsGoogleOAuthConfigured(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if !body.GoogleOAuthConfigured {
		t.Error("GoogleOAuthConfigured should be true with valid client configuration")
	}
}

func TestServeCalendarAccountStatusRejectsIncompleteGoogleOAuthClient(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1"}}`)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.GoogleOAuthConfigured {
		t.Error("GoogleOAuthConfigured should be false without client_secret")
	}
}

func TestServeCalendarAccountStatusReportsGoogleOAuthManagePermission(t *testing.T) {
	service := newCalendarTestService(t)
	adminRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/account-status", nil)
	adminRequest.RemoteAddr = "127.0.0.1:34567"
	adminRecorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(adminRecorder, adminRequest)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("admin status: %d", adminRecorder.Code)
	}
	var adminBody calendarAccountStatusResponse
	if errorValue := json.Unmarshal(adminRecorder.Body.Bytes(), &adminBody); errorValue != nil {
		t.Fatalf("decode admin: %v", errorValue)
	}
	if !adminBody.CanManageGoogleOAuth {
		t.Error("CanManageGoogleOAuth should be true for admin requests")
	}

	configureCalendarTestUsers(t, service, `[{"email":"operator@example.com","role":"operationsAdmin"},{"email":"staff@example.com","role":"member"}]`)
	operationsAdminRequest := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	operationsAdminRequest.Header.Set("X-Forwarded-Email", "operator@example.com")
	operationsAdminRecorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(operationsAdminRecorder, operationsAdminRequest)
	if operationsAdminRecorder.Code != http.StatusOK {
		t.Fatalf("operations admin status: %d", operationsAdminRecorder.Code)
	}
	var operationsAdminBody calendarAccountStatusResponse
	if errorValue := json.Unmarshal(operationsAdminRecorder.Body.Bytes(), &operationsAdminBody); errorValue != nil {
		t.Fatalf("decode operations admin: %v", errorValue)
	}
	if !operationsAdminBody.CanManageGoogleOAuth {
		t.Error("CanManageGoogleOAuth should be true for operations admin requests")
	}

	staffRequest := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	staffRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	staffRecorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(staffRecorder, staffRequest)
	if staffRecorder.Code != http.StatusOK {
		t.Fatalf("staff status: %d", staffRecorder.Code)
	}
	var staffBody calendarAccountStatusResponse
	if errorValue := json.Unmarshal(staffRecorder.Body.Bytes(), &staffBody); errorValue != nil {
		t.Fatalf("decode staff: %v", errorValue)
	}
	if staffBody.CanManageGoogleOAuth {
		t.Error("CanManageGoogleOAuth should be false for non-admin requests")
	}
}

func TestServeCalendarAccountStatusAllowsOperationsAdminToManageGoogleOAuth(t *testing.T) {
	service := newOperationsAdminAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodGet, "http://admind.local/calendar/api/account-status", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "operator@example.com")
	recorder := httptest.NewRecorder()

	service.serveCalendarAccountStatus(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if !body.CanManageGoogleOAuth {
		t.Error("CanManageGoogleOAuth should be true for operations admin requests")
	}
}

func TestServeCalendarAccountStatusReportsConnectedHealthy(t *testing.T) {
	service := newCalendarTestService(t)
	seedAccountWithDiscovery(t, service)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if !body.Connected {
		t.Error("Connected should be true")
	}
	if body.Provider != remoteCalendarProviderGoogle {
		t.Errorf("Provider: got %q", body.Provider)
	}
	if body.NeedsReauth {
		t.Error("NeedsReauth should be false without auth error")
	}
	if body.LastAuthError != "" {
		t.Errorf("LastAuthError should be empty: %q", body.LastAuthError)
	}
	if !body.NeedsCalendarSelection {
		t.Error("NeedsCalendarSelection should be true without selected calendar")
	}
	if body.CalendarSyncReady {
		t.Error("CalendarSyncReady should be false without selected calendar")
	}
	if body.InitialSyncCompleted {
		t.Error("InitialSyncCompleted should be false without selected calendar")
	}
	if body.CalendarReadinessStatus != calendarReadinessStatusCalendarSelectionRequired {
		t.Errorf("CalendarReadinessStatus: got %q", body.CalendarReadinessStatus)
	}
}

func TestServeCalendarAccountStatusReportsSelectedCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account = seedSelectedCalendar(t, service, ctx, account, "writer", true)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.SelectedCalendarID != account.SelectedCalendarID {
		t.Errorf("SelectedCalendarID: got %q, want %q", body.SelectedCalendarID, account.SelectedCalendarID)
	}
	if body.SelectedCalendarName != account.SelectedCalendarSummary {
		t.Errorf("SelectedCalendarName: got %q, want %q", body.SelectedCalendarName, account.SelectedCalendarSummary)
	}
	if body.SelectedCalendarAccessRole != account.SelectedCalendarAccessRole {
		t.Errorf("SelectedCalendarAccessRole: got %q, want %q", body.SelectedCalendarAccessRole, account.SelectedCalendarAccessRole)
	}
	if body.NeedsCalendarSelection {
		t.Error("NeedsCalendarSelection should be false with selected calendar")
	}
	if !body.InitialSyncCompleted {
		t.Error("InitialSyncCompleted should be true when initial sync timestamp exists")
	}
	if !body.CalendarSyncReady {
		t.Error("CalendarSyncReady should be true with selected calendar, completed initial sync, and no auth error")
	}
	if body.CalendarReadinessStatus != calendarReadinessStatusSyncReady {
		t.Errorf("CalendarReadinessStatus: got %q", body.CalendarReadinessStatus)
	}
}

func TestServeCalendarAccountStatusWaitsForInitialSync(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	seedSelectedCalendar(t, service, ctx, account, "writer", false)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.NeedsCalendarSelection {
		t.Error("NeedsCalendarSelection should be false with selected calendar")
	}
	if body.InitialSyncCompleted {
		t.Error("InitialSyncCompleted should be false without initial sync timestamp")
	}
	if body.CalendarSyncReady {
		t.Error("CalendarSyncReady should be false until initial sync completes")
	}
	if body.CalendarReadinessStatus != calendarReadinessStatusInitialSyncPending {
		t.Errorf("CalendarReadinessStatus: got %q", body.CalendarReadinessStatus)
	}
}

func TestServeCalendarAccountStatusReportsInitialExportPending(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account = seedSelectedCalendar(t, service, ctx, account, "writer", false)
	account.SelectedCalendarReadinessStatus = calendarReadinessStatusInitialExportPending
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
		t.Fatalf("save readiness status: %v", errorValue)
	}
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.InitialSyncCompleted {
		t.Error("InitialSyncCompleted should stay false while initial export is pending")
	}
	if body.CalendarSyncReady {
		t.Error("CalendarSyncReady should be false while initial export is pending")
	}
	if body.CalendarReadinessStatus != calendarReadinessStatusInitialExportPending {
		t.Errorf("CalendarReadinessStatus: got %q", body.CalendarReadinessStatus)
	}
}

func TestServeCalendarAccountStatusRequiresWritableCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	seedSelectedCalendar(t, service, ctx, account, "reader", true)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.NeedsCalendarSelection {
		t.Error("NeedsCalendarSelection should be false with selected calendar")
	}
	if !body.InitialSyncCompleted {
		t.Error("InitialSyncCompleted should be true when initial sync timestamp exists")
	}
	if body.CalendarSyncReady {
		t.Error("CalendarSyncReady should be false for read-only calendars")
	}
	if body.CalendarReadinessStatus != calendarReadinessStatusWritePermissionRequired {
		t.Errorf("CalendarReadinessStatus: got %q", body.CalendarReadinessStatus)
	}
}

func TestServeCalendarAccountStatusFlagsReauthOnAuthError(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account = seedSelectedCalendar(t, service, ctx, account, "writer", true)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("invalid_grant: expired"))
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if !body.NeedsReauth {
		t.Error("NeedsReauth should be true when last_auth_error set")
	}
	if !strings.Contains(body.LastAuthError, "invalid_grant") {
		t.Errorf("LastAuthError: got %q", body.LastAuthError)
	}
	if body.LastAuthErrorAt == "" {
		t.Error("LastAuthErrorAt should be populated")
	}
	if body.CalendarSyncReady {
		t.Error("CalendarSyncReady should be false when reauth is required")
	}
	if body.CalendarReadinessStatus != calendarReadinessStatusReauthRequired {
		t.Errorf("CalendarReadinessStatus: got %q", body.CalendarReadinessStatus)
	}
}

func TestServeCalendarAccountStatusReportsCalendarInaccessible(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account = seedSelectedCalendar(t, service, ctx, account, "writer", true)
	account.SelectedCalendarReadinessStatus = calendarReadinessStatusCalendarInaccessible
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
		t.Fatalf("save readiness status: %v", errorValue)
	}
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.CalendarSyncReady {
		t.Error("CalendarSyncReady should be false when selected calendar is inaccessible")
	}
	if body.CalendarReadinessStatus != calendarReadinessStatusCalendarInaccessible {
		t.Errorf("CalendarReadinessStatus: got %q", body.CalendarReadinessStatus)
	}
}

func seedSelectedCalendar(t *testing.T, service *Service, ctx context.Context, account remoteCalendarAccount, accessRole string, isInitialSyncCompleted bool) remoteCalendarAccount {
	t.Helper()
	selectedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	account, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "회사 일정", accessRole, "/calendars/company/", selectedAt)
	if errorValue != nil {
		t.Fatalf("save selected calendar: %v", errorValue)
	}
	if isInitialSyncCompleted {
		completedAt := time.Date(2026, 7, 1, 10, 5, 0, 0, time.UTC)
		account, errorValue = service.saveCalendarPullState(ctx, account, account.DefaultCalendarCTag, completedAt, true)
		if errorValue != nil {
			t.Fatalf("mark initial sync completed: %v", errorValue)
		}
	}
	return account
}
