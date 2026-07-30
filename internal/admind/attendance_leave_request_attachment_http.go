package admind

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (service *Service) writeAttendanceLeaveRequestAttachment(
	responseWriter http.ResponseWriter,
	request *http.Request,
	requestID string,
	attachmentID string,
) {
	employeeEmail := normalizeAttendanceLeaveEmail(service.webStaffActorEmail(request))
	isAdministrator := service.isAuthorized(request)
	database, errorValue := service.openAttendanceDatabase(request.Context())
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	defer database.Close()
	var attachment attendanceLeaveRequestAttachment
	errorValue = database.QueryRowContext(request.Context(), `
SELECT attachment.id, attachment.file_name, attachment.content_type, attachment.size_bytes, attachment.storage_key
FROM attendance_leave_request_attachments attachment
JOIN attendance_leave_requests request ON request.id = attachment.request_id
WHERE request.id = ? AND attachment.id = ?
	AND (request.employee_email = ? OR ?)`,
		requestID,
		attachmentID,
		employeeEmail,
		isAdministrator,
	).Scan(
		&attachment.ID,
		&attachment.FileName,
		&attachment.ContentType,
		&attachment.SizeBytes,
		&attachment.StorageKey,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveRequestNotFound)
		return
	}
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	if attachment.StorageKey == "" || filepath.Base(attachment.StorageKey) != attachment.StorageKey {
		writeAttendanceLeaveRequestError(responseWriter, fmt.Errorf("invalid attachment storage key"))
		return
	}
	file, errorValue := os.Open(filepath.Join(service.attendanceLeaveAttachmentDirectory(), attachment.StorageKey))
	if errorValue != nil {
		if errors.Is(errorValue, os.ErrNotExist) {
			writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveRequestNotFound)
			return
		}
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	defer file.Close()
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	responseWriter.Header().Set("Content-Type", attachment.ContentType)
	responseWriter.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
	responseWriter.Header().Set(
		"Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{"filename": attachment.FileName}),
	)
	responseWriter.WriteHeader(http.StatusOK)
	if _, errorValue := io.Copy(responseWriter, file); errorValue != nil {
		slog.WarnContext(
			request.Context(),
			"attendance leave attachment response failed",
			"request_id",
			requestID,
			"attachment_id",
			attachmentID,
			"error",
			errorValue,
		)
	}
}

func attendanceLeaveRequestAttachmentPath(path string) (string, string, bool) {
	prefix := "/leave-requests/"
	separator := "/attachments/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	values := strings.Split(strings.TrimPrefix(path, prefix), separator)
	if len(values) != 2 || values[0] == "" || values[1] == "" ||
		strings.Contains(values[0], "/") || strings.Contains(values[1], "/") {
		return "", "", false
	}
	return values[0], values[1], true
}
