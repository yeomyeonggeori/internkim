export const memberRoles = ['admin', 'member'] as const;

export type MemberRole = (typeof memberRoles)[number];

export const memberStatuses = ['pending', 'invited', 'active', 'departed', 'withdrawn'] as const;

export type MemberStatus = (typeof memberStatuses)[number];
