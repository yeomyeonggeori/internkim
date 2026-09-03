package admind

import (
	"context"
	"strings"
)

func (service *Service) adminSessionRole(ctx context.Context, callerEmail string) string {
	if service.isTaskAdminEmail(ctx, callerEmail) {
		return adminUserRoleAdmin
	}
	return service.currentAdminUserRole(ctx, callerEmail)
}

func isReservedAdminCircleID(circleID string) bool {
	switch strings.ToLower(strings.TrimSpace(circleID)) {
	case "member", "admin":
		return true
	default:
		return false
	}
}
