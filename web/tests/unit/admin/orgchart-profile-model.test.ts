import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import {
	isOrgProfileChanged,
	normalizeOrgProfileRecord,
	orgProfileSnapshot,
	orgProfileUpdate
} from '../../../src/routes/admin/orgchart-profile-model';

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

describe('orgchart profile model', () => {
	test('normalizes legacy group metadata and editable strings', () => {
		const record = normalizeOrgProfileRecord(
			userRecord({
				jobTitle: '  Engineer  ',
				group: 'engineering',
				groupIDs: [' operations ', 'engineering', 'operations']
			})
		);

		expect(record.jobTitle).toBe('Engineer');
		expect(record.primaryGroupID).toBe('engineering');
		expect(record.group).toBe('engineering');
		expect(record.groupIDs).toEqual(['engineering']);
	});

	test('falls back to legacy group metadata when primary group is empty', () => {
		const record = normalizeOrgProfileRecord(
			userRecord({
				primaryGroupID: '',
				group: 'operations',
				groupIDs: ['engineering']
			})
		);

		expect(record.primaryGroupID).toBe('operations');
		expect(record.group).toBe('operations');
		expect(record.groupIDs).toEqual(['operations']);
	});

	test('falls back to the first legacy group id when primary and group are empty', () => {
		const record = normalizeOrgProfileRecord(
			userRecord({
				primaryGroupID: '',
				group: '',
				groupIDs: ['engineering', 'operations']
			})
		);

		expect(record.primaryGroupID).toBe('engineering');
		expect(record.group).toBe('engineering');
		expect(record.groupIDs).toEqual(['engineering']);
	});

	test('compares snapshots with minimal editable values', () => {
		const original = orgProfileSnapshot(
			userRecord({
				jobTitle: 'Designer',
				primaryGroupID: 'design',
				groupIDs: ['design'],
				projectIDs: ['brand'],
				teamRole: 'system',
				employmentStatus: 'active',
				isOrgchartVisible: true
			})
		);

		expect(
			isOrgProfileChanged(
				userRecord({
					jobTitle: '  Designer  ',
					primaryGroupID: 'design',
					groupIDs: ['design'],
					projectIDs: [' brand '],
					teamRole: ' system ',
					employmentStatus: 'leave',
					isOrgchartVisible: false
				}),
				original
			)
		).toBe(false);
	});

	test('builds a normalized profile update payload', () => {
		const update = orgProfileUpdate(
			userRecord({
				jobTitle: ' Lead ',
				primaryGroupID: 'leadership',
				groupIDs: ['engineering', 'leadership', 'engineering'],
				supervisorID: 'manager-1'
			})
		);

		expect(update).toEqual({
			userID: 'user-1',
			email: 'user@example.com',
			jobTitle: 'Lead',
			group: 'leadership',
			primaryGroupID: 'leadership',
			groupIDs: ['leadership'],
			supervisorID: 'manager-1'
		});
	});
});
