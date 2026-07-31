package admind

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type attendancePaidLeaveReader func(
	context.Context,
	string,
	string,
) ([]attendancePaidLeaveOccurrence, error)

type attendanceHolidayDatesReader func(
	context.Context,
	time.Time,
	time.Time,
) (map[string]struct{}, error)

type attendanceWorkStatusResponse struct {
	Period      string                 `json:"period"`
	Anchor      string                 `json:"anchor"`
	PeriodStart string                 `json:"periodStart"`
	PeriodEnd   string                 `json:"periodEnd"`
	TimeZone    string                 `json:"timeZone"`
	IsAdmin     bool                   `json:"isAdmin"`
	Personal    *attendanceWorkStatus  `json:"personal,omitempty"`
	Employees   []attendanceWorkStatus `json:"employees"`
}

func (service *Service) writeAttendanceWorkStatus(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	service.writeAttendanceWorkStatusWithReadersAt(
		responseWriter,
		request,
		service.readAttendanceEvents,
		service.readPaidAttendanceLeaveOccurrences,
		service.readCalendarHolidayDatesForRange,
		time.Now().UTC(),
	)
}

func (service *Service) writeAttendanceWorkStatusWithReadersAt(
	responseWriter http.ResponseWriter,
	request *http.Request,
	eventsReader attendanceEventsReader,
	leaveReader attendancePaidLeaveReader,
	holidayReader attendanceHolidayDatesReader,
	now time.Time,
) {
	timeZone := service.workspaceTimeZone()
	period, anchor, startDate, endDate, errorValue := attendanceWorkPeriod(
		request.URL.Query().Get("period"),
		request.URL.Query().Get("anchor"),
		now.In(timeZone.location),
		timeZone.location,
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request)))
	isAdmin := service.isAuthorized(request)
	events, errorValue := readAttendanceWorkPeriodEvents(
		request.Context(),
		startDate,
		endDate,
		eventsReader,
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	paidLeave, errorValue := leaveReader(request.Context(), startDate, endDate)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	policy, errorValue := service.readAttendanceWorkPolicy(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	holidayStart, errorValue := attendanceWorkScheduleDate(startDate, timeZone.location)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	holidayEnd, errorValue := attendanceWorkScheduleDate(endDate, timeZone.location)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	holidayDates, errorValue := holidayReader(request.Context(), holidayStart, holidayEnd)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	response := attendanceWorkStatusResponse{
		Period:      period,
		Anchor:      anchor,
		PeriodStart: startDate,
		PeriodEnd:   endDate,
		TimeZone:    timeZone.name,
		IsAdmin:     isAdmin,
		Employees:   []attendanceWorkStatus{},
	}
	if actorEmail != "" {
		personal, metricError := calculateAttendanceWorkStatus(
			actorEmail,
			attendanceWorkDisplayName(events, actorEmail),
			startDate,
			endDate,
			events,
			paidLeave,
			policy,
			holidayDates,
			timeZone.location,
			now,
		)
		if metricError != nil {
			http.Error(responseWriter, metricError.Error(), http.StatusInternalServerError)
			return
		}
		response.Personal = &personal
	}
	if isAdmin {
		members := service.attendanceMembersForSummary(request, actorEmail, true, true)
		members = mergeAttendanceWorkMembers(members, events, actorEmail)
		for _, member := range members {
			status, metricError := calculateAttendanceWorkStatus(
				member.Email,
				member.DisplayName,
				startDate,
				endDate,
				events,
				paidLeave,
				policy,
				holidayDates,
				timeZone.location,
				now,
			)
			if metricError != nil {
				http.Error(responseWriter, metricError.Error(), http.StatusInternalServerError)
				return
			}
			response.Employees = append(response.Employees, status)
		}
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, response)
}

func attendanceWorkPeriod(
	periodValue string,
	anchorValue string,
	now time.Time,
	location *time.Location,
) (string, string, string, string, error) {
	period := strings.TrimSpace(periodValue)
	if period == "" {
		period = "week"
	}
	if period != "day" && period != "week" && period != "month" {
		return "", "", "", "", fmt.Errorf("period must be day, week, or month")
	}
	anchor := strings.TrimSpace(anchorValue)
	if anchor == "" {
		anchor = now.Format(time.DateOnly)
	}
	anchorDate, errorValue := attendanceWorkScheduleDate(anchor, location)
	if errorValue != nil {
		return "", "", "", "", errorValue
	}
	start := anchorDate
	end := anchorDate
	if period == "week" {
		weekdayOffset := (int(anchorDate.Weekday()) + 6) % 7
		start = anchorDate.AddDate(0, 0, -weekdayOffset)
		end = start.AddDate(0, 0, 6)
	} else if period == "month" {
		start = time.Date(anchorDate.Year(), anchorDate.Month(), 1, 0, 0, 0, 0, location)
		end = start.AddDate(0, 1, -1)
	}
	return period, anchor, start.Format(time.DateOnly), end.Format(time.DateOnly), nil
}

func readAttendanceWorkPeriodEvents(
	ctx context.Context,
	startDate string,
	endDate string,
	reader attendanceEventsReader,
) ([]attendanceEvent, error) {
	start, startError := time.Parse(time.DateOnly, startDate)
	end, endError := time.Parse(time.DateOnly, endDate)
	if startError != nil || endError != nil {
		return nil, fmt.Errorf("invalid attendance work period")
	}
	expandedStart := start.AddDate(0, 0, -1)
	expandedEnd := end.AddDate(0, 0, 1)
	months := []string{}
	for month := time.Date(expandedStart.Year(), expandedStart.Month(), 1, 0, 0, 0, 0, time.UTC); !month.After(expandedEnd); month = month.AddDate(0, 1, 0) {
		months = append(months, month.Format("2006-01"))
	}
	events := []attendanceEvent{}
	seenIDs := make(map[string]struct{})
	for _, month := range months {
		monthlyEvents, errorValue := reader(ctx, month, "")
		if errorValue != nil {
			return nil, errorValue
		}
		for _, event := range monthlyEvents {
			if event.LocalDate != "" &&
				(event.LocalDate < expandedStart.Format(time.DateOnly) ||
					event.LocalDate > expandedEnd.Format(time.DateOnly)) {
				continue
			}
			if _, seen := seenIDs[event.ID]; seen && event.ID != "" {
				continue
			}
			if event.ID != "" {
				seenIDs[event.ID] = struct{}{}
			}
			events = append(events, event)
		}
	}
	return events, nil
}

func mergeAttendanceWorkMembers(
	members []attendanceMember,
	events []attendanceEvent,
	actorEmail string,
) []attendanceMember {
	byEmail := make(map[string]attendanceMember)
	for _, member := range members {
		email := strings.ToLower(strings.TrimSpace(member.Email))
		if email != "" {
			member.Email = email
			byEmail[email] = member
		}
	}
	for _, event := range events {
		email := strings.ToLower(strings.TrimSpace(event.Email))
		if email == "" {
			continue
		}
		if _, exists := byEmail[email]; !exists {
			byEmail[email] = attendanceMember{Email: email, DisplayName: event.DisplayName}
		}
	}
	if actorEmail != "" {
		if _, exists := byEmail[actorEmail]; !exists {
			byEmail[actorEmail] = attendanceMember{
				Email:       actorEmail,
				DisplayName: attendanceWorkDisplayName(events, actorEmail),
			}
		}
	}
	result := make([]attendanceMember, 0, len(byEmail))
	for _, member := range byEmail {
		if member.DisplayName == "" {
			member.DisplayName = member.Email
		}
		result = append(result, member)
	}
	sort.Slice(result, func(left int, right int) bool {
		return result[left].DisplayName < result[right].DisplayName
	})
	return result
}

func attendanceWorkDisplayName(events []attendanceEvent, email string) string {
	for _, event := range events {
		if strings.EqualFold(event.Email, email) && strings.TrimSpace(event.DisplayName) != "" {
			return strings.TrimSpace(event.DisplayName)
		}
	}
	if atIndex := strings.Index(email, "@"); atIndex > 0 {
		return email[:atIndex]
	}
	return email
}
