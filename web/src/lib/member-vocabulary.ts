export const memberRoles = ['admin', 'member'] as const;

export type MemberRole = (typeof memberRoles)[number];

export const memberStatuses = ['pending', 'invited', 'active', 'departed', 'withdrawn'] as const;

export type MemberStatus = (typeof memberStatuses)[number];

export function memberRoleOf(isAdmin: boolean): MemberRole {
	return isAdmin ? 'admin' : 'member';
}

export function isMemberRole(value: unknown): value is MemberRole {
	return memberRoles.some((role) => role === value);
}

export function isMemberStatus(value: unknown): value is MemberStatus {
	return memberStatuses.some((status) => status === value);
}
