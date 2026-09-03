package admind

import "net/http"

const calendarBelongsToTheCompanyMessage = "the company keeps the calendar, and this device names no company"

func (service *Service) belongsToACompany() bool {
	return service.centralPlane() != nil
}

func writeCalendarBelongsToTheCompany(responseWriter http.ResponseWriter) {
	http.Error(responseWriter, calendarBelongsToTheCompanyMessage, http.StatusGone)
}
