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
		TodayStatus:          attendanceStatusForEvents(statusEvents, time.Now().In(location).Format("2006-01-02")),
		Locations:            locations,
		TeamViewVisibleToAll: teamVisible,
		TeamViewBlocked:      teamViewBlocked,
	})
}

func attendanceStatusForEvents(events []attendanceEvent, localDate string) string {
	for _, event := range events {
		if event.LocalDate != localDate || event.CanceledAt != "" {
			continue
		}
		if event.Kind == attendanceKindClockIn {
			return "clocked_in"
		}
		if event.Kind == attendanceKindClockOut {
			return "clocked_out"
		}
	}
	return "not_clocked_in"
}

func normalizeAttendanceMonth(value string, fallback time.Time) string {
	trimmedValue := strings.TrimSpace(value)
	if _, errorValue := time.Parse("2006-01", trimmedValue); errorValue == nil {
		return trimmedValue
	}
	return fallback.Format("2006-01")
}

func attendanceNextMonth(month string) string {
	parsedTime, errorValue := time.Parse("2006-01", month)
	if errorValue != nil {
		return month
	}
	return parsedTime.AddDate(0, 1, 0).Format("2006-01")
}
