package admind

import "net/http"

func (service *Service) canManageGoogleOAuthClient(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	callerEmail := service.adminConsoleActorEmail(request)
	if callerEmail == "" {
		return false
	}
	role := service.currentAdminUserRole(request.Context(), callerEmail)
	return role == adminUserRoleAdmin || role == adminUserRoleOperationsAdmin
}
