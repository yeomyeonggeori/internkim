import type { AdminSection, AdminSession, UserRole } from './admin-types';

const operationsAdminSections: AdminSection[] = ['users', 'settings', 'apiTokens'];
const memberSections: AdminSection[] = ['apiTokens'];
type LegacyAdminSession = Omit<AdminSession, 'isAdmin'> & { isAdmin?: boolean };

export function adminSessionRole(session: LegacyAdminSession | null): UserRole {
	if (!session) return 'member';
	if (session?.role) return session.role;
	return session.isAdmin === true ? 'admin' : 'member';
}

export function canViewAdminSection(role: UserRole, section: AdminSection): boolean {
	if (role === 'admin') return true;
	if (role === 'operationsAdmin') return operationsAdminSections.includes(section);
	return memberSections.includes(section);
}

export function canManageOrgchart(role: UserRole): boolean {
	return role === 'admin' || role === 'operationsAdmin';
}

export function firstVisibleAdminSection(role: UserRole, sections: AdminSection[]): AdminSection | null {
	return sections.find((section) => canViewAdminSection(role, section)) ?? null;
}
