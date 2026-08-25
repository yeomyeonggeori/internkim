package admind

import (
	"log"
	"net/http"
)

// The company is where somebody is invited and this device is where they are
// given a Linux user, so the device has to hear about it. Waiting for the timer
// leaves whoever was just invited unknown to the agent for as long as it takes
// to come round, and they find that out by being refused.
func (service *Service) handleDirectoryChanged(responseWriter http.ResponseWriter, request *http.Request) {
	log.Printf("the company says its directory changed, so this device reads it again")
	service.triggerUsersSync(request.Context())
	responseWriter.WriteHeader(http.StatusAccepted)
}
