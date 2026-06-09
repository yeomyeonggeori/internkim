package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (service *Service) writeAttendanceAbsences(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request)))
	isAdmin := service.isAuthorized(request)
	var payload attendanceAbsenceRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	targetEmail := strings.ToLower(strings.TrimSpace(firstNonEmpty(payload.Email, actorEmail)))
	if targetEmail == "" {
		http.Error(responseWriter, "attendance absence email is required", http.StatusBadRequest)
		return
	}
	if !isAdmin && !strings.EqualFold(targetEmail, actorEmail) {
		http.Error(responseWriter, "admin required to create attendance absence for another user", http.StatusForbidden)
		return
	}
	kind, errorValue := normalizeAttendanceAbsenceKind(payload.Kind)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	dates, errorValue := attendanceAbsenceDates(payload.StartDate, payload.EndDate)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	createdAbsences, errorValue := service.insertAttendanceAbsences(request.Context(), targetEmail, kind, dates, payload.Reason, firstNonEmpty(actorEmail, attendanceAbsenceLocalAdminActor))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, attendanceAbsencesResponse{
		Absences: projectAttendanceAbsences(createdAbsences, actorEmail, isAdmin),
	})
}

func (service *Service) deleteAttendanceAbsence(responseWriter http.ResponseWriter, request *http.Request, absenceID string) {
	absence, exists, errorValue := service.readAttendanceAbsenceByID(request.Context(), absenceID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(responseWriter, "attendance absence not found", http.StatusNotFound)
		return
	}
	actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request)))
	isAdmin := service.isAuthorized(request)
	if !isAdmin && !strings.EqualFold(absence.Email, actorEmail) {
		http.Error(responseWriter, "admin required to cancel attendance absence for another user", http.StatusForbidden)
		return
	}
	if errorValue := service.cancelAttendanceAbsence(request.Context(), absenceID); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func projectAttendanceAbsences(absences []attendanceAbsence, actorEmail string, isAdmin bool) []attendanceAbsence {
	projectedAbsences := make([]attendanceAbsence, 0, len(absences))
	for _, absence := range absences {
		projectedAbsences = append(projectedAbsences, projectAttendanceAbsence(absence, actorEmail, isAdmin))
	}
	return projectedAbsences
}

func projectAttendanceAbsence(absence attendanceAbsence, actorEmail string, isAdmin bool) attendanceAbsence {
	canViewPrivateFields := isAdmin || strings.EqualFold(absence.Email, actorEmail)
	absence.LabelKey = absence.Kind
	if !canViewPrivateFields {
		absence.Reason = ""
		absence.CreatedBy = ""
	}
	return absence
}
