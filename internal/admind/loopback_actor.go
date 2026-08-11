package admind

import (
	"net/http"
	"strings"
)

// A caller on loopback is another service on this machine acting for a person
// it has already identified, so it names them. Anything arriving from outside
// has to prove who it is the ordinary way, which is why the header is read only
// after isLocalRequest and never instead of a session.
func (service *Service) actorEmailAllowingLoopback(request *http.Request) string {
	if actorEmail := service.webActorEmail(request); actorEmail != "" {
		return actorEmail
	}
	if !isLocalRequest(request) {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(request.Header.Get(flowRequesterEmailHeader)))
}
