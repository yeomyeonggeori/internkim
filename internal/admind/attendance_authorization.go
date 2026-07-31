package admind

import "net/http"

func (service *Service) canManageAttendance(request *http.Request) bool {
	role := service.adminSessionRole(
		request.Context(),
		service.adminConsoleActorEmail(request),
	)
	return role == adminUserRoleAdmin || role == adminUserRoleOperationsAdmin
}
