package admind

import (
	"log"
	"net/http"
)

// An invited person is not yet given a Linux user or a messenger identity, so
// the device has to hear about the invitation. Waiting for the timer leaves
// whoever was just invited unknown to the agent for as long as it takes to come
// round, and they find that out by being refused. The credential recording runs
// before the answer so the caller can heal its projection the moment this
// returns.
func (service *Service) handleDirectoryChanged(responseWriter http.ResponseWriter, request *http.Request) {
	log.Printf("the company says its directory changed, so this device reads it again")
	service.triggerUsersSync(request.Context())
	recording := service.recordBuzzCredentials(request.Context())
	log.Printf("buzz credentials after the directory changed: %s", recording)
	responseWriter.WriteHeader(http.StatusAccepted)
}
