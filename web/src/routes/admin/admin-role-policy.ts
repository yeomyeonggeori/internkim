import type { AdminSession, UserRole } from './admin-types';

type LegacyAdminSession = Omit<AdminSession, 'isAdmin'> & { isAdmin?: boolean };

export function adminSessionRole(session: LegacyAdminSession | null): UserRole {
	if (!session) return 'member';
	if (session?.role) return session.role;
	return session.isAdmin === true ? 'admin' : 'member';
}

export function canManageOrganization(role: UserRole): boolean {
	return role === 'admin';
}
