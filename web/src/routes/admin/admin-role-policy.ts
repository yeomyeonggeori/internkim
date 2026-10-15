import type { AdminSession, UserRole } from './admin-types';

export function adminSessionRole(session: AdminSession | null): UserRole {
	return session?.role === 'admin' ? 'admin' : 'member';
}

export function canManageOrganization(role: UserRole): boolean {
	return role === 'admin';
}
