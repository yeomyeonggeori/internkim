import { describe, expect, test } from 'bun:test';
import type { UserRecord } from '../../../src/routes/admin/admin-types';
import {
	isOrgProfileChanged,
	isPositionLevelInvalid,
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
				groupIDs: [' operations ', 'engineering', 'operations'],
				projectIDs: [' blueclaw ', 'admin', 'blueclaw'],
				teamRole: ' frontend ',
				employmentStatus: undefined,
				isOrgchartVisible: undefined
			})
		);

		expect(record.jobTitle).toBe('Engineer');
		expect(record.primaryGroupID).toBe('engineering');
		expect(record.group).toBe('engineering');
		expect(record.groupIDs).toEqual(['engineering', 'operations']);
		expect(record.projectIDs).toEqual(['blueclaw', 'admin']);
		expect(record.teamRole).toBe('frontend');
		expect(record.employmentStatus).toBe('active');
		expect(record.isOrgchartVisible).toBe(true);
	});

	test('compares snapshots with trimmed values', () => {
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
					employmentStatus: 'active',
					isOrgchartVisible: true
				}),
				original
			)
		).toBe(false);
	});

	test('builds a normalized profile update payload', () => {
		const update = orgProfileUpdate(
			userRecord({
				jobTitle: ' Lead ',
				positionLevel: 2,
				primaryGroupID: 'leadership',
				groupIDs: ['engineering', 'leadership', 'engineering'],
				supervisorID: 'manager-1',
				projectIDs: [' blueclaw ', 'blueclaw'],
				teamRole: ' owner ',
				employmentStatus: 'leave',
				isOrgchartVisible: false
			})
		);

		expect(update).toEqual({
			userID: 'user-1',
			email: 'user@example.com',
			jobTitle: 'Lead',
			group: 'leadership',
			positionLevel: 2,
			primaryGroupID: 'leadership',
			groupIDs: ['leadership', 'engineering'],
			supervisorID: 'manager-1',
			projectIDs: ['blueclaw'],
			teamRole: 'owner',
			employmentStatus: 'leave',
			isOrgchartVisible: false
		});
	});

	test('validates position level as a positive integer', () => {
		expect(isPositionLevelInvalid(userRecord({ positionLevel: undefined }))).toBe(false);
		expect(isPositionLevelInvalid(userRecord({ positionLevel: 0 }))).toBe(true);
		expect(isPositionLevelInvalid(userRecord({ positionLevel: 1 }))).toBe(false);
		expect(isPositionLevelInvalid(userRecord({ positionLevel: 1.5 }))).toBe(true);
		expect(isPositionLevelInvalid(userRecord({ positionLevel: -1 }))).toBe(true);
	});
});
