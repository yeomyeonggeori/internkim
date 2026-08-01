package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestAttendanceManagementRoleAccessThroughLoopback(t *testing.T) {
	service := newAttendanceRoleAuthorizationTestService(t)
	tests := []struct {
		name    string
		email   string
		allowed bool
	}{
		{name: "member", email: "member@example.com", allowed: false},
		{name: "operations admin", email: "operator@example.com", allowed: true},
		{name: "admin", email: "admin@example.com", allowed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary", nil)
			request.RemoteAddr = "127.0.0.1:1234"
			request.Header.Set("Cf-Access-Authenticated-User-Email", test.email)
			if allowed := service.canManageAttendance(request); allowed != test.allowed {
				t.Fatalf("canManageAttendance() = %v, want %v", allowed, test.allowed)
			}
		})
	}
}

func TestAttendanceSummaryManagementFlagByRoleThroughLoopback(t *testing.T) {
	service := newAttendanceRoleAuthorizationTestService(t)
	tests := []struct {
		name    string
		email   string
		isAdmin bool
	}{
		{name: "member", email: "member@example.com", isAdmin: false},
		{name: "operations admin", email: "operator@example.com", isAdmin: true},
		{name: "admin", email: "admin@example.com", isAdmin: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary", nil)
			request.RemoteAddr = "127.0.0.1:1234"
			request.Header.Set("Cf-Access-Authenticated-User-Email", test.email)
			recorder := httptest.NewRecorder()
			service.writeAttendanceSummaryWithReadersAt(
				recorder,
				request,
				func(context.Context, string, string) ([]attendanceEvent, error) {
					return []attendanceEvent{}, nil
				},
				func(context.Context, string, string) ([]attendanceAbsence, error) {
					return []attendanceAbsence{}, nil
				},
				time.Date(2026, 7, 31, 3, 0, 0, 0, time.UTC),
			)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
			}
			var summary attendanceSummaryResponse
			if errorValue := json.NewDecoder(recorder.Body).Decode(&summary); errorValue != nil {
				t.Fatal(errorValue)
			}
			if summary.IsAdmin != test.isAdmin {
				t.Fatalf("isAdmin = %v, want %v", summary.IsAdmin, test.isAdmin)
			}
		})
	}
}

func TestAttendanceWorkStatusManagementFlagByRoleThroughLoopback(t *testing.T) {
	service := newAttendanceRoleAuthorizationTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	tests := []struct {
		name    string
		email   string
		isAdmin bool
	}{
		{name: "member", email: "member@example.com", isAdmin: false},
		{name: "operations admin", email: "operator@example.com", isAdmin: true},
		{name: "admin", email: "admin@example.com", isAdmin: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/attendance/api/work-status?period=day&anchor=2026-07-31", nil)
			request.RemoteAddr = "127.0.0.1:1234"
			request.Header.Set("Cf-Access-Authenticated-User-Email", test.email)
			recorder := httptest.NewRecorder()
			service.writeAttendanceWorkStatusWithReadersAt(
				recorder,
				request,
				func(context.Context, string, string) ([]attendanceEvent, error) {
					return []attendanceEvent{}, nil
				},
				func(context.Context, string, string) ([]attendanceApprovedLeaveOccurrence, error) {
					return []attendanceApprovedLeaveOccurrence{}, nil
				},
				func(context.Context, time.Time, time.Time) (map[string]struct{}, error) {
					return map[string]struct{}{}, nil
				},
				time.Date(2026, 7, 31, 3, 0, 0, 0, time.UTC),
			)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
			}
			var response attendanceWorkStatusResponse
			if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.IsAdmin != test.isAdmin {
				t.Fatalf("isAdmin = %v, want %v", response.IsAdmin, test.isAdmin)
			}
			if test.isAdmin && len(response.Employees) != 3 {
				t.Fatalf("employees = %d, want 3", len(response.Employees))
			}
		})
	}
}

func TestAttendanceLocationsAccessByRoleThroughLoopback(t *testing.T) {
	service := newAttendanceRoleAuthorizationTestService(t)
	tests := []struct {
		name       string
		email      string
		statusCode int
	}{
		{name: "member", email: "member@example.com", statusCode: http.StatusForbidden},
		{name: "operations admin", email: "operator@example.com", statusCode: http.StatusOK},
		{name: "admin", email: "admin@example.com", statusCode: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/admin/api/attendance-locations", nil)
			request.RemoteAddr = "127.0.0.1:1234"
			request.Header.Set("Cf-Access-Authenticated-User-Email", test.email)
			recorder := httptest.NewRecorder()
			service.router().ServeHTTP(recorder, request)
			if recorder.Code != test.statusCode {
				t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAttendanceLeaveAdministrationAccessByRoleThroughLoopback(t *testing.T) {
	service := newAttendanceRoleAuthorizationTestService(t)
	for _, path := range []string{
		"/attendance/api/leave-approvals",
		"/attendance/api/leave-management",
	} {
		for _, test := range []struct {
			name       string
			email      string
			statusCode int
		}{
			{name: "member", email: "member@example.com", statusCode: http.StatusForbidden},
			{name: "operations admin", email: "operator@example.com", statusCode: http.StatusOK},
			{name: "admin", email: "admin@example.com", statusCode: http.StatusOK},
		} {
			t.Run(path+"/"+test.name, func(t *testing.T) {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				request.RemoteAddr = "127.0.0.1:1234"
				request.Header.Set("Cf-Access-Authenticated-User-Email", test.email)
				recorder := httptest.NewRecorder()
				service.router().ServeHTTP(recorder, request)
				if recorder.Code != test.statusCode {
					t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
				}
			})
		}
	}
}

func newAttendanceRoleAuthorizationTestService(t *testing.T) *Service {
	t.Helper()
	rootPath := t.TempDir()
	service := NewService(Configuration{
		APIBaseURL:            "https://api.example.test",
		ClaimedAdminEmailPath: writeTestFile(t, "owner@example.com"),
		FleetIDPath:           writeTestFile(t, "dc719d8e"),
		FleetSecretPath:       writeTestFile(t, "secret-value"),
		StateDirectory:        filepath.Join(rootPath, "state"),
		AttendanceDatabasePath: filepath.Join(
			rootPath,
			"attendance.sqlite",
		),
		AdminUIPath: t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" {
			return jsonResponse(
				http.StatusOK,
				`{"users":["member@example.com","operator@example.com","admin@example.com"],"records":[{"email":"member@example.com","role":"member"},{"email":"operator@example.com","role":"operationsAdmin"},{"email":"admin@example.com","role":"admin"}]}`,
				nil,
			), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	return service
}
