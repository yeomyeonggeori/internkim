import { describe, expect, test } from 'bun:test';
import { adminSessionRole, canManageOrganization } from '../../src/routes/admin/admin-role-policy';
import type { AdminSession, UserRole } from '../../src/routes/admin/admin-types';

function session(role: UserRole | undefined, isAdmin?: boolean): Omit<AdminSession, 'isAdmin'> & { isAdmin?: boolean } {
	return {
		email: 'admin@example.com',
		claimedAdminEmail: 'admin@example.com',
		...(isAdmin === undefined ? {} : { isAdmin }),
		...(role ? { role } : {})
	};
}

describe('admin role policy', () => {
	test('only a full admin manages the organization from the employee page', () => {
		expect(canManageOrganization('admin')).toBe(true);
		expect(canManageOrganization('member')).toBe(false);
	});

	test('derives roles from current and legacy admin sessions', () => {
		expect(adminSessionRole(session('member', false))).toBe('member');
		expect(adminSessionRole(session('admin', false))).toBe('admin');
		expect(adminSessionRole(session(undefined, true))).toBe('admin');
		expect(adminSessionRole(session(undefined, false))).toBe('member');
		expect(adminSessionRole(session(undefined))).toBe('member');
		expect(adminSessionRole(null)).toBe('member');
	});
});
