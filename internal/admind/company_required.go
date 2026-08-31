package admind

import "net/http"

const calendarBelongsToTheCompanyMessage = "this device keeps its calendar in the company's record, which this address does not reach"

func (service *Service) belongsToACompany() bool {
	return service.centralPlane() != nil
}

func (service *Service) refuseACalendarBesideTheRecord(responseWriter http.ResponseWriter) bool {
	if !service.belongsToACompany() {
		return false
	}
	http.Error(responseWriter, calendarBelongsToTheCompanyMessage, http.StatusGone)
	return true
}
