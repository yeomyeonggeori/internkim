package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const attendanceCarryRecoveryAction = "attendance-carry-into-the-record"
const attendanceCarryTimeout = 5 * time.Second

// A device that names no company covers nothing, so it counts everything it
// holds as uncovered and keeps its store. Reading this the other way round is
// how a two-row store gets dropped by a device the record has never heard of.
func (service *Service) sweepTheAttendanceTheCompanyNowHolds(ctx context.Context) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return
	}
	defer database.Close()

	if errorValue := dropAttendanceTables(ctx, database, derivedAttendanceTables); errorValue != nil {
		slog.WarnContext(ctx, "a derived attendance table could not be dropped", "error", errorValue)
		return
	}

	pinned := attendanceTablesTheRecordCannotTake(ctx, database)
	if len(pinned) > 0 {
		slog.WarnContext(ctx, "this device holds attendance the record has no place for, so its store stays",
			"tables", strings.Join(pinned, ","), "recovery_action", attendanceCarryRecoveryAction)
		return
	}

	uncovered, errorValue := service.attendanceTheRecordDoesNotHold(ctx, database)
	if errorValue != nil {
		slog.WarnContext(ctx, "the attendance this device still holds could not be counted, so none of it was let go",
			"error", errorValue)
		return
	}
	if uncovered > 0 {
		slog.WarnContext(ctx, "this device holds attendance the record does not, so its store stays",
			"uncovered", uncovered, "recovery_action", attendanceCarryRecoveryAction)
		return
	}
	if errorValue := dropAttendanceTables(ctx, database, carriedAttendanceTables); errorValue != nil {
		slog.WarnContext(ctx, "an attendance table the company now holds could not be dropped", "error", errorValue)
	}
}

func attendanceTablesTheRecordCannotTake(ctx context.Context, database *sql.DB) []string {
	pinned := []string{}
	for _, tableName := range sortedAttendanceTableNames() {
		rowCount, held := countRowsInAttendanceTable(ctx, database, tableName)
		if held && rowCount > 0 {
			pinned = append(pinned, tableName+" ("+attendanceTablesWithNowhereToGo[tableName]+")")
		}
	}
	return pinned
}

func sortedAttendanceTableNames() []string {
	names := make([]string, 0, len(attendanceTablesWithNowhereToGo))
	for tableName := range attendanceTablesWithNowhereToGo {
		names = append(names, tableName)
	}
	slices.Sort(names)
	return names
}

// Every clock and every leave this device holds that the record has not taken.
// A carried row is remembered by id, so asking again after a carry answers zero.
func (service *Service) attendanceTheRecordDoesNotHold(ctx context.Context, database *sql.DB) (int, error) {
	if service.centralPlane() == nil {
		clocks, errorValue := countRowsOrZero(ctx, database, "attendance_events")
		if errorValue != nil {
			return 0, errorValue
		}
		leaves, errorValue := countRowsOrZero(ctx, database, "attendance_leave_requests")
		if errorValue != nil {
			return 0, errorValue
		}
		return clocks + leaves, nil
	}
	return service.countUncarriedAttendance(ctx, database)
}

func countRowsOrZero(ctx context.Context, database *sql.DB, tableName string) (int, error) {
	rowCount, held := countRowsInAttendanceTable(ctx, database, tableName)
	if !held {
		return 0, nil
	}
	return rowCount, nil
}

func (service *Service) countUncarriedAttendance(ctx context.Context, database *sql.DB) (int, error) {
	clocks, errorValue := service.uncarriedClocks(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	leaves, errorValue := service.uncarriedLeaves(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	return len(clocks) + len(leaves), nil
}

type deviceClock struct {
	ID         string
	Email      string
	Kind       string
	Location   string
	OccurredAt time.Time
}

// An override replaces the moment a clock happened, so the carry takes the
// moment the device last agreed on rather than the one it first wrote.
const deviceClockQuery = `
SELECT events.id, events.email, events.kind, events.location_name,
	COALESCE(NULLIF(overrides.override_occurred_at, ''), events.occurred_at)
FROM attendance_events AS events
LEFT JOIN attendance_event_overrides AS overrides ON overrides.event_id = events.id
WHERE events.canceled_at = ''
ORDER BY events.occurred_at ASC`

func (service *Service) uncarriedClocks(ctx context.Context, database *sql.DB) ([]deviceClock, error) {
	alreadyCarried, errorValue := carriedRowIDs(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, held := countRowsInAttendanceTable(ctx, database, "attendance_events"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, deviceClockQuery)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	clocks := []deviceClock{}
	for rows.Next() {
		var clock deviceClock
		var occurredAt string
		if errorValue := rows.Scan(&clock.ID, &clock.Email, &clock.Kind, &clock.Location, &occurredAt); errorValue != nil {
			return nil, errorValue
		}
		moment, parseError := time.Parse(time.RFC3339, occurredAt)
		if parseError != nil || strings.TrimSpace(clock.Email) == "" {
			continue
		}
		if _, carried := alreadyCarried[clock.ID]; carried {
			continue
		}
		clock.OccurredAt = moment
		clocks = append(clocks, clock)
	}
	return clocks, rows.Err()
}

type deviceLeave struct {
	ID          string
	Email       string
	Kind        string
	BalanceMode string
	Status      string
	StartDate   string
	EndDate     string
	Days        float64
	Note        string
}

const deviceLeaveQuery = `
SELECT id, employee_email, leave_type_id, balance_mode, status, start_date, end_date,
	total_deduction_milli_days, reason
FROM attendance_leave_requests
WHERE status IN ('pending', 'approved', 'rejected')
ORDER BY start_date ASC`

func (service *Service) uncarriedLeaves(ctx context.Context, database *sql.DB) ([]deviceLeave, error) {
	alreadyCarried, errorValue := carriedRowIDs(ctx, database)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, held := countRowsInAttendanceTable(ctx, database, "attendance_leave_requests"); !held {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, deviceLeaveQuery)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	leaves := []deviceLeave{}
	for rows.Next() {
		var leave deviceLeave
		var milliDays int
		if errorValue := rows.Scan(&leave.ID, &leave.Email, &leave.Kind, &leave.BalanceMode,
			&leave.Status, &leave.StartDate, &leave.EndDate, &milliDays, &leave.Note); errorValue != nil {
			return nil, errorValue
		}
		if strings.TrimSpace(leave.Email) == "" {
			continue
		}
		if _, carried := alreadyCarried[leave.ID]; carried {
			continue
		}
		leave.Days = float64(milliDays) / 1000
		leaves = append(leaves, leave)
	}
	return leaves, rows.Err()
}

type attendanceCarryReport struct {
	Clocks  int      `json:"clocks"`
	Leaves  int      `json:"leaves"`
	Refused []string `json:"refused"`
}

// A row the record refuses is reported, not reshaped: it stays here, it is
// named, and the tables stay with it.
func (service *Service) carryAttendanceIntoTheRecord(ctx context.Context) (attendanceCarryReport, error) {
	report := attendanceCarryReport{Refused: []string{}}
	client := service.centralPlane()
	if client == nil {
		return report, fmt.Errorf("this device names no company to carry its attendance into")
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return report, errorValue
	}
	defer database.Close()

	clocks, errorValue := service.uncarriedClocks(ctx, database)
	if errorValue != nil {
		return report, errorValue
	}
	for _, clock := range clocks {
		carryContext, cancel := context.WithTimeout(ctx, attendanceCarryTimeout)
		errorValue := client.CarryClock(carryContext, centralplane.CarriedClock{
			Email:      clock.Email,
			Kind:       clock.Kind,
			Location:   clock.Location,
			OccurredAt: clock.OccurredAt,
		})
		cancel()
		if errorValue != nil {
			report.Refused = append(report.Refused, clock.ID+": "+errorValue.Error())
			continue
		}
		rememberCarriedRow(ctx, database, clock.ID)
		report.Clocks++
	}

	administratorEmail := service.claimedAdminEmail()
	paidByLeaveType, errorValue := service.paidByLeaveType()
	if errorValue != nil {
		return report, errorValue
	}
	leaves, errorValue := service.uncarriedLeaves(ctx, database)
	if errorValue != nil {
		return report, errorValue
	}
	if len(leaves) > 0 && administratorEmail == "" {
		return report, fmt.Errorf("no administrator is claimed here, and a decided leave is an administrator's to record")
	}
	for _, leave := range leaves {
		carried, errorValue := carriedLeaveOf(leave, paidByLeaveType)
		if errorValue != nil {
			report.Refused = append(report.Refused, leave.ID+": "+errorValue.Error())
			continue
		}
		carryContext, cancel := context.WithTimeout(ctx, attendanceCarryTimeout)
		errorValue = client.CarryLeave(carryContext, administratorEmail, carried)
		cancel()
		if errorValue != nil {
			report.Refused = append(report.Refused, leave.ID+": "+errorValue.Error())
			continue
		}
		rememberCarriedRow(ctx, database, leave.ID)
		report.Leaves++
	}
	return report, nil
}

// What the record took, kept against the local id so a second sweep does not
// count a carried row as still missing. The table is retired with the store it
// describes, so it never outlives what it is about.
func carriedRowIDs(ctx context.Context, database *sql.DB) (map[string]struct{}, error) {
	carried := map[string]struct{}{}
	if _, held := countRowsInAttendanceTable(ctx, database, "attendance_carried_rows"); !held {
		return carried, nil
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT id FROM attendance_carried_rows")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if errorValue := rows.Scan(&id); errorValue != nil {
			return nil, errorValue
		}
		carried[id] = struct{}{}
	}
	return carried, rows.Err()
}

func rememberCarriedRow(ctx context.Context, database *sql.DB, id string) {
	if _, errorValue := database.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS attendance_carried_rows (id TEXT PRIMARY KEY)"); errorValue != nil {
		return
	}
	database.ExecContext(ctx, "INSERT OR IGNORE INTO attendance_carried_rows (id) VALUES (?)", id)
}

var recordLeaveStatus = map[string]string{
	"pending":  "requested",
	"approved": "approved",
	"rejected": "rejected",
}

func carriedLeaveOf(leave deviceLeave, paidByLeaveType map[string]bool) (centralplane.CarriedLeave, error) {
	startsAt, errorValue := time.Parse(time.DateOnly, leave.StartDate)
	if errorValue != nil {
		return centralplane.CarriedLeave{}, fmt.Errorf("start date %q is not a date", leave.StartDate)
	}
	lastDay := leave.EndDate
	if strings.TrimSpace(lastDay) == "" {
		lastDay = leave.StartDate
	}
	endsAt, errorValue := time.Parse(time.DateOnly, lastDay)
	if errorValue != nil {
		return centralplane.CarriedLeave{}, fmt.Errorf("end date %q is not a date", lastDay)
	}
	status, known := recordLeaveStatus[leave.Status]
	if !known {
		return centralplane.CarriedLeave{}, fmt.Errorf("status %q is not one the record keeps", leave.Status)
	}
	isPaid, described := paidByLeaveType[leave.Kind]
	if !described {
		return centralplane.CarriedLeave{}, fmt.Errorf("this device's leave policy does not describe %q, so whether it was paid is not known here", leave.Kind)
	}
	return centralplane.CarriedLeave{
		Email:      leave.Email,
		Kind:       leave.Kind,
		IsPaid:     isPaid,
		IsDeducted: leave.BalanceMode == "annual",
		Days:       leave.Days,
		Status:     status,
		StartsAt:   startsAt,
		EndsAt:     endsAt.AddDate(0, 0, 1),
		Note:       leave.Note,
	}, nil
}

type attendanceRecordCoverage struct {
	Clocks  int      `json:"clocks"`
	Leaves  int      `json:"leaves"`
	Pinned  []string `json:"pinned"`
	Carried int      `json:"carried"`
	Refused []string `json:"refused"`
}

func (service *Service) handleAttendanceRecordCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	coverage, errorValue := service.attendanceCoverageOfTheRecord(request)
	if errorValue != nil {
		log.Printf("attendance coverage failed: %v", errorValue)
		http.Error(responseWriter, "attendance_coverage_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, coverage)
}

func (service *Service) attendanceCoverageOfTheRecord(request *http.Request) (attendanceRecordCoverage, error) {
	coverage := attendanceRecordCoverage{Pinned: []string{}, Refused: []string{}}
	database, errorValue := service.openAttendanceDatabase(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	clocks, errorValue := service.uncarriedClocks(request.Context(), database)
	if errorValue != nil {
		database.Close()
		return coverage, errorValue
	}
	leaves, errorValue := service.uncarriedLeaves(request.Context(), database)
	if errorValue != nil {
		database.Close()
		return coverage, errorValue
	}
	coverage.Clocks = len(clocks)
	coverage.Leaves = len(leaves)
	coverage.Pinned = attendanceTablesTheRecordCannotTake(request.Context(), database)
	database.Close()

	if request.URL.Query().Get("carry") != "true" {
		return coverage, nil
	}
	report, errorValue := service.carryAttendanceIntoTheRecord(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	coverage.Carried = report.Clocks + report.Leaves
	coverage.Refused = report.Refused
	return coverage, nil
}

func (service *Service) paidByLeaveType() (map[string]bool, error) {
	path := filepath.Join(service.Configuration.StateDirectory, "attendance-settings.json")
	document, errorValue := os.ReadFile(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if errorValue != nil {
		return nil, fmt.Errorf("read attendance settings %q: %w", path, errorValue)
	}
	var settings struct {
		LeavePolicy struct {
			LeaveTypes []struct {
				ID   string `json:"id"`
				Paid bool   `json:"paid"`
			} `json:"leaveTypes"`
		} `json:"leavePolicy"`
	}
	if errorValue := json.Unmarshal(document, &settings); errorValue != nil {
		return nil, fmt.Errorf("decode attendance settings %q: %w", path, errorValue)
	}
	paid := map[string]bool{}
	for _, leaveType := range settings.LeavePolicy.LeaveTypes {
		paid[leaveType.ID] = leaveType.Paid
	}
	return paid, nil
}
