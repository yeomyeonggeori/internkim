package admind

import (
	"context"
)

func (service *Service) adminSessionRole(ctx context.Context, callerEmail string) string {
	if service.isTaskAdminEmail(ctx, callerEmail) {
		return adminUserRoleAdmin
	}
	return service.currentAdminUserRole(ctx, callerEmail)
}
