package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	attendanceLeaveRequestMaximumAttachmentCount = 5
	attendanceLeaveRequestMaximumAttachmentBytes = 10 << 20
	attendanceLeaveRequestAttachmentCleanupTries = 3
	attendanceLeaveRequestAttachmentCleanupDelay = 10 * time.Millisecond
)

var attendanceLeaveRequestAttachmentContentTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"application/pdf": true,
}

type attendanceLeaveRequestAttachmentQuarantine struct {
	OriginalPath   string
	QuarantinePath string
}

func (service *Service) attendanceLeaveAttachmentDirectory() string {
	return filepath.Join(
		filepath.Dir(service.Configuration.AttendanceDatabasePath),
		"attendance-leave-attachments",
	)
}

func (service *Service) storeAttendanceLeaveRequestAttachments(
	uploads []attendanceLeaveRequestAttachmentUpload,
) ([]attendanceLeaveRequestAttachment, error) {
	if len(uploads) == 0 {
		return []attendanceLeaveRequestAttachment{}, nil
	}
	directory := service.attendanceLeaveAttachmentDirectory()
	if errorValue := os.MkdirAll(directory, 0o700); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := os.Chmod(directory, 0o700); errorValue != nil {
		return nil, errorValue
	}
	attachments := make([]attendanceLeaveRequestAttachment, 0, len(uploads))
	for _, upload := range uploads {
		attachmentID, errorValue := generateRandomURLToken(18)
		if errorValue != nil {
			return failedStoredAttendanceLeaveRequestAttachments(
				directory,
				attachments,
				errorValue,
			)
		}
		storageKey, errorValue := generateRandomURLToken(24)
		if errorValue != nil {
			return failedStoredAttendanceLeaveRequestAttachments(
				directory,
				attachments,
				errorValue,
			)
		}
		file, errorValue := os.OpenFile(
			filepath.Join(directory, storageKey),
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0o600,
		)
		if errorValue != nil {
			return failedStoredAttendanceLeaveRequestAttachments(
				directory,
				attachments,
				errorValue,
			)
		}
		_, writeError := file.Write(upload.Content)
		closeError := file.Close()
		if writeError != nil || closeError != nil {
			currentAttachment := attendanceLeaveRequestAttachment{StorageKey: storageKey}
			failedAttachments := append(attachments, currentAttachment)
			return failedStoredAttendanceLeaveRequestAttachments(
				directory,
				failedAttachments,
				errors.Join(writeError, closeError),
			)
		}
		attachments = append(attachments, attendanceLeaveRequestAttachment{
			ID:          "attachment-" + attachmentID,
			FileName:    upload.FileName,
			ContentType: upload.ContentType,
			SizeBytes:   int64(len(upload.Content)),
			StorageKey:  storageKey,
		})
	}
	return attachments, nil
}

func failedStoredAttendanceLeaveRequestAttachments(
	directory string,
	attachments []attendanceLeaveRequestAttachment,
	operationError error,
) ([]attendanceLeaveRequestAttachment, error) {
	return attachments, errors.Join(
		operationError,
		removeAttendanceLeaveRequestAttachmentFiles(directory, attachments),
	)
}

func insertAttendanceLeaveRequestAttachments(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	attachments []attendanceLeaveRequestAttachment,
	createdAt string,
) error {
	for _, attachment := range attachments {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_request_attachments (
	id, request_id, file_name, content_type, size_bytes, storage_key, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			attachment.ID,
			requestID,
			attachment.FileName,
			attachment.ContentType,
			attachment.SizeBytes,
			attachment.StorageKey,
			createdAt,
		); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func removeAttendanceLeaveRequestAttachmentFiles(
	directory string,
	attachments []attendanceLeaveRequestAttachment,
) error {
	var removalError error
	for _, attachment := range attachments {
		if attachment.StorageKey == "" || filepath.Base(attachment.StorageKey) != attachment.StorageKey {
			continue
		}
		errorValue := retryAttendanceLeaveRequestAttachmentFileOperation(func() error {
			errorValue := os.Remove(filepath.Join(directory, attachment.StorageKey))
			if errors.Is(errorValue, os.ErrNotExist) {
				return nil
			}
			return errorValue
		})
		if errorValue != nil && removalError == nil {
			removalError = errorValue
		}
	}
	return removalError
}

func quarantineAttendanceLeaveRequestAttachmentFiles(
	directory string,
	attachments []attendanceLeaveRequestAttachment,
) ([]attendanceLeaveRequestAttachmentQuarantine, error) {
	quarantined := make([]attendanceLeaveRequestAttachmentQuarantine, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment.StorageKey == "" || filepath.Base(attachment.StorageKey) != attachment.StorageKey {
			return failedAttendanceLeaveRequestAttachmentQuarantine(
				quarantined,
				fmt.Errorf("invalid attachment storage key"),
				restoreAttendanceLeaveRequestAttachmentFiles,
			)
		}
		token, errorValue := generateRandomURLToken(18)
		if errorValue != nil {
			return failedAttendanceLeaveRequestAttachmentQuarantine(
				quarantined,
				errorValue,
				restoreAttendanceLeaveRequestAttachmentFiles,
			)
		}
		entry := attendanceLeaveRequestAttachmentQuarantine{
			OriginalPath:   filepath.Join(directory, attachment.StorageKey),
			QuarantinePath: filepath.Join(directory, ".deleting-"+token),
		}
		if errorValue := os.Rename(entry.OriginalPath, entry.QuarantinePath); errorValue != nil {
			return failedAttendanceLeaveRequestAttachmentQuarantine(
				quarantined,
				errorValue,
				restoreAttendanceLeaveRequestAttachmentFiles,
			)
		}
		quarantined = append(quarantined, entry)
	}
	return quarantined, nil
}

func failedAttendanceLeaveRequestAttachmentQuarantine(
	quarantined []attendanceLeaveRequestAttachmentQuarantine,
	operationError error,
	restore func([]attendanceLeaveRequestAttachmentQuarantine) error,
) ([]attendanceLeaveRequestAttachmentQuarantine, error) {
	return quarantined, errors.Join(operationError, restore(quarantined))
}

func restoreAttendanceLeaveRequestAttachmentFiles(
	quarantined []attendanceLeaveRequestAttachmentQuarantine,
) error {
	var restoreError error
	for index := len(quarantined) - 1; index >= 0; index-- {
		entry := quarantined[index]
		errorValue := retryAttendanceLeaveRequestAttachmentFileOperation(func() error {
			errorValue := os.Rename(entry.QuarantinePath, entry.OriginalPath)
			if errors.Is(errorValue, os.ErrNotExist) {
				if _, statError := os.Stat(entry.OriginalPath); statError == nil {
					return nil
				}
			}
			return errorValue
		})
		if errorValue != nil && restoreError == nil {
			restoreError = errorValue
		}
	}
	return restoreError
}

func removeQuarantinedAttendanceLeaveRequestAttachmentFiles(
	quarantined []attendanceLeaveRequestAttachmentQuarantine,
) error {
	var removalError error
	for _, entry := range quarantined {
		errorValue := retryAttendanceLeaveRequestAttachmentFileOperation(func() error {
			errorValue := os.Remove(entry.QuarantinePath)
			if errors.Is(errorValue, os.ErrNotExist) {
				return nil
			}
			return errorValue
		})
		if errorValue != nil && removalError == nil {
			removalError = errorValue
		}
	}
	return removalError
}

func retryAttendanceLeaveRequestAttachmentFileOperation(operation func() error) error {
	var operationError error
	for attempt := 0; attempt < attendanceLeaveRequestAttachmentCleanupTries; attempt++ {
		operationError = operation()
		if operationError == nil {
			return nil
		}
		if attempt+1 < attendanceLeaveRequestAttachmentCleanupTries {
			time.Sleep(attendanceLeaveRequestAttachmentCleanupDelay << attempt)
		}
	}
	return operationError
}

func cleanupFailedAttendanceLeaveRequestAttachments(
	ctx context.Context,
	directory string,
	attachments []attendanceLeaveRequestAttachment,
) {
	if errorValue := removeAttendanceLeaveRequestAttachmentFiles(directory, attachments); errorValue != nil {
		slog.WarnContext(
			ctx,
			"attendance leave attachment cleanup failed",
			"error",
			errorValue,
		)
	}
}

func cleanupCancelledAttendanceLeaveRequestAttachments(
	ctx context.Context,
	requestID string,
	quarantined []attendanceLeaveRequestAttachmentQuarantine,
) {
	if errorValue := removeQuarantinedAttendanceLeaveRequestAttachmentFiles(quarantined); errorValue != nil {
		slog.WarnContext(
			ctx,
			"attendance leave cancelled attachment cleanup failed",
			"request_id",
			requestID,
			"error",
			errorValue,
		)
	}
}

func readAttendanceLeaveRequestAttachmentInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
) ([]attendanceLeaveRequestAttachment, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, file_name, content_type, size_bytes, storage_key
FROM attendance_leave_request_attachments
WHERE request_id = ?
ORDER BY created_at, id`,
		requestID,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	attachments := []attendanceLeaveRequestAttachment{}
	for rows.Next() {
		var attachment attendanceLeaveRequestAttachment
		if errorValue := rows.Scan(
			&attachment.ID,
			&attachment.FileName,
			&attachment.ContentType,
			&attachment.SizeBytes,
			&attachment.StorageKey,
		); errorValue != nil {
			return nil, errorValue
		}
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

func normalizeAttendanceLeaveRequestAttachmentUpload(
	fileName string,
	content []byte,
) (attendanceLeaveRequestAttachmentUpload, error) {
	normalizedFileName := filepath.Base(strings.ReplaceAll(strings.TrimSpace(fileName), "\\", "/"))
	if normalizedFileName == "" || normalizedFileName == "." {
		return attendanceLeaveRequestAttachmentUpload{}, fmt.Errorf("attachment file name is required")
	}
	if len(content) == 0 || len(content) > attendanceLeaveRequestMaximumAttachmentBytes {
		return attendanceLeaveRequestAttachmentUpload{}, fmt.Errorf(
			"attachment must be between 1 byte and %d bytes",
			attendanceLeaveRequestMaximumAttachmentBytes,
		)
	}
	contentType := http.DetectContentType(content)
	if !attendanceLeaveRequestAttachmentContentTypes[contentType] {
		return attendanceLeaveRequestAttachmentUpload{}, fmt.Errorf("unsupported attachment content type %q", contentType)
	}
	return attendanceLeaveRequestAttachmentUpload{
		FileName:    normalizedFileName,
		ContentType: contentType,
		Content:     content,
	}, nil
}
