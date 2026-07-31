package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errAttendanceLegacyMigrationBatchNotFound = errors.New("legacy absence migration batch not found")
var errAttendanceLegacyMigrationBatchRolledBack = errors.New("legacy absence migration batch is already rolled back")

func (service *Service) rollbackAttendanceLegacyAbsenceMigration(
	ctx context.Context,
	batchID string,
	now time.Time,
) (attendanceLegacyAbsenceMigrationBatch, error) {
	service.attendanceLeavePolicyMutationMutex.Lock()
	defer service.attendanceLeavePolicyMutationMutex.Unlock()
	database, errorValue := service.openAttendanceLegacyAbsenceMigrationDatabase(ctx, true)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	defer transaction.Rollback()
	batch, errorValue := readAttendanceLegacyAbsenceMigrationBatch(
		ctx,
		transaction,
		strings.TrimSpace(batchID),
	)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	if batch.Status == attendanceLegacyMigrationRolled {
		return attendanceLegacyAbsenceMigrationBatch{}, errAttendanceLegacyMigrationBatchRolledBack
	}
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT request_id
FROM attendance_legacy_absence_migration_items
WHERE batch_id = ? AND rolled_back_at = ''
ORDER BY range_id`,
		batch.ID,
	)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	requestIDs := []string{}
	for rows.Next() {
		var requestID string
		if errorValue := rows.Scan(&requestID); errorValue != nil {
			rows.Close()
			return attendanceLegacyAbsenceMigrationBatch{}, errorValue
		}
		requestIDs = append(requestIDs, requestID)
	}
	if errorValue := rows.Close(); errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	for _, requestID := range requestIDs {
		operationKey := attendanceLegacyAbsenceUseOperationKey(requestID)
		if _, errorValue := transaction.ExecContext(ctx, `
DELETE FROM attendance_leave_ledger_entries
WHERE operation_key = ?`,
			operationKey,
		); errorValue != nil {
			return attendanceLegacyAbsenceMigrationBatch{}, errorValue
		}
		operationResult, errorValue := transaction.ExecContext(ctx, `
DELETE FROM attendance_leave_operations
WHERE operation_key = ?
	AND kind = ?
	AND reference_id = ?
	AND leave_type_id = ?`,
			operationKey,
			attendanceLeaveOperationUntrackedUse,
			requestID,
			attendanceAnnualLeaveTypeID,
		)
		if errorValue != nil {
			return attendanceLegacyAbsenceMigrationBatch{}, errorValue
		}
		deletedOperationCount, errorValue := operationResult.RowsAffected()
		if errorValue != nil {
			return attendanceLegacyAbsenceMigrationBatch{}, errorValue
		}
		if deletedOperationCount != 1 {
			return attendanceLegacyAbsenceMigrationBatch{}, fmt.Errorf(
				"legacy migration leave operation %s is missing during rollback",
				operationKey,
			)
		}
		result, errorValue := transaction.ExecContext(ctx, `
DELETE FROM attendance_leave_requests
WHERE id = ? AND leave_type_id = ?`,
			requestID,
			attendanceAnnualLeaveTypeID,
		)
		if errorValue != nil {
			return attendanceLegacyAbsenceMigrationBatch{}, errorValue
		}
		deletedCount, errorValue := result.RowsAffected()
		if errorValue != nil {
			return attendanceLegacyAbsenceMigrationBatch{}, errorValue
		}
		if deletedCount != 1 {
			return attendanceLegacyAbsenceMigrationBatch{}, fmt.Errorf(
				"legacy migration request %s is missing during rollback",
				requestID,
			)
		}
	}
	rolledBackAt := now.UTC().Format(time.RFC3339Nano)
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_legacy_absence_migration_items
SET rolled_back_at = ?
WHERE batch_id = ? AND rolled_back_at = ''`,
		rolledBackAt,
		batch.ID,
	); errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_legacy_absence_migration_batches
SET status = ?, rolled_back_at = ?
WHERE id = ?`,
		attendanceLegacyMigrationRolled,
		rolledBackAt,
		batch.ID,
	); errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	batch.Status = attendanceLegacyMigrationRolled
	batch.RolledBackAt = rolledBackAt
	return batch, nil
}

func readAttendanceLegacyAbsenceMigrationBatch(
	ctx context.Context,
	queryer attendanceLegacyAbsenceQueryer,
	batchID string,
) (attendanceLegacyAbsenceMigrationBatch, error) {
	var batch attendanceLegacyAbsenceMigrationBatch
	errorValue := queryer.QueryRowContext(ctx, `
SELECT id, status, leave_count, other_count, created_at, rolled_back_at
FROM attendance_legacy_absence_migration_batches
WHERE id = ?`,
		batchID,
	).Scan(
		&batch.ID,
		&batch.Status,
		&batch.LeaveCount,
		&batch.PreservedOtherCount,
		&batch.CreatedAt,
		&batch.RolledBackAt,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceLegacyAbsenceMigrationBatch{}, errAttendanceLegacyMigrationBatchNotFound
	}
	return batch, errorValue
}
