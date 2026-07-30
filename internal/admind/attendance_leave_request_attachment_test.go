package admind

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttendanceLeaveRequestAttachmentIsPrivateAndOwnerDownloadable(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	pngContent := append(
		[]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
		[]byte("private-evidence")...,
	)
	createRecorder := performAttendanceLeaveMultipartRequestWithAttachment(
		t,
		service,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"fullDay","startDate":"2027-05-03","reason":"Medical appointment"}`,
		"evidence.png",
		pngContent,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Request attendanceLeaveRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(created.Request.Attachments) != 1 {
		t.Fatalf("attachments = %+v", created.Request.Attachments)
	}
	attachment := created.Request.Attachments[0]
	if attachment.FileName != "evidence.png" ||
		attachment.ContentType != "image/png" ||
		attachment.SizeBytes != int64(len(pngContent)) ||
		attachment.DownloadURL == "" {
		t.Fatalf("attachment = %+v", attachment)
	}
	if strings.Contains(createRecorder.Body.String(), "storage") ||
		strings.Contains(createRecorder.Body.String(), service.Configuration.AttendanceDatabasePath) {
		t.Fatalf("response exposes storage path: %s", createRecorder.Body.String())
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var storageKey string
	if errorValue := database.QueryRowContext(
		t.Context(),
		`SELECT storage_key FROM attendance_leave_request_attachments WHERE id = ?`,
		attachment.ID,
	).Scan(&storageKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()
	fileInformation, errorValue := os.Stat(filepath.Join(service.attendanceLeaveAttachmentDirectory(), storageKey))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if fileInformation.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o", fileInformation.Mode().Perm())
	}

	ownerRequest := httptest.NewRequest(http.MethodGet, attachment.DownloadURL, nil)
	ownerRequest.RemoteAddr = "203.0.113.10:1234"
	ownerRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	ownerRecorder := httptest.NewRecorder()
	service.handleAttendance(ownerRecorder, ownerRequest)
	if ownerRecorder.Code != http.StatusOK {
		t.Fatalf("owner download status = %d body = %s", ownerRecorder.Code, ownerRecorder.Body.String())
	}
	downloaded, errorValue := io.ReadAll(ownerRecorder.Body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !bytes.Equal(downloaded, pngContent) {
		t.Fatalf("downloaded content = %q", downloaded)
	}

	otherRequest := httptest.NewRequest(http.MethodGet, attachment.DownloadURL, nil)
	otherRequest.RemoteAddr = "203.0.113.10:1234"
	otherRequest.Header.Set("X-Forwarded-Email", "other@example.com")
	otherRecorder := httptest.NewRecorder()
	service.handleAttendance(otherRecorder, otherRequest)
	assertAttendanceLeaveErrorResponse(
		t,
		otherRecorder,
		http.StatusNotFound,
		attendanceLeaveErrorRequestNotFound,
	)

	adminRequest := httptest.NewRequest(http.MethodGet, attachment.DownloadURL, nil)
	adminRequest.RemoteAddr = "203.0.113.10:1234"
	adminRequest.Header.Set("X-Forwarded-Email", "admin@example.com")
	adminRecorder := httptest.NewRecorder()
	service.handleAttendance(adminRecorder, adminRequest)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("admin download status = %d body = %s", adminRecorder.Code, adminRecorder.Body.String())
	}
	adminDownloaded, errorValue := io.ReadAll(adminRecorder.Body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !bytes.Equal(adminDownloaded, pngContent) {
		t.Fatalf("admin downloaded content = %q", adminDownloaded)
	}

	cancelRequest := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/"+created.Request.ID+"/cancel",
		nil,
	)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	if _, errorValue := os.Stat(filepath.Join(service.attendanceLeaveAttachmentDirectory(), storageKey)); !os.IsNotExist(errorValue) {
		t.Fatalf("attachment remains after cancel: %v", errorValue)
	}
}

func TestAttendanceLeaveRequestRejectsInvalidAttachmentWithJSONError(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := performAttendanceLeaveMultipartRequestWithAttachment(
		t,
		service,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"fullDay","startDate":"2027-05-03","reason":"Medical appointment"}`,
		"evidence.txt",
		[]byte("not an allowed attachment"),
	)

	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusBadRequest,
		attendanceLeaveErrorInvalidAttachment,
	)
}

func TestAttendanceLeaveRequestStoredAttachmentFailurePreservesCleanupTargets(t *testing.T) {
	directory := t.TempDir()
	storageKey := "tracked-attachment"
	storageDirectory := filepath.Join(directory, storageKey)
	if errorValue := os.Mkdir(storageDirectory, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(
		filepath.Join(storageDirectory, "child"),
		[]byte("keeps directory non-empty"),
		0o600,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	operationError := errors.New("attachment write failed")
	attachments := []attendanceLeaveRequestAttachment{{StorageKey: storageKey}}

	tracked, errorValue := failedStoredAttendanceLeaveRequestAttachments(
		directory,
		attachments,
		operationError,
	)

	if len(tracked) != 1 || tracked[0].StorageKey != storageKey {
		t.Fatalf("tracked attachments = %+v", tracked)
	}
	if !errors.Is(errorValue, operationError) {
		t.Fatalf("error = %v", errorValue)
	}
	joined, ok := errorValue.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 2 {
		t.Fatalf("joined error = %#v", errorValue)
	}
}

func TestAttendanceLeaveRequestQuarantineFailurePreservesRecoveryTargets(t *testing.T) {
	operationError := errors.New("attachment quarantine failed")
	restoreError := errors.New("attachment restore failed")
	quarantined := []attendanceLeaveRequestAttachmentQuarantine{
		{
			OriginalPath:   "/private/attachment",
			QuarantinePath: "/private/.deleting-attachment",
		},
	}

	tracked, errorValue := failedAttendanceLeaveRequestAttachmentQuarantine(
		quarantined,
		operationError,
		func(values []attendanceLeaveRequestAttachmentQuarantine) error {
			if len(values) != 1 || values[0] != quarantined[0] {
				t.Fatalf("restore targets = %+v", values)
			}
			return restoreError
		},
	)

	if len(tracked) != 1 || tracked[0] != quarantined[0] {
		t.Fatalf("tracked quarantine = %+v", tracked)
	}
	if !errors.Is(errorValue, operationError) || !errors.Is(errorValue, restoreError) {
		t.Fatalf("error = %v", errorValue)
	}
}
