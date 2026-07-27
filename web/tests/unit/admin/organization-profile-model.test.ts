import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import {
	isOrgProfileChanged,
	normalizeOrgProfileRecord,
	orgProfileSnapshot,
	orgProfileUpdate
} from '../../../src/routes/admin/organization-profile-model';

function userRecord(overrides: Partial<UserRecord> = {}): UserRecord {
	return {
		userID: 'user-1',
		handle: 'user',
		name: 'User',
		email: 'user@example.com',
		role: 'member',
		...overrides
	};
}

describe('organization profile model', () => {
	test('normalizes group metadata and editable strings', () => {
		const record = normalizeOrgProfileRecord(
			userRecord({
				jobTitle: '  Engineer  ',
				groupID: 'engineering'
			})
		);

		expect(record.jobTitle).toBe('Engineer');
		expect(record.groupID).toBe('engineering');
	});

	test('falls back to an empty group when none is set', () => {
		const record = normalizeOrgProfileRecord(userRecord());

		expect(record.groupID).toBe('');
	});

	test('compares snapshots with minimal editable values', () => {
		const original = orgProfileSnapshot(
			userRecord({
				jobTitle: 'Designer',
				groupID: 'design',
				handle: 'designer',
				name: 'Designer'
			})
		);

		expect(
			isOrgProfileChanged(
				userRecord({
					jobTitle: '  Designer  ',
					groupID: 'design',
					handle: 'designer-next',
					name: 'Designer Next'
				}),
				original
			)
		).toBe(false);
	});

	test('builds a normalized profile update payload', () => {
		const update = orgProfileUpdate(
			userRecord({
				jobTitle: ' Lead ',
				groupID: 'leadership',
				supervisorID: 'manager-1'
			})
		);

		expect(update).toEqual({
			userID: 'user-1',
			email: 'user@example.com',
			jobTitle: 'Lead',
			groupID: 'leadership',
			phoneNumber: '',
			supervisorID: 'manager-1'
		});
	});
});
