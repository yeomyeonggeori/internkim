package admind

import (
	"net/http"
)

func (service *Service) handleBuzzRelayConfig(responseWriter http.ResponseWriter, request *http.Request) {
	service.writeJSON(responseWriter, map[string]string{
		"relayURL": service.buzzRelayEffectiveURL(),
	})
}
