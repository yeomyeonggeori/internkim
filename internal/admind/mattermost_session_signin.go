package admind

import (
	"net/http"
	"strings"
)

// Mattermost is served from the same host as this web app, so a browser already
// signed into it carries that session cookie here too. Reading it saves the
// person a second sign-in with the same credential they just used.
//
// This is a bridge for people who arrived through the device, and it goes away
// with the device path. Everything it needs lives in this file and it is off
// unless the deployment writes MattermostSessionSignInPath, so removing it is
// deleting one file, one field and one call in webActorEmail.
func (service *Service) mattermostSessionSignInIsOffered() bool {
	return strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostSessionSignInPath)) == "1"
}

func (service *Service) emailOfMattermostSession(request *http.Request) string {
	if !service.mattermostSessionSignInIsOffered() {
		return ""
	}
	cookieHeader := mattermostSessionCookieHeader(request)
	if cookieHeader == "" {
		return ""
	}
	userRecord, found := service.mattermostSessionUser(request, cookieHeader)
	if !found || userRecord.IsBot || userRecord.DeleteAt != 0 {
		return ""
	}
	email := strings.ToLower(strings.TrimSpace(userRecord.Email))
	if email == "" || !service.isTaskStaffActor(request.Context(), email) {
		return ""
	}
	return email
}
