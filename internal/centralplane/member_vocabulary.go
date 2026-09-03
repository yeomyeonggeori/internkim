package centralplane

import "strings"

const (
	MemberRoleAdmin  = "admin"
	MemberRoleMember = "member"
)

const (
	MemberStatusPending   = "pending"
	MemberStatusInvited   = "invited"
	MemberStatusActive    = "active"
	MemberStatusDeparted  = "departed"
	MemberStatusWithdrawn = "withdrawn"
)

func MemberRoles() []string {
	return []string{MemberRoleAdmin, MemberRoleMember}
}

func MemberStatuses() []string {
	return []string{
		MemberStatusPending,
		MemberStatusInvited,
		MemberStatusActive,
		MemberStatusDeparted,
		MemberStatusWithdrawn,
	}
}

func NormalizeMemberRole(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), MemberRoleAdmin) {
		return MemberRoleAdmin
	}
	return MemberRoleMember
}
