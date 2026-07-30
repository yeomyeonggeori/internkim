package admind

import (
	"net/http"
)

// handleBuzzRelayConfig gives the browser the Buzz relay URL. It returns the
// public gateway URL (wss://<domain>/relay) so an external Buzz app on another
// network can connect; internal services use the loopback relay directly.
func (service *Service) handleBuzzRelayConfig(responseWriter http.ResponseWriter, request *http.Request) {
	service.writeJSON(responseWriter, map[string]string{
		"relayURL": service.publicRelayURL(request),
	})
}
