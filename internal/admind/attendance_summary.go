package admind

import (
	"net/http"
	"strings"
	"time"
)

func (service *Service) writeAttendanceSummary(responseWriter http.ResponseWriter, request *http.Request) {
	location, timeZoneName := service.workspaceTimeLocation()
	month := normalizeAttendanceMonth(request.URL.Query().Get("month"), time.Now().In(location))
	actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request)))
	isAdmin := service.isAuthorized(request)
	targetEmail := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("email")))
	teamVisible, errorValue := service.readAttendanceTeamViewVisibleToAll(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !isAdmin && !teamVisible {
		targetEmail = actorEmail
	}
	members := service.attendanceMembersForSummary(request, actorEmail, isAdmin, teamVisible)
	events, errorValue := service.readAttendanceEvents(request.Context(), month, targetEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	absences, errorValue := service.readAttendanceAbsences(request.Context(), month, targetEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	statusEvents := events
	if actorEmail != "" && targetEmail != actorEmail {
		if actorEvents, actorError := service.readAttendanceEvents(request.Context(), month, actorEmail); actorError == nil {
			statusEvents = actorEvents
		}
	}
	teamViewBlocked := false
	visibleEvents := events
	if !isAdmin && !teamVisible {
		teamViewBlocked = true
		filtered := make([]attendanceEvent, 0, len(events))
		for _, event := range events {
			if strings.EqualFold(event.Email, actorEmail) {
				filtered = append(filtered, event)
			}
		}
		visibleEvents = filtered
	}
	service.writeJSON(responseWriter, attendanceSummaryResponse{
		Month:                month,
		CurrentUserEmail:     actorEmail,
		IsAdmin:              isAdmin,
		TimeZone:             timeZoneName,
		Events:               visibleEvents,
		Absences:             projectAttendanceAbsences(absences, actorEmail, isAdmin),
		Members:              members,
		TodayStatus:          attendanceStatusForEvents(statusEvents, time.Now().In(location).Format("2006-01-02"), time.Now().UTC()),
		Locations:            locations,
		TeamViewVisibleToAll: teamVisible,
		TeamViewBlocked:      teamViewBlocked,
	})
}
