import { describe, expect, test } from 'bun:test';
import {
	adminSessionRole,
	canManageOrganization,
	canViewAdminSection,
	firstVisibleAdminSection
} from '../../src/routes/admin/admin-role-policy';
import type { AdminSection, AdminSession, UserRole } from '../../src/routes/admin/admin-types';

const allSections: AdminSection[] = [
	'users',
	'credentials',
	'bot',
	'settings',
	'workSettings',
	'leaveSettings'
];

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
		...(role ? { role } : {})
	};
}

describe('admin role policy', () => {
	test('allows full admins to view every admin section', () => {
		expect(visibleSections('admin')).toEqual(allSections);
	});

	test('only a full admin manages the organization from the employee page', () => {
		expect(canManageOrganization('admin')).toBe(true);
		expect(canManageOrganization('member')).toBe(false);
	});

	test('does not expose admin sections to members', () => {
		expect(visibleSections('member')).toEqual([]);
	});

	test('selects the first visible section for the current role', () => {
		expect(firstVisibleAdminSection('admin', allSections)).toBe('users');
		expect(firstVisibleAdminSection('member', allSections)).toBe(null);
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
