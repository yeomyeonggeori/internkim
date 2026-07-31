package admind

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"time"
)

type attendanceWorkPolicyResponse struct {
	Policy       attendanceWorkPolicy `json:"policy"`
	CurrentMonth string               `json:"currentMonth"`
	HolidayDates []string             `json:"holidayDates"`
	TimeZone     string               `json:"timeZone"`
}

func (service *Service) handleAttendanceWorkPolicy(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	service.handleAttendanceWorkPolicyAt(
		responseWriter,
		request,
		time.Now(),
		service.readCalendarHolidayDatesForRange,
	)
}

func (service *Service) handleAttendanceWorkPolicyAt(
	responseWriter http.ResponseWriter,
	request *http.Request,
	now time.Time,
	holidayReader attendanceHolidayDatesReader,
) {
	var policy attendanceWorkPolicy
	if request.Method == http.MethodGet {
		var errorValue error
		policy, errorValue = service.readAttendanceWorkPolicy(request.Context())
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		var revision attendanceWorkPolicyRevision
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if errorValue := decoder.Decode(&revision); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
			http.Error(responseWriter, "request must contain one JSON object", http.StatusBadRequest)
			return
		}
		effectiveDate := now.In(service.workspaceTimeZone().location).Format(time.DateOnly)
		var errorValue error
		policy, errorValue = service.saveAttendanceWorkPolicyRevision(
			request.Context(),
			revision,
			effectiveDate,
			now,
		)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
	}
	response, errorValue := service.attendanceWorkPolicyResponse(
		request.Context(),
		policy,
		now,
		holidayReader,
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) attendanceWorkPolicyResponse(
	ctx context.Context,
	policy attendanceWorkPolicy,
	now time.Time,
	holidayReader attendanceHolidayDatesReader,
) (attendanceWorkPolicyResponse, error) {
	timeZone := service.workspaceTimeZone()
	localNow := now.In(timeZone.location)
	monthStart := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, timeZone.location)
	monthEnd := monthStart.AddDate(0, 1, 0)
	holidaySet, errorValue := holidayReader(ctx, monthStart, monthEnd)
	if errorValue != nil {
		return attendanceWorkPolicyResponse{}, errorValue
	}
	holidayDates := make([]string, 0, len(holidaySet))
	for date := range holidaySet {
		holidayDates = append(holidayDates, date)
	}
	sort.Strings(holidayDates)
	return attendanceWorkPolicyResponse{
		Policy:       policy,
		CurrentMonth: monthStart.Format("2006-01"),
		HolidayDates: holidayDates,
		TimeZone:     timeZone.name,
	}, nil
}
