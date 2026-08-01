package admind

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type attendanceEventsReader func(context.Context, string, string) ([]attendanceEvent, error)

type attendanceAbsencesReader func(context.Context, string, string) ([]attendanceAbsence, error)

func (service *Service) writeAttendanceSummary(responseWriter http.ResponseWriter, request *http.Request) {
	service.writeAttendanceSummaryWithReaders(
		responseWriter,
		request,
		service.readCachedAttendanceEvents,
		service.readCachedAttendanceAbsences,
	)
}

func (service *Service) writeAttendanceSummaryWithReaders(
	responseWriter http.ResponseWriter,
	request *http.Request,
	eventsReader attendanceEventsReader,
	absencesReader attendanceAbsencesReader,
) {
	service.writeAttendanceSummaryWithReadersAt(responseWriter, request, eventsReader, absencesReader, time.Now().UTC())
}

func (service *Service) writeAttendanceSummaryWithReadersAt(
	responseWriter http.ResponseWriter,
	request *http.Request,
	eventsReader attendanceEventsReader,
	absencesReader attendanceAbsencesReader,
	serverTime time.Time,
) {
	timeZone := service.workspaceTimeZone()
	month := normalizeAttendanceMonth(request.URL.Query().Get("month"), serverTime.In(timeZone.location))
	actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request)))
	isAdmin := service.canManageAttendance(request)
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
	activeLeave, errorValue := service.reconcileApprovedLeaveClockOut(
		request.Context(),
		actorEmail,
		serverTime,
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	events, errorValue := eventsReader(request.Context(), month, targetEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	absences, errorValue := absencesReader(request.Context(), month, targetEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	statusEvents := events
	if actorEmail != "" && targetEmail != actorEmail {
		if actorEvents, actorError := eventsReader(request.Context(), month, actorEmail); actorError == nil {
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
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	todayStatus := attendanceStatusForEvents(
		statusEvents,
		serverTime.In(timeZone.location).Format("2006-01-02"),
		serverTime,
	)
	if activeLeave != nil {
		todayStatus = "on_leave"
	}
	service.writeJSON(responseWriter, attendanceSummaryResponse{
		Month:                 month,
		ServerTime:            serverTime.Format(time.RFC3339Nano),
		CurrentUserEmail:      actorEmail,
		IsAdmin:               isAdmin,
		TimeZone:              timeZone.name,
		TimeZoneAuthoritative: timeZone.isAuthoritative,
		Events:                visibleEvents,
		Absences:              projectAttendanceAbsences(absences, actorEmail, isAdmin),
		Members:               members,
		TodayStatus:           todayStatus,
		ActiveLeave:           activeLeave,
		Locations:             locations,
		TeamViewVisibleToAll:  teamVisible,
		TeamViewBlocked:       teamViewBlocked,
	})
}
