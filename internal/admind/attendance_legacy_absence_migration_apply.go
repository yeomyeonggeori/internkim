package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (service *Service) applyAttendanceLegacyAbsenceMigration(
	ctx context.Context,
	actorEmail string,
	expectedFingerprint string,
	employeeUserIDs map[string]string,
	now time.Time,
) (attendanceLegacyAbsenceMigrationBatch, error) {
	expectedFingerprint = strings.TrimSpace(expectedFingerprint)
	if expectedFingerprint == "" {
		return attendanceLegacyAbsenceMigrationBatch{}, attendanceLeaveInvalidInputErrorf(
			"migration preview fingerprint is required",
		)
	}
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
	preview, errorValue := readAttendanceLegacyAbsenceMigrationPreview(
		ctx,
		transaction,
		employeeUserIDs,
	)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	if preview.Fingerprint != expectedFingerprint {
		return attendanceLegacyAbsenceMigrationBatch{}, attendanceLeaveInvalidInputErrorf(
			"legacy absence migration preview is stale",
		)
	}
	if preview.ConflictCount > 0 {
		return attendanceLegacyAbsenceMigrationBatch{}, attendanceLeaveInvalidInputErrorf(
			"legacy absence migration has %d conflicts",
			preview.ConflictCount,
		)
	}
	if preview.CandidateLeaveCount == 0 {
		return attendanceLegacyAbsenceMigrationBatch{
			Status:              attendanceLegacyMigrationApplied,
			PreservedOtherCount: preview.PreservedOtherCount,
		}, nil
	}
	batchToken, errorValue := generateRandomURLToken(12)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	timestamp := now.UTC().Format(time.RFC3339Nano)
	batch := attendanceLegacyAbsenceMigrationBatch{
		ID:                  "legacy-absence-" + batchToken,
		Status:              attendanceLegacyMigrationApplied,
		LeaveCount:          preview.CandidateLeaveCount,
		PreservedOtherCount: preview.PreservedOtherCount,
		CreatedAt:           timestamp,
	}
	if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_legacy_absence_migration_batches (
	id, status, actor_email, leave_count, other_count, created_at, rolled_back_at
) VALUES (?, ?, ?, ?, ?, ?, '')`,
		batch.ID,
		batch.Status,
		normalizeAttendanceLeaveEmail(actorEmail),
		batch.LeaveCount,
		batch.PreservedOtherCount,
		batch.CreatedAt,
	); errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	ranges, errorValue := readAttendanceLegacyAbsenceRanges(
		ctx,
		transaction,
		employeeUserIDs,
	)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	for _, absenceRange := range ranges {
		if absenceRange.Status != "candidate" {
			continue
		}
		requestID, migrateError := service.migrateAttendanceLegacyAbsenceRange(
			ctx,
			transaction,
			absenceRange,
			actorEmail,
			timestamp,
		)
		if migrateError != nil {
			return attendanceLegacyAbsenceMigrationBatch{}, migrateError
		}
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_legacy_absence_migration_items (
	batch_id, range_id, request_id, migrated_at, rolled_back_at
) VALUES (?, ?, ?, ?, '')`,
			batch.ID,
			absenceRange.ID,
			requestID,
			timestamp,
		); errorValue != nil {
			return attendanceLegacyAbsenceMigrationBatch{}, errorValue
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLegacyAbsenceMigrationBatch{}, errorValue
	}
	return batch, nil
}

func (service *Service) migrateAttendanceLegacyAbsenceRange(
	ctx context.Context,
	transaction *sql.Tx,
	absenceRange attendanceLegacyAbsenceRange,
	actorEmail string,
	fallbackTimestamp string,
) (string, error) {
	schedule := defaultAttendanceWorkSchedule()
	location := service.workspaceTimeZone().location
	occurrences := make([]attendanceLeaveRequestOccurrence, 0, len(absenceRange.Dates))
	for _, date := range absenceRange.Dates {
		occurrence, errorValue := attendanceLeaveRequestOccurrenceForDate(
			schedule,
			attendanceLeaveRequestInput{Unit: attendanceWorkScheduleFullDay},
			date,
			location,
		)
		if errorValue != nil {
			return "", fmt.Errorf("migrate legacy absence range %s: %w", absenceRange.ID, errorValue)
		}
		occurrences = append(occurrences, occurrence)
	}
	requestID := attendanceLegacyAbsenceRequestID(absenceRange.ID)
	createdAt := firstNonEmpty(strings.TrimSpace(absenceRange.CreatedAt), fallbackTimestamp)
	updatedAt := firstNonEmpty(strings.TrimSpace(absenceRange.UpdatedAt), createdAt)
	record := attendanceLeaveRequestRecord{
		ID:                      requestID,
		EmployeeEmail:           absenceRange.Email,
		UserID:                  absenceRange.UserID,
		LeaveTypeID:             attendanceLegacyLeaveTypeID,
		LeaveTypeName:           attendanceLegacyLeaveTypeName,
		BalanceMode:             "none",
		Status:                  attendanceLeaveRequestStatusApproved,
		Unit:                    attendanceWorkScheduleFullDay,
		StartDate:               absenceRange.StartDate,
		EndDate:                 absenceRange.EndDate,
		Reason:                  strings.TrimSpace(absenceRange.Reason),
		TotalDeductionMilliDays: len(occurrences) * 1000,
		Revision:                1,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		Occurrences:             occurrences,
	}
	if errorValue := insertAttendanceLeaveRequestRecord(
		ctx,
		transaction,
		record,
		service.workspaceTimeZone().name,
	); errorValue != nil {
		return "", errorValue
	}
	eventActor := normalizeAttendanceLeaveEmail(
		firstNonEmpty(absenceRange.CreatedBy, actorEmail),
	)
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_request_events
SET kind = 'legacyMigrated', actor_email = ?
WHERE request_id = ?`,
		eventActor,
		requestID,
	); errorValue != nil {
		return "", errorValue
	}
	if errorValue := linkAttendanceLeaveRequestAbsenceRange(
		ctx,
		transaction,
		requestID,
		absenceRange.ID,
	); errorValue != nil {
		return "", errorValue
	}
	if _, errorValue := recordUntrackedAttendanceLeaveUseInTransaction(
		ctx,
		transaction,
		attendanceLeaveOperation{
			OperationKey: attendanceLegacyAbsenceUseOperationKey(requestID),
			Employee: attendanceLeaveEmployee{
				Email:  absenceRange.Email,
				UserID: absenceRange.UserID,
			},
			LeaveTypeID: attendanceLegacyLeaveTypeID,
			ReferenceID: requestID,
			Amount:      record.TotalDeductionMilliDays,
			EffectiveOn: absenceRange.StartDate,
		},
	); errorValue != nil {
		return "", errorValue
	}
	return requestID, nil
}

func attendanceLegacyAbsenceUseOperationKey(requestID string) string {
	return "leave-request:" + requestID + ":legacy-use:1"
}
