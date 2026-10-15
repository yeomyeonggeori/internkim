package admind

import (
	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

const (
	adminUserRoleAdmin  = centralplane.MemberRoleAdmin
	adminUserRoleMember = centralplane.MemberRoleMember
)

func normalizeAdminUserRole(role string) string {
	return centralplane.NormalizeMemberRole(role)
}
