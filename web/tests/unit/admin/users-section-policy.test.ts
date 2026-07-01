import { describe, expect, test } from 'bun:test';
import { adminText } from '../../../src/routes/admin/text';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import {
	canManageUserRecord,
	hasUserCircle,
	isReservedCircleID,
	isValidHandle,
	normalizeUserCircles,
	sortUserRecordsByHireDate,
	userAdminCount,
	userRoleLabel,
	userRoleOptions,
	visibleUserCircles
} from '../../../src/routes/admin/users-section-policy';

const adminRecord: UserRecord = {
	userID: 'admin-user',
	handle: 'admin',
	name: 'Admin User',
	email: 'admin@example.com',
	hireDate: '2026-01-01',
	role: 'admin',
	circles: ['staff', 'admin']
};

const memberRecord: UserRecord = {
	userID: 'member-user',
	handle: 'member',
	name: 'Member User',
	email: 'member@example.com',
	hireDate: '2026-01-02',
	role: 'member',
	circles: ['staff']
};

describe('users section policy', () => {
	test('hides full admin role options from operations admins', () => {
		expect(userRoleOptions(adminText.ko, false).map((option) => option.value)).toEqual(['member', 'operationsAdmin']);
		expect(userRoleOptions(adminText.ko, true).map((option) => option.value)).toEqual(['member', 'operationsAdmin', 'admin']);
		expect(userRoleLabel(adminText.ko, 'operationsAdmin')).toBe('운영자');
	});

	test('protects admin users from limited admins', () => {
		expect(canManageUserRecord(false, adminRecord)).toBe(false);
		expect(canManageUserRecord(false, memberRecord)).toBe(true);
		expect(canManageUserRecord(true, adminRecord)).toBe(true);
	});

	test('normalizes reserved and role-backed circles', () => {
		expect(isReservedCircleID(' Admin ')).toBe(true);
		expect(
			visibleUserCircles([
				{ circleID: 'staff', displayName: 'Staff' },
				{ circleID: 'admin', displayName: 'Admin' },
				{ circleID: 'team', displayName: 'Team' }
			]).map((circle) => circle.circleID)
		).toEqual(['staff', 'team']);
		expect(normalizeUserCircles(['Staff', 'team'], 'admin')).toEqual(['staff', 'team', 'admin']);
		expect(hasUserCircle({ ...memberRecord, circles: ['Team'] }, 'team')).toBe(true);
	});

	test('validates handles and sorts users by hire date', () => {
		expect(isValidHandle(' chanhee ')).toBe(true);
		expect(isValidHandle('1chanhee')).toBe(false);
		expect(userAdminCount([adminRecord, memberRecord])).toBe(1);
		expect(sortUserRecordsByHireDate([memberRecord, adminRecord]).map((record) => record.email)).toEqual(['admin@example.com', 'member@example.com']);
	});
});
