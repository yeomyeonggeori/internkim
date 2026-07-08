import { describe, expect, test } from 'bun:test';
import type { OrgGroup, UserRecord } from '../../../src/routes/admin/admin-types';
import {
	filterOrgchartRecords,
	orgchartCanvasModel,
	orgchartFilterOptions,
	unassignedGroupID,
	type OrgchartDirectoryFilters,
	type OrgchartTreeNode
} from '../../../src/routes/orgchart/orgchart-directory-model';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
		role: 'member',
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

	test('builds a supervisor based canvas with roots and team columns', () => {
		const groups: OrgGroup[] = [
			{ id: 'engineering', name: '엔지니어링' },
			{ id: 'operations', name: '운영' }
		];
		const records = [
			userRecord({ userID: 'ceo', name: 'CEO', primaryGroupID: 'leadership', groupIDs: ['leadership'] }),
			userRecord({ userID: 'ada', name: 'Ada', hireDate: '2026-02-01', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'ceo' }),
			userRecord({ userID: 'grace', name: 'Grace', hireDate: '2026-01-01', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'ada' }),
			userRecord({ userID: 'lin', name: 'Lin', hireDate: '2026-01-02', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'ada' }),
			userRecord({ userID: 'tess', name: 'Tess', hireDate: '2026-01-03', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'grace' }),
			userRecord({ userID: 'min', name: 'Min', primaryGroupID: 'operations', groupIDs: ['operations'], supervisorID: 'ceo' }),
			userRecord({ userID: 'yuna', name: 'Yuna', supervisorID: 'ceo' })
		];

		const model = orgchartCanvasModel(records, groups, '팀 미지정');

		expect(model.roots.map((record) => record.userID)).toEqual(['ceo']);
		expect(model.columns.map((column) => column.id)).toEqual(['engineering', 'operations', unassignedGroupID]);
		expect(model.columns[0]?.nodes.map((node) => [node.record.userID, node.depth])).toEqual([
			['ada', 1],
			['grace', 2],
			['lin', 2],
			['tess', 3]
		]);
		expect(model.columns[0]?.treeRoots.map((node) => treeNodeIDs(node))).toEqual([{ id: 'ada', children: [{ id: 'grace', children: [{ id: 'tess', children: [] }] }, { id: 'lin', children: [] }] }]);
		expect(model.columns[2]?.records.map((record) => record.userID)).toEqual(['yuna']);
	});

	test('orders people on the same level by hire date before name', () => {
		const groups: OrgGroup[] = [{ id: 'engineering', name: '엔지니어링' }];
		const records = [
			userRecord({ userID: 'root-missing-zara', name: 'Zara Root' }),
			userRecord({ userID: 'root-dated', name: 'Dated Root', hireDate: '2026-01-01' }),
			userRecord({ userID: 'root-missing-ada', name: 'Ada Root' }),
			userRecord({ userID: 'lead', name: 'Lead', hireDate: '2026-01-02', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'root-dated' }),
			userRecord({ userID: 'sibling-missing-zara', name: 'Zara Sibling', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'lead' }),
			userRecord({ userID: 'sibling-dated-bora', name: 'Bora Sibling', hireDate: '2026-02-01', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'lead' }),
			userRecord({ userID: 'sibling-dated-ada', name: 'Ada Sibling', hireDate: '2026-02-01', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'lead' }),
			userRecord({ userID: 'sibling-missing-ada', name: 'Ada Missing', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'lead' })
		];

		const model = orgchartCanvasModel(records, groups, '팀 미지정');
		const leadNode = model.columns[0]?.treeRoots[0];

		expect(model.roots.map((record) => record.userID)).toEqual(['root-dated', 'root-missing-ada', 'root-missing-zara']);
		expect(leadNode?.children.map((node) => node.record.userID)).toEqual([
			'sibling-dated-ada',
			'sibling-dated-bora',
			'sibling-missing-ada',
			'sibling-missing-zara'
		]);
	});

	test('promotes missing supervisors and supervisor cycles to roots', () => {
		const groups: OrgGroup[] = [{ id: 'engineering', name: '엔지니어링' }];
		const records = [
			userRecord({ userID: 'orphan', name: 'Orphan', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'missing' }),
			userRecord({ userID: 'cycle-a', name: 'Cycle A', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'cycle-b' }),
			userRecord({ userID: 'cycle-b', name: 'Cycle B', primaryGroupID: 'engineering', groupIDs: ['engineering'], supervisorID: 'cycle-a' })
		];

		const model = orgchartCanvasModel(records, groups, '팀 미지정');

		expect(model.roots.map((record) => record.userID)).toEqual(['cycle-a', 'cycle-b', 'orphan']);
		expect(model.columns).toEqual([]);
	});
});

function treeNodeIDs(node: OrgchartTreeNode): unknown {
	return {
		id: node.record.userID,
		children: node.children.map((child) => treeNodeIDs(child))
	};
}
