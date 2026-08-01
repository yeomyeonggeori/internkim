package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const attendanceSettingsDocumentVersion = 1

var attendanceSettingsFileMutex sync.Mutex

type attendanceSettingsDocument struct {
	Version              int                   `json:"version"`
	UpdatedAt            string                `json:"updatedAt"`
	TeamViewVisibleToAll *bool                 `json:"teamViewVisibleToAll"`
	WorkPolicy           attendanceWorkPolicy  `json:"workPolicy"`
	LeavePolicy          attendanceLeavePolicy `json:"leavePolicy"`
	LegacyWorkSchedule   json.RawMessage       `json:"workSchedule,omitempty"`
}

func (service *Service) readAttendanceSettingsDocument(ctx context.Context) (attendanceSettingsDocument, error) {
	if errorValue := ctx.Err(); errorValue != nil {
		return attendanceSettingsDocument{}, errorValue
	}
	attendanceSettingsFileMutex.Lock()
	defer attendanceSettingsFileMutex.Unlock()
	if errorValue := ctx.Err(); errorValue != nil {
		return attendanceSettingsDocument{}, errorValue
	}
	return service.readAttendanceSettingsDocumentUnlocked()
}

func (service *Service) updateAttendanceSettingsDocument(
	ctx context.Context,
	update func(*attendanceSettingsDocument) error,
) error {
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	attendanceSettingsFileMutex.Lock()
	defer attendanceSettingsFileMutex.Unlock()
	if errorValue := ctx.Err(); errorValue != nil {
		return errorValue
	}
	document, errorValue := service.readAttendanceSettingsDocumentUnlocked()
	if errorValue != nil {
		return errorValue
	}
	if errorValue = update(&document); errorValue != nil {
		return errorValue
	}
	document.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return service.writeAttendanceSettingsDocumentUnlocked(document)
}

func (service *Service) readAttendanceSettingsDocumentUnlocked() (attendanceSettingsDocument, error) {
	path := service.attendanceSettingsPath()
	encodedDocument, errorValue := os.ReadFile(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return defaultAttendanceSettingsDocument(), nil
	}
	if errorValue != nil {
		return attendanceSettingsDocument{}, fmt.Errorf("read attendance settings %q: %w", path, errorValue)
	}
	document, errorValue := decodeAttendanceSettingsDocument(encodedDocument)
	if errorValue != nil {
		return attendanceSettingsDocument{}, fmt.Errorf("decode attendance settings %q: %w", path, errorValue)
	}
	return document, nil
}

func (service *Service) writeAttendanceSettingsDocumentUnlocked(document attendanceSettingsDocument) error {
	if errorValue := validateAttendanceSettingsDocument(&document); errorValue != nil {
		return errorValue
	}
	encodedDocument, errorValue := json.MarshalIndent(document, "", "  ")
	if errorValue != nil {
		return fmt.Errorf("encode attendance settings: %w", errorValue)
	}
	path := service.attendanceSettingsPath()
	directory := filepath.Dir(path)
	if errorValue = os.MkdirAll(directory, 0o700); errorValue != nil {
		return fmt.Errorf("create attendance settings directory %q: %w", directory, errorValue)
	}
	if errorValue = os.Chmod(directory, 0o700); errorValue != nil {
		return fmt.Errorf("secure attendance settings directory %q: %w", directory, errorValue)
	}
	temporaryPath, errorValue := writeAttendanceSettingsTemporaryFile(directory, append(encodedDocument, '\n'))
	if errorValue != nil {
		return errorValue
	}
	if errorValue = os.Rename(temporaryPath, path); errorValue != nil {
		os.Remove(temporaryPath)
		return fmt.Errorf("replace attendance settings %q: %w", path, errorValue)
	}
	return nil
}

func writeAttendanceSettingsTemporaryFile(directory string, document []byte) (string, error) {
	temporaryFile, errorValue := os.CreateTemp(directory, ".attendance-settings-*.tmp")
	if errorValue != nil {
		return "", fmt.Errorf("create temporary attendance settings file in %q: %w", directory, errorValue)
	}
	temporaryPath := temporaryFile.Name()
	closeAndRemove := func() {
		temporaryFile.Close()
		os.Remove(temporaryPath)
	}
	if errorValue = temporaryFile.Chmod(0o600); errorValue != nil {
		closeAndRemove()
		return "", fmt.Errorf("secure temporary attendance settings file %q: %w", temporaryPath, errorValue)
	}
	if _, errorValue = temporaryFile.Write(document); errorValue != nil {
		closeAndRemove()
		return "", fmt.Errorf("write temporary attendance settings file %q: %w", temporaryPath, errorValue)
	}
	if errorValue = temporaryFile.Sync(); errorValue != nil {
		closeAndRemove()
		return "", fmt.Errorf("sync temporary attendance settings file %q: %w", temporaryPath, errorValue)
	}
	if errorValue = temporaryFile.Close(); errorValue != nil {
		os.Remove(temporaryPath)
		return "", fmt.Errorf("close temporary attendance settings file %q: %w", temporaryPath, errorValue)
	}
	return temporaryPath, nil
}

func decodeAttendanceSettingsDocument(encodedDocument []byte) (attendanceSettingsDocument, error) {
	var document attendanceSettingsDocument
	decoder := json.NewDecoder(bytes.NewReader(encodedDocument))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&document); errorValue != nil {
		return attendanceSettingsDocument{}, errorValue
	}
	if errorValue := decoder.Decode(&struct{}{}); !errors.Is(errorValue, io.EOF) {
		if errorValue == nil {
			return attendanceSettingsDocument{}, errors.New("attendance settings must contain one JSON document")
		}
		return attendanceSettingsDocument{}, errorValue
	}
	normalizeLegacyAttendanceLeavePolicy(&document.LeavePolicy)
	if document.WorkPolicy.Version == 0 {
		document.WorkPolicy = defaultAttendanceWorkPolicy()
	}
	if errorValue := validateAttendanceSettingsDocument(&document); errorValue != nil {
		return attendanceSettingsDocument{}, errorValue
	}
	document.LegacyWorkSchedule = nil
	return document, nil
}

func validateAttendanceSettingsDocument(document *attendanceSettingsDocument) error {
	if document.Version != attendanceSettingsDocumentVersion {
		return fmt.Errorf("attendance settings version must be %d", attendanceSettingsDocumentVersion)
	}
	if document.UpdatedAt == "" {
		return errors.New("attendance settings updatedAt is required")
	}
	if _, errorValue := time.Parse(time.RFC3339, document.UpdatedAt); errorValue != nil {
		return fmt.Errorf("attendance settings updatedAt must use RFC3339: %w", errorValue)
	}
	if document.TeamViewVisibleToAll == nil {
		return errors.New("attendance settings teamViewVisibleToAll is required")
	}
	if errorValue := validateAndNormalizeAttendanceWorkPolicy(&document.WorkPolicy); errorValue != nil {
		return fmt.Errorf("invalid attendance work policy: %w", errorValue)
	}
	if errorValue := validateAttendanceLeavePolicy(&document.LeavePolicy, nil); errorValue != nil {
		return fmt.Errorf("invalid attendance leave policy: %w", errorValue)
	}
	return nil
}

func defaultAttendanceSettingsDocument() attendanceSettingsDocument {
	teamViewVisibleToAll := true
	return attendanceSettingsDocument{
		Version:              attendanceSettingsDocumentVersion,
		TeamViewVisibleToAll: &teamViewVisibleToAll,
		WorkPolicy:           defaultAttendanceWorkPolicy(),
		LeavePolicy:          defaultAttendanceLeavePolicy(),
	}
}

func (service *Service) attendanceSettingsPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "attendance-settings.json")
}

func (service *Service) readAttendanceTeamViewVisibleToAll(ctx context.Context) (bool, error) {
	document, errorValue := service.readAttendanceSettingsDocument(ctx)
	if errorValue != nil {
		return true, errorValue
	}
	return *document.TeamViewVisibleToAll, nil
}

func (service *Service) writeAttendanceTeamViewVisibleToAll(ctx context.Context, visible bool) error {
	return service.updateAttendanceSettingsDocument(ctx, func(document *attendanceSettingsDocument) error {
		document.TeamViewVisibleToAll = &visible
		return nil
	})
}
