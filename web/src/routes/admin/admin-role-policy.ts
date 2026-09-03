import type { AdminSection, AdminSession, UserRole } from './admin-types';

const memberSections: AdminSection[] = ['apiTokens'];
type LegacyAdminSession = Omit<AdminSession, 'isAdmin'> & { isAdmin?: boolean };

export function adminSessionRole(session: LegacyAdminSession | null): UserRole {
	if (!session) return 'member';
	if (session?.role) return session.role;
	return session.isAdmin === true ? 'admin' : 'member';
}

export function canViewAdminSection(role: UserRole, section: AdminSection): boolean {
	if (role === 'admin') return true;
	return memberSections.includes(section);
}

export function canManageOrganization(role: UserRole): boolean {
	return role === 'admin';
}

export function firstVisibleAdminSection(role: UserRole, sections: AdminSection[]): AdminSection | null {
	return sections.find((section) => canViewAdminSection(role, section)) ?? null;
}
