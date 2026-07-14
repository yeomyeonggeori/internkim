import { describe, expect, test } from 'bun:test';
import type { OrgGroup, UserRecord } from '../../../src/routes/orgchart/orgchart-types';
import {
	filterOrgchartRecords,
	orgchartFilterOptions,
	type OrgchartDirectoryFilters
} from '../../../src/routes/orgchart/orgchart-directory-model';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		...overrides
	};
}

describe('orgchart directory model', () => {
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

		const filters: OrgchartDirectoryFilters = {
			query: 'engineer',
			groupID: 'engineering'
		};

		expect(filterOrgchartRecords(records, filters).map((record) => record.userID)).toEqual(['ada']);
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

		const options = orgchartFilterOptions(records, groups);

		expect(options.groups.map((group) => group.id)).toEqual(['engineering', 'operations']);
		expect(options.hasUnassigned).toBe(true);
	});
});
