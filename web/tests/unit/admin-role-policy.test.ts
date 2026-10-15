import { describe, expect, test } from 'bun:test';
import { adminSessionRole, canManageOrganization } from '../../src/routes/admin/admin-role-policy';
import type { AdminSession, UserRole } from '../../src/routes/admin/admin-types';

function session(role: UserRole): AdminSession {
	return {
		email: 'admin@example.com',
		claimedAdminEmail: 'admin@example.com',
		isAdmin: role === 'admin',
		role,
		isClaimed: true,
		bootstrapStatus: 'claimed'
	};
}

describe('admin role policy', () => {
	test('only a full admin manages the organization from the employee page', () => {
		expect(canManageOrganization('admin')).toBe(true);
		expect(canManageOrganization('member')).toBe(false);
	});

	test('reads the role the session names, and nobody signed in is a member', () => {
		expect(adminSessionRole(session('admin'))).toBe('admin');
		expect(adminSessionRole(session('member'))).toBe('member');
		expect(adminSessionRole(null)).toBe('member');
	});
});
