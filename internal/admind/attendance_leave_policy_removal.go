package admind

import (
	"context"
	"database/sql"
	"fmt"
)

func (service *Service) preserveUsedRemovedAttendanceLeaveTypes(
	ctx context.Context,
	existing attendanceLeavePolicy,
	updated *attendanceLeavePolicy,
) error {
	updatedByID := make(map[string]bool, len(updated.LeaveTypes))
	for _, leaveType := range updated.LeaveTypes {
		updatedByID[leaveType.ID] = true
	}
	removed := make([]attendanceLeaveType, 0)
	for _, leaveType := range existing.LeaveTypes {
		if !updatedByID[leaveType.ID] {
			removed = append(removed, leaveType)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	for _, leaveType := range removed {
		used, usedError := attendanceLeaveTypeHasHistory(ctx, database, leaveType.ID)
		if usedError != nil {
			return usedError
		}
		if !used {
			continue
		}
		leaveType.IsActive = false
		leaveType.IncludeInSummary = false
		leaveType.SortOrder = len(updated.LeaveTypes)
		updated.LeaveTypes = append(updated.LeaveTypes, leaveType)
	}
	return nil
}

func attendanceLeaveTypeHasHistory(
	ctx context.Context,
	database *sql.DB,
	leaveTypeID string,
) (bool, error) {
	var hasHistory int
	errorValue := database.QueryRowContext(ctx, `
SELECT (
	EXISTS (
		SELECT 1
		FROM attendance_leave_requests
		WHERE leave_type_id = ?
		LIMIT 1
	)
	OR EXISTS (
		SELECT 1
		FROM attendance_leave_operations
		WHERE leave_type_id = ?
		LIMIT 1
	)
)`,
		leaveTypeID,
		leaveTypeID,
	).Scan(&hasHistory)
	if errorValue != nil {
		return false, fmt.Errorf("check leave type %q history: %w", leaveTypeID, errorValue)
	}
	return hasHistory != 0, nil
}
