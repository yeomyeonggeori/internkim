import { describe, expect, test } from 'bun:test';
import {
	hasUserCircle,
	normalizeUserCircles,
	toggleUserRecordCircle,
	updateUserRecords
} from '../../src/routes/admin/admin-user-record-changes';
import type { UserRecord } from '../../src/routes/admin/admin-types';

const adminRecord: UserRecord = {
	userID: 'user-1',
	handle: 'chanhee',
	name: '최견본',
	email: 'chanhee@example.com',
	role: 'admin',
	circles: ['staff', 'c-level']
};

describe('admin user record changes', () => {
	test('normalizes staff and admin circles from role', () => {
		expect(normalizeUserCircles([' staff ', 'C-Level', ''], 'admin')).toEqual(['staff', 'c-level', 'admin']);
		expect(normalizeUserCircles([], 'member')).toEqual(['staff']);
	});

	test('checks user circle membership using normalized circles', () => {
		expect(hasUserCircle(adminRecord, 'admin')).toBe(true);
		expect(hasUserCircle(adminRecord, 'hr-compensation')).toBe(false);
	});

	test('updates one user record without mutating the original array', () => {
		const records = [adminRecord];
		const updatedRecords = updateUserRecords(records, adminRecord.email, { name: '김인턴' });

		expect(updatedRecords).not.toBe(records);
		expect(updatedRecords[0]).toEqual({ ...adminRecord, name: '김인턴' });
		expect(records[0].name).toBe('최견본');
	});

	test('toggles a circle while preserving required role circles', () => {
		const records = [adminRecord];
		const addedRecords = toggleUserRecordCircle(records, adminRecord.email, 'hr-compensation');
		const removedRecords = toggleUserRecordCircle(addedRecords, adminRecord.email, 'c-level');

		expect(addedRecords[0].circles).toEqual(['staff', 'c-level', 'admin', 'hr-compensation']);
		expect(removedRecords[0].circles).toEqual(['staff', 'admin', 'hr-compensation']);
		expect(toggleUserRecordCircle(records, adminRecord.email, 'staff')).toBe(records);
	});
});
