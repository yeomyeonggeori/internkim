package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const attendanceSettingTeamViewVisibleToAll = "team_view_visible_to_all"

func (service *Service) writeAttendanceSettings(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.isAuthorized(request) {
		http.Error(responseWriter, "admin required", http.StatusForbidden)
		return
	}
	var body attendanceSettingsRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if body.TeamViewVisibleToAll != nil {
		if errorValue := service.writeAttendanceTeamViewVisibleToAll(request.Context(), *body.TeamViewVisibleToAll); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	}
	teamVisible, errorValue := service.readAttendanceTeamViewVisibleToAll(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]any{
		"teamViewVisibleToAll": teamVisible,
	})
}

func (service *Service) readAttendanceTeamViewVisibleToAll(ctx context.Context) (bool, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return true, errorValue
	}
	defer database.Close()
	var value string
	errorValue = database.QueryRowContext(ctx, `SELECT value FROM attendance_settings WHERE key = ?`, attendanceSettingTeamViewVisibleToAll).Scan(&value)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return true, nil
	}
	if errorValue != nil {
		return true, errorValue
	}
	return value == "true", nil
}

func (service *Service) writeAttendanceTeamViewVisibleToAll(ctx context.Context, visible bool) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	value := "false"
	if visible {
		value = "true"
	}
	_, errorValue = database.ExecContext(ctx, `INSERT INTO attendance_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		attendanceSettingTeamViewVisibleToAll, value, time.Now().UTC().Format(time.RFC3339))
	return errorValue
}
