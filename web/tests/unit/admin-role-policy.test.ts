import { describe, expect, test } from 'bun:test';
import {
	adminSessionRole,
	canManageOrgchart,
	canViewAdminSection,
	firstVisibleAdminSection
} from '../../src/routes/admin/admin-role-policy';
import type { AdminSection, AdminSession, UserRole } from '../../src/routes/admin/admin-types';

const allSections: AdminSection[] = ['device', 'users', 'credentials', 'backup', 'bot', 'settings', 'network'];

function visibleSections(role: UserRole): AdminSection[] {
	return allSections.filter((section) => canViewAdminSection(role, section));
}

function session(role: UserRole | undefined, isAdmin?: boolean): Omit<AdminSession, 'isAdmin'> & { isAdmin?: boolean } {
	return {
		email: 'admin@example.com',
		claimedAdminEmail: 'admin@example.com',
		...(isAdmin === undefined ? {} : { isAdmin }),
		isClaimed: true,
		bootstrapStatus: 'claimed',
		deviceManaged: true,
		...(role ? { role } : {})
	};
}

describe('admin role policy', () => {
	test('allows full admins to view every admin section', () => {
		expect(visibleSections('admin')).toEqual(allSections);
	});

	test('limits operations admins to people operations sections', () => {
		expect(visibleSections('operationsAdmin')).toEqual(['users', 'settings']);
	});

	test('allows operations admins and full admins to manage orgchart from the employee page', () => {
		expect(canManageOrgchart('admin')).toBe(true);
		expect(canManageOrgchart('operationsAdmin')).toBe(true);
		expect(canManageOrgchart('member')).toBe(false);
	});

	test('does not expose admin sections to members', () => {
		expect(visibleSections('member')).toEqual([]);
	});

	test('selects the first visible section for the current role', () => {
		expect(firstVisibleAdminSection('operationsAdmin', allSections)).toBe('users');
		expect(firstVisibleAdminSection('member', allSections)).toBe(null);
	});

	test('derives roles from current and legacy admin sessions', () => {
		expect(adminSessionRole(session('operationsAdmin', false))).toBe('operationsAdmin');
		expect(adminSessionRole(session(undefined, true))).toBe('admin');
		expect(adminSessionRole(session(undefined, false))).toBe('member');
		expect(adminSessionRole(session(undefined))).toBe('member');
		expect(adminSessionRole(null)).toBe('member');
	});
});
