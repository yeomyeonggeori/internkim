package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAttendanceLegacyAbsenceMigrationPreservesSourceAndSupportsRollback(
	t *testing.T,
) {
	service, _ := newAttendanceActionTestService(t)
	ctx := t.Context()
	employeeUserIDs := attendanceLegacyAbsenceMigrationTestEmployeeUserIDs()
	if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-legacy-migration",
			Employee: attendanceLeaveEmployee{
				Email:  "staff@example.com",
				UserID: "user-1",
			},
			LeaveTypeID: attendanceAnnualLeaveTypeID,
			Amount:      3000,
			EffectiveOn: "2026-01-01",
		},
		ExpiresOn: "2027-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	leaveRangeID := insertLegacyAttendanceAbsenceForMigration(
		t,
		service,
		"staff@example.com",
		attendanceAbsenceLeave,
		"2026-07-20",
		"2026-07-21",
		"private legacy reason",
	)
	insertLegacyAttendanceAbsenceForMigration(
		t,
		service,
		"staff@example.com",
		attendanceAbsenceOther,
		"2026-07-22",
		"2026-07-22",
		"other absence",
	)
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, tableName := range []string{
		"attendance_legacy_absence_migration_batches",
		"attendance_legacy_absence_migration_items",
	} {
		if attendanceTableExists(t, database, tableName) {
			t.Fatalf("migration table %s exists before explicit preview", tableName)
		}
	}
	initialRequestCount := attendanceMigrationTableCount(t, database, "attendance_leave_requests")
	initialOperationCount := attendanceMigrationTableCount(t, database, "attendance_leave_operations")
	database.Close()

	preview, errorValue := service.previewAttendanceLegacyAbsenceMigration(ctx, employeeUserIDs)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if preview.CandidateLeaveCount != 1 ||
		preview.CandidateOccurrenceCount != 2 ||
		preview.PreservedOtherCount != 1 ||
		preview.AlreadyMigratedCount != 0 ||
		preview.ConflictCount != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	if len(preview.Items) != 2 ||
		preview.Items[0].Status != "candidate" ||
		preview.Items[0].UserID != "user-1" ||
		preview.Items[1].Status != "preserved" ||
		preview.Items[1].OriginalKind != attendanceAbsenceOther {
		t.Fatalf("preview items = %+v", preview.Items)
	}
	if !strings.HasPrefix(preview.Fingerprint, "sha256:") {
		t.Fatalf("preview fingerprint = %q", preview.Fingerprint)
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requestCount := attendanceMigrationTableCount(t, database, "attendance_leave_requests"); requestCount != initialRequestCount {
		t.Fatalf("dry-run request count = %d, want %d", requestCount, initialRequestCount)
	}
	for _, tableName := range []string{
		"attendance_legacy_absence_migration_batches",
		"attendance_legacy_absence_migration_items",
	} {
		if !attendanceTableExists(t, database, tableName) {
			t.Fatalf("migration table %s missing after explicit preview", tableName)
		}
	}
	database.Close()

	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	batch, errorValue := service.applyAttendanceLegacyAbsenceMigration(
		ctx,
		"admin@example.com",
		preview.Fingerprint,
		employeeUserIDs,
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if batch.ID == "" ||
		batch.Status != attendanceLegacyMigrationApplied ||
		batch.LeaveCount != 1 ||
		batch.PreservedOtherCount != 1 {
		t.Fatalf("batch = %+v", batch)
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	requestID := attendanceLegacyAbsenceRequestID(leaveRangeID)
	var leaveTypeID string
	var leaveTypeName string
	var userID string
	var balanceMode string
	var status string
	var reason string
	var totalDeduction int
	if errorValue := database.QueryRowContext(ctx, `
SELECT leave_type_id, leave_type_name, user_id, balance_mode, status, reason, total_deduction_milli_days
FROM attendance_leave_requests
WHERE id = ?`,
		requestID,
	).Scan(
		&leaveTypeID,
		&leaveTypeName,
		&userID,
		&balanceMode,
		&status,
		&reason,
		&totalDeduction,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	if leaveTypeID != attendanceAnnualLeaveTypeID ||
		leaveTypeName != "연차" ||
		userID != "user-1" ||
		balanceMode != "none" ||
		status != attendanceLeaveRequestStatusApproved ||
		reason != "private legacy reason" ||
		totalDeduction != 2000 {
		t.Fatalf(
			"migrated request type=%q name=%q user=%q balance=%q status=%q reason=%q deduction=%d",
			leaveTypeID,
			leaveTypeName,
			userID,
			balanceMode,
			status,
			reason,
			totalDeduction,
		)
	}
	if occurrenceCount := attendanceMigrationRequestOccurrenceCount(t, database, requestID); occurrenceCount != 2 {
		t.Fatalf("occurrence count = %d, want 2", occurrenceCount)
	}
	if operationCount := attendanceMigrationTableCount(t, database, "attendance_leave_operations"); operationCount != initialOperationCount+1 {
		t.Fatalf("leave operation count = %d, want %d", operationCount, initialOperationCount+1)
	}
	annualBalance, errorValue := queryAttendanceLeaveBalance(
		ctx,
		database,
		attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"},
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if annualBalance.AvailableMilliDays != 3000 ||
		annualBalance.ReservedMilliDays != 0 ||
		annualBalance.UsedMilliDays != 2000 {
		t.Fatalf("annual balance = %+v", annualBalance)
	}
	var linkedRangeID string
	if errorValue := database.QueryRowContext(ctx, `
SELECT range_id
FROM attendance_leave_request_absence_ranges
WHERE request_id = ?`,
		requestID,
	).Scan(&linkedRangeID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if linkedRangeID != leaveRangeID {
		t.Fatalf("linked range ID = %q, want %q", linkedRangeID, leaveRangeID)
	}
	database.Close()
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2099-01-01",
	}
	dashboard, errorValue := service.readAttendanceLeaveDashboard(ctx, employee, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if dashboard.Summary.UsedMilliDays != 2000 ||
		dashboard.Summary.AvailableMilliDays != 1000 {
		t.Fatalf("migrated dashboard summary = %+v", dashboard.Summary)
	}
	policy := defaultAttendanceLeavePolicy()
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	if errorValue := service.writeAttendanceLeavePolicy(ctx, policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	dashboard, errorValue = service.readAttendanceLeaveDashboard(ctx, employee, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if dashboard.Summary.UsedMilliDays != 2000 {
		t.Fatalf("used leave after policy update = %d, want 2000", dashboard.Summary.UsedMilliDays)
	}

	repeatedPreview, errorValue := service.previewAttendanceLegacyAbsenceMigration(
		ctx,
		employeeUserIDs,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	repeatedBatch, errorValue := service.applyAttendanceLegacyAbsenceMigration(
		ctx,
		"admin@example.com",
		repeatedPreview.Fingerprint,
		employeeUserIDs,
		now.Add(time.Minute),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if repeatedBatch.ID != "" || repeatedBatch.LeaveCount != 0 {
		t.Fatalf("repeated batch = %+v", repeatedBatch)
	}

	rolledBackBatch, errorValue := service.rollbackAttendanceLegacyAbsenceMigration(
		ctx,
		batch.ID,
		now.Add(2*time.Minute),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if rolledBackBatch.Status != attendanceLegacyMigrationRolled ||
		rolledBackBatch.RolledBackAt == "" {
		t.Fatalf("rolled back batch = %+v", rolledBackBatch)
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if requestCount := attendanceMigrationTableCount(t, database, "attendance_leave_requests"); requestCount != initialRequestCount {
		t.Fatalf("request count after rollback = %d, want %d", requestCount, initialRequestCount)
	}
	for _, tableName := range []string{
		"attendance_leave_request_occurrences",
		"attendance_leave_request_events",
		"attendance_leave_request_absence_ranges",
	} {
		if count := attendanceMigrationRequestReferenceCount(t, database, tableName, requestID); count != 0 {
			t.Fatalf("%s count after rollback = %d", tableName, count)
		}
	}
	if operationCount := attendanceMigrationTableCount(t, database, "attendance_leave_operations"); operationCount != initialOperationCount {
		t.Fatalf("operation count after rollback = %d, want %d", operationCount, initialOperationCount)
	}
	if sourceCount := attendanceMigrationTableCount(t, database, "attendance_absence_ranges"); sourceCount != 2 {
		t.Fatalf("source absence count after rollback = %d, want 2", sourceCount)
	}
	database.Close()
	dashboard, errorValue = service.readAttendanceLeaveDashboard(ctx, employee, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if dashboard.Summary.UsedMilliDays != 0 {
		t.Fatalf("used leave after rollback = %d, want 0", dashboard.Summary.UsedMilliDays)
	}

	preview, errorValue = service.previewAttendanceLegacyAbsenceMigration(ctx, employeeUserIDs)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if preview.CandidateLeaveCount != 1 || preview.PreservedOtherCount != 1 {
		t.Fatalf("preview after rollback = %+v", preview)
	}
	_, errorValue = service.rollbackAttendanceLegacyAbsenceMigration(
		ctx,
		batch.ID,
		now.Add(3*time.Minute),
	)
	if !errors.Is(errorValue, errAttendanceLegacyMigrationBatchRolledBack) {
		t.Fatalf("second rollback error = %v", errorValue)
	}
}

func TestAttendanceLegacyAbsenceMigrationRollsBackFailedApply(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := t.Context()
	employeeUserIDs := attendanceLegacyAbsenceMigrationTestEmployeeUserIDs()
	insertLegacyAttendanceAbsenceForMigration(
		t,
		service,
		"staff@example.com",
		attendanceAbsenceLeave,
		"2026-07-20",
		"2026-07-20",
		"legacy leave",
	)
	preview, errorValue := service.previewAttendanceLegacyAbsenceMigration(ctx, employeeUserIDs)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TRIGGER fail_legacy_absence_migration_item
BEFORE INSERT ON attendance_legacy_absence_migration_items
BEGIN
	SELECT RAISE(ABORT, 'forced legacy migration failure');
END`); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	_, errorValue = service.applyAttendanceLegacyAbsenceMigration(
		ctx,
		"admin@example.com",
		preview.Fingerprint,
		employeeUserIDs,
		time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC),
	)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced legacy migration failure") {
		t.Fatalf("apply error = %v", errorValue)
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if count := attendanceMigrationTableCount(t, database, "attendance_leave_requests"); count != 0 {
		t.Fatalf("request count after failed apply = %d", count)
	}
	if count := attendanceMigrationTableCount(t, database, "attendance_legacy_absence_migration_batches"); count != 0 {
		t.Fatalf("batch count after failed apply = %d", count)
	}
}

func TestAttendanceLegacyAbsenceMigrationRejectsStalePreview(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := t.Context()
	employeeUserIDs := attendanceLegacyAbsenceMigrationTestEmployeeUserIDs()
	insertLegacyAttendanceAbsenceForMigration(
		t,
		service,
		"staff@example.com",
		attendanceAbsenceLeave,
		"2026-07-20",
		"2026-07-20",
		"first legacy leave",
	)
	preview, errorValue := service.previewAttendanceLegacyAbsenceMigration(ctx, employeeUserIDs)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	insertLegacyAttendanceAbsenceForMigration(
		t,
		service,
		"staff@example.com",
		attendanceAbsenceLeave,
		"2026-07-21",
		"2026-07-21",
		"second legacy leave",
	)
	_, errorValue = service.applyAttendanceLegacyAbsenceMigration(
		ctx,
		"admin@example.com",
		preview.Fingerprint,
		employeeUserIDs,
		time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC),
	)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "preview is stale") {
		t.Fatalf("stale preview error = %v", errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if count := attendanceMigrationTableCount(t, database, "attendance_leave_requests"); count != 0 {
		t.Fatalf("request count after stale apply = %d", count)
	}
	if count := attendanceMigrationTableCount(t, database, "attendance_legacy_absence_migration_batches"); count != 0 {
		t.Fatalf("batch count after stale apply = %d", count)
	}
}

func TestAttendanceLegacyAbsenceMigrationReportsUnknownEmployeeConflict(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	insertLegacyAttendanceAbsenceForMigration(
		t,
		service,
		"unknown@example.com",
		attendanceAbsenceLeave,
		"2026-07-20",
		"2026-07-20",
		"unknown employee leave",
	)
	preview, errorValue := service.previewAttendanceLegacyAbsenceMigration(
		t.Context(),
		attendanceLegacyAbsenceMigrationTestEmployeeUserIDs(),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if preview.ConflictCount != 1 ||
		len(preview.Items) != 1 ||
		preview.Items[0].Status != "conflict" ||
		preview.Items[0].Issue != "employee is not in the account directory" {
		t.Fatalf("unknown employee preview = %+v", preview)
	}
}

func TestAttendanceLegacyAbsenceMigrationHTTPRequiresAdmin(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	request := httptest.NewRequest(
		http.MethodGet,
		"/attendance/api/leave-management/legacy-migration",
		nil,
	)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("staff preview status = %d body = %s", recorder.Code, recorder.Body.String())
	}

	adminRecorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodGet,
		"/attendance/api/leave-management/legacy-migration",
		"",
	)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("admin preview status = %d body = %s", adminRecorder.Code, adminRecorder.Body.String())
	}
	var preview attendanceLegacyAbsenceMigrationPreview
	if errorValue := json.NewDecoder(adminRecorder.Body).Decode(&preview); errorValue != nil {
		t.Fatal(errorValue)
	}
	applyBody, errorValue := json.Marshal(attendanceLegacyAbsenceMigrationApplyInput{
		Fingerprint: preview.Fingerprint,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	applyRecorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-management/legacy-migration/apply",
		string(applyBody),
	)
	if applyRecorder.Code != http.StatusOK {
		t.Fatalf("admin apply status = %d body = %s", applyRecorder.Code, applyRecorder.Body.String())
	}
}

func insertLegacyAttendanceAbsenceForMigration(
	t *testing.T,
	service *Service,
	email string,
	kind string,
	startDate string,
	endDate string,
	reason string,
) string {
	t.Helper()
	absences, errorValue := service.insertAttendanceAbsenceRange(
		t.Context(),
		email,
		kind,
		startDate,
		endDate,
		reason,
		"admin@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) == 0 {
		t.Fatal("expected inserted legacy absence")
	}
	return absences[0].RangeID
}

func attendanceMigrationTableCount(
	t *testing.T,
	database *sql.DB,
	tableName string,
) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM "+tableName,
	).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func attendanceMigrationRequestOccurrenceCount(
	t *testing.T,
	database *sql.DB,
	requestID string,
) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM attendance_leave_request_occurrences
WHERE request_id = ?`,
		requestID,
	).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func attendanceMigrationRequestReferenceCount(
	t *testing.T,
	database *sql.DB,
	tableName string,
	requestID string,
) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM "+tableName+" WHERE request_id = ?",
		requestID,
	).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func attendanceLegacyAbsenceMigrationTestEmployeeUserIDs() map[string]string {
	return map[string]string{
		"admin@example.com": "admin",
		"staff@example.com": "user-1",
	}
}
