package admind

import (
	"net/http"
	"strings"
)

// handleBuzzRelayConfig gives the browser the Buzz relay URL so it can sign and
// publish messages itself. The relay URL is not a secret (it also appears in
// invite deep links), so this is readable by any authenticated messenger user.
func (service *Service) handleBuzzRelayConfig(responseWriter http.ResponseWriter, request *http.Request) {
	service.writeJSON(responseWriter, map[string]string{
		"relayURL": strings.TrimSpace(service.Configuration.BuzzRelayURL),
	})
}
