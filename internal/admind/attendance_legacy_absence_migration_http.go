package admind

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type attendanceLegacyAbsenceMigrationApplyInput struct {
	Fingerprint string `json:"fingerprint"`
}

type attendanceLegacyAbsenceMigrationRollbackInput struct {
	BatchID string `json:"batchID"`
}

func (service *Service) writeAttendanceLegacyAbsenceMigrationPreview(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	employeeUserIDs, errorValue := service.attendanceLegacyAbsenceMigrationEmployeeUserIDs(request)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	preview, errorValue := service.previewAttendanceLegacyAbsenceMigration(
		request.Context(),
		employeeUserIDs,
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, preview)
}

func (service *Service) writeAttendanceLegacyAbsenceMigrationApply(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	var input attendanceLegacyAbsenceMigrationApplyInput
	if errorValue := decodeAttendanceLeaveManagementJSON(request, &input); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	input.Fingerprint = strings.TrimSpace(input.Fingerprint)
	if input.Fingerprint == "" {
		writeAttendanceLeaveRequestError(
			responseWriter,
			attendanceLeaveInvalidInputErrorf("migration preview fingerprint is required"),
		)
		return
	}
	employeeUserIDs, errorValue := service.attendanceLegacyAbsenceMigrationEmployeeUserIDs(request)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	batch, errorValue := service.applyAttendanceLegacyAbsenceMigration(
		request.Context(),
		service.webStaffActorEmail(request),
		input.Fingerprint,
		employeeUserIDs,
		time.Now(),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]attendanceLegacyAbsenceMigrationBatch{
		"batch": batch,
	})
}

func (service *Service) attendanceLegacyAbsenceMigrationEmployeeUserIDs(
	request *http.Request,
) (map[string]string, error) {
	records, found := service.attendanceUserRecordsForMembers(request)
	if !found {
		return nil, fmt.Errorf("load account directory for legacy absence migration")
	}
	userIDs := map[string]string{}
	for _, record := range records {
		email := normalizeAttendanceLeaveEmail(record.Email)
		userID := strings.TrimSpace(record.UserID)
		if email == "" || userID == "" {
			continue
		}
		existingUserID, exists := userIDs[email]
		if exists && existingUserID != userID {
			userIDs[email] = ""
			continue
		}
		userIDs[email] = userID
	}
	return userIDs, nil
}

func (service *Service) writeAttendanceLegacyAbsenceMigrationRollback(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	var input attendanceLegacyAbsenceMigrationRollbackInput
	if errorValue := decodeAttendanceLeaveManagementJSON(request, &input); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	input.BatchID = strings.TrimSpace(input.BatchID)
	if input.BatchID == "" {
		writeAttendanceLeaveRequestError(
			responseWriter,
			attendanceLeaveInvalidInputErrorf("migration batch ID is required"),
		)
		return
	}
	batch, errorValue := service.rollbackAttendanceLegacyAbsenceMigration(
		request.Context(),
		input.BatchID,
		time.Now(),
	)
	if errorValue != nil {
		if errorValue == errAttendanceLegacyMigrationBatchNotFound ||
			errorValue == errAttendanceLegacyMigrationBatchRolledBack {
			errorValue = attendanceLeaveInvalidInputError(errorValue)
		}
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]attendanceLegacyAbsenceMigrationBatch{
		"batch": batch,
	})
}
