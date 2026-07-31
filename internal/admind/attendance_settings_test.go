package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAttendanceSettingsDefaultsDoNotCreateFile(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	document, errorValue := service.readAttendanceSettingsDocument(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if document.Version != attendanceSettingsDocumentVersion ||
		document.TeamViewVisibleToAll == nil ||
		!*document.TeamViewVisibleToAll ||
		document.LeavePolicy.Version != attendanceLeavePolicyVersion {
		t.Fatalf("defaults = %+v", document)
	}
	if _, errorValue = os.Stat(service.attendanceSettingsPath()); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("settings file must not exist before first save: %v", errorValue)
	}
}

func TestAttendanceTeamViewVisibilityRoundtrip(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()

	if errorValue := service.writeAttendanceTeamViewVisibleToAll(ctx, false); errorValue != nil {
		t.Fatal(errorValue)
	}

	visible, errorValue := service.readAttendanceTeamViewVisibleToAll(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if visible {
		t.Fatalf("expected visible=false, got true")
	}

	reloadedService := NewService(service.Configuration)
	visible, errorValue = reloadedService.readAttendanceTeamViewVisibleToAll(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if visible {
		t.Fatal("reloaded visibility must remain false")
	}
}

func TestAttendanceSettingsFilePermissionsAndAtomicReplacement(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(t.Context(), false); errorValue != nil {
		t.Fatal(errorValue)
	}

	fileInfo, errorValue := os.Stat(service.attendanceSettingsPath())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("file permissions = %o", fileInfo.Mode().Perm())
	}
	directoryInfo, errorValue := os.Stat(service.Configuration.StateDirectory)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory permissions = %o", directoryInfo.Mode().Perm())
	}
	temporaryPaths, errorValue := filepath.Glob(filepath.Join(service.Configuration.StateDirectory, ".attendance-settings-*.tmp"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(temporaryPaths) != 0 {
		t.Fatalf("temporary files = %v", temporaryPaths)
	}
}

func TestAttendanceSettingsConcurrentUpdatesPreserveSections(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := defaultAttendanceLeavePolicy()
	policy.FiscalYearStartMonth = 4

	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-start
		errorsChannel <- service.writeAttendanceLeavePolicy(t.Context(), policy)
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		errorsChannel <- service.writeAttendanceTeamViewVisibleToAll(t.Context(), false)
	}()
	close(start)
	waitGroup.Wait()
	close(errorsChannel)
	for errorValue := range errorsChannel {
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	document, errorValue := service.readAttendanceSettingsDocument(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if document.LeavePolicy.FiscalYearStartMonth != 4 ||
		document.TeamViewVisibleToAll == nil ||
		*document.TeamViewVisibleToAll {
		t.Fatalf("stored document = %+v", document)
	}
}

func TestAttendanceSettingsDropsLegacyWorkScheduleAndEvidenceGuidanceOnSave(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	document := validAttendanceSettingsTestDocument()
	document.LegacyWorkSchedule = json.RawMessage(`{"version":1}`)
	document.LeavePolicy.LeaveTypes[0].LegacyEvidenceGuidance = "Attach a document"
	encodedDocument, errorValue := json.Marshal(document)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue = os.WriteFile(service.attendanceSettingsPath(), encodedDocument, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue = service.writeAttendanceTeamViewVisibleToAll(t.Context(), false); errorValue != nil {
		t.Fatal(errorValue)
	}
	savedDocument, errorValue := os.ReadFile(service.attendanceSettingsPath())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(savedDocument), `"workSchedule"`) ||
		strings.Contains(string(savedDocument), `"evidenceGuidance"`) {
		t.Fatalf("legacy settings remain in document: %s", savedDocument)
	}
}

func TestAttendanceSettingsDoNotCreateDatabaseTable(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(t.Context(), false); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	var tableCount int
	if errorValue = database.QueryRowContext(
		t.Context(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'attendance_settings'`,
	).Scan(&tableCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if tableCount != 0 {
		t.Fatalf("attendance_settings table count = %d", tableCount)
	}
}

func TestAttendanceSettingsRejectsCorruptAndInvalidDocuments(t *testing.T) {
	tests := []struct {
		name     string
		document func(*testing.T) []byte
	}{
		{
			name: "corrupt JSON",
			document: func(*testing.T) []byte {
				return []byte(`{"version":`)
			},
		},
		{
			name: "unknown field",
			document: func(t *testing.T) []byte {
				document := validAttendanceSettingsTestDocument()
				encodedDocument, errorValue := json.Marshal(document)
				if errorValue != nil {
					t.Fatal(errorValue)
				}
				return append(encodedDocument[:len(encodedDocument)-1], []byte(`,"unknown":true}`)...)
			},
		},
		{
			name: "missing team visibility",
			document: func(t *testing.T) []byte {
				document := validAttendanceSettingsTestDocument()
				document.TeamViewVisibleToAll = nil
				encodedDocument, errorValue := json.Marshal(document)
				if errorValue != nil {
					t.Fatal(errorValue)
				}
				return encodedDocument
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service, _ := newAttendanceActionTestService(t)
			if errorValue := os.WriteFile(service.attendanceSettingsPath(), testCase.document(t), 0o600); errorValue != nil {
				t.Fatal(errorValue)
			}
			if _, errorValue := service.readAttendanceSettingsDocument(t.Context()); errorValue == nil {
				t.Fatal("expected invalid document error")
			}
		})
	}
}

func TestAttendanceSettingsToggle(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/attendance/api/settings", strings.NewReader(`{"teamViewVisibleToAll":false}`))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-Email", "admin@example.com")
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatal(errorValue)
	}
	if body["teamViewVisibleToAll"] != false {
		t.Fatalf("response = %+v", body)
	}
}

func TestAttendanceSettingsRejectsMemberThroughLoopback(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/attendance/api/settings", strings.NewReader(`{"teamViewVisibleToAll":false}`))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func validAttendanceSettingsTestDocument() attendanceSettingsDocument {
	teamViewVisibleToAll := true
	return attendanceSettingsDocument{
		Version:              attendanceSettingsDocumentVersion,
		UpdatedAt:            time.Now().UTC().Format(time.RFC3339),
		TeamViewVisibleToAll: &teamViewVisibleToAll,
		LeavePolicy:          defaultAttendanceLeavePolicy(),
	}
}
