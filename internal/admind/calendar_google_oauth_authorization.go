package admind

import "net/http"

func (service *Service) canManageGoogleOAuth(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	callerEmail := service.adminConsoleActorEmail(request)
	if callerEmail == "" {
		return false
	}
	role := service.adminSessionRole(request.Context(), callerEmail)
	return role == adminUserRoleAdmin || role == adminUserRoleOperationsAdmin
}
