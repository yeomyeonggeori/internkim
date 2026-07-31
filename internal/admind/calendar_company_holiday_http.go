package admind

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const calendarCompanyHolidaysAdminPath = "/company-holidays"

func (service *Service) handleCalendarCompanyHolidays(
	responseWriter http.ResponseWriter,
	request *http.Request,
	path string,
) {
	switch {
	case request.Method == http.MethodGet && path == calendarCompanyHolidaysAdminPath:
		holidays, errorValue := service.listCalendarCompanyHolidays(request.Context())
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		service.writeJSON(responseWriter, calendarCompanyHolidaysResponse{Holidays: holidays})
	case request.Method == http.MethodPost && path == calendarCompanyHolidaysAdminPath:
		input, errorValue := decodeCalendarCompanyHolidayInput(request)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		holiday, errorValue := service.createCalendarCompanyHoliday(request.Context(), input, time.Now())
		if calendarCompanyHolidayHTTPError(responseWriter, errorValue) {
			return
		}
		responseWriter.WriteHeader(http.StatusCreated)
		service.writeJSON(responseWriter, holiday)
	case (request.Method == http.MethodPut || request.Method == http.MethodDelete) &&
		strings.HasPrefix(path, calendarCompanyHolidaysAdminPath+"/"):
		holidayID, errorValue := calendarCompanyHolidayIDFromPath(path)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		if request.Method == http.MethodDelete {
			errorValue = service.deleteCalendarCompanyHoliday(request.Context(), holidayID)
			if calendarCompanyHolidayHTTPError(responseWriter, errorValue) {
				return
			}
			responseWriter.WriteHeader(http.StatusNoContent)
			return
		}
		input, errorValue := decodeCalendarCompanyHolidayInput(request)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		holiday, errorValue := service.updateCalendarCompanyHoliday(request.Context(), holidayID, input, time.Now())
		if calendarCompanyHolidayHTTPError(responseWriter, errorValue) {
			return
		}
		service.writeJSON(responseWriter, holiday)
	default:
		http.NotFound(responseWriter, request)
	}
}

func decodeCalendarCompanyHolidayInput(request *http.Request) (calendarCompanyHolidayInput, error) {
	var input calendarCompanyHolidayInput
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&input); errorValue != nil {
		return calendarCompanyHolidayInput{}, errorValue
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		return calendarCompanyHolidayInput{}, errors.New("request body must contain one JSON object")
	}
	return normalizeCalendarCompanyHolidayInput(input)
}

func calendarCompanyHolidayIDFromPath(path string) (string, error) {
	encodedID := strings.TrimPrefix(path, calendarCompanyHolidaysAdminPath+"/")
	if encodedID == "" || strings.Contains(encodedID, "/") {
		return "", errors.New("invalid company holiday id")
	}
	holidayID, errorValue := url.PathUnescape(encodedID)
	if errorValue != nil || strings.TrimSpace(holidayID) == "" {
		return "", errors.New("invalid company holiday id")
	}
	return holidayID, nil
}

func calendarCompanyHolidayHTTPError(responseWriter http.ResponseWriter, errorValue error) bool {
	if errorValue == nil {
		return false
	}
	if errors.Is(errorValue, errCalendarCompanyHolidayNotFound) {
		http.Error(responseWriter, errorValue.Error(), http.StatusNotFound)
		return true
	}
	http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
	return true
}
