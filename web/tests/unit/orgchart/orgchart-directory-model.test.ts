import { describe, expect, test } from 'bun:test';
import type { OrgGroup, UserRecord } from '../../../src/lib/organization/types';
import {
	filterOrganizationRecords,
	organizationFilterOptions,
	type OrganizationDirectoryFilters
} from '../../../src/routes/organization/organization-directory-model';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		...overrides
	};
}

describe('organization directory model', () => {
	test('filters by search and organization', () => {
		const records = [
			userRecord({
				userID: 'ada',
				name: 'Ada Kim',
				email: 'ada@example.com',
				jobTitle: 'Engineering Lead',
				primaryGroupID: 'engineering',
				groupIDs: ['engineering']
			}),
			userRecord({
				userID: 'grace',
				name: 'Grace Lee',
				email: 'grace@example.com',
				jobTitle: 'Operations Manager',
				primaryGroupID: 'operations',
				groupIDs: ['operations']
			})
		];

		const filters: OrganizationDirectoryFilters = {
			query: 'engineer',
			groupID: 'engineering'
		};

		expect(filterOrganizationRecords(records, filters).map((record) => record.userID)).toEqual(['ada']);
	});

	test('returns only organizations and assignment state connected to people', () => {
		const groups: OrgGroup[] = [
			{ id: 'engineering', name: '엔지니어링' },
			{ id: 'operations', name: '운영' },
			{ id: 'empty', name: '미배정' }
		];
		const records = [
			userRecord({ userID: 'ada', groupIDs: ['engineering'] }),
			userRecord({ userID: 'unassigned' }),
			userRecord({ userID: 'operations', groupIDs: ['operations'] })
		];

		const options = organizationFilterOptions(records, groups);

		expect(options.groups.map((group) => group.id)).toEqual(['engineering', 'operations']);
		expect(options.hasUnassigned).toBe(true);
	});

	test('normalizes legacy memberships consistently for filters and assignment state', () => {
		const groups: OrgGroup[] = [{ id: 'engineering', name: '엔지니어링' }];
		const records = [
			userRecord({
				userID: 'ada',
				groupIDs: [' engineering ', 'engineering']
			})
		];

		expect(filterOrganizationRecords(records, { query: '', groupID: 'engineering' }).map((record) => record.userID)).toEqual(['ada']);
		expect(filterOrganizationRecords(records, { query: '', groupID: '__unassigned__' })).toEqual([]);
		expect(organizationFilterOptions(records, groups)).toEqual({ groups, hasUnassigned: false });
	});
});
