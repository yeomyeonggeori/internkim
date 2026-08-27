package admind

import "net/http"

func (service *Service) actorEmailAllowingAssertedRequester(request *http.Request) string {
	if actorEmail := service.webActorEmail(request); actorEmail != "" {
		return actorEmail
	}
	return assertedRequesterEmail(request)
}
