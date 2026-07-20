import { describe, expect, test } from 'bun:test';
import type { OrgGroup, UserRecord } from '../../../src/lib/orgchart/types';
import {
	moveOrgchartOrganization,
	orgchartOrganizationMovePreview,
	orgchartOrganizationSubtreeIDs,
	orgchartOrganizationTree,
	orgchartOrganizationTreeIndex
} from '../../../src/routes/orgchart/orgchart-organization-tree-model';

function record(userID: string, primaryGroupID = ''): UserRecord {
	return {
		userID,
		handle: userID,
		name: userID,
		email: `${userID}@example.com`,
		primaryGroupID,
		groupIDs: primaryGroupID ? [primaryGroupID] : []
	};
}

describe('orgchart organization tree model', () => {
	test('builds nested nodes with descendant member totals', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'design', name: '디자인팀', parentID: 'product' },
			{ id: 'sales', name: '세일즈' }
		];
		const records = [record('ceo'), record('product-lead', 'product'), record('engineer', 'engineering'), record('designer', 'design'), record('sales', 'sales')];

		const tree = orgchartOrganizationTree(groups, records, '전체 조직');

		expect(tree.root.memberCount).toBe(5);
		expect(tree.root.directRecords.map((item) => item.userID)).toEqual(['ceo']);
		expect(tree.nodes.map((node) => [node.id, node.depth, node.memberCount])).toEqual([
			['product', 0, 3],
			['engineering', 1, 1],
			['design', 1, 1],
			['sales', 0, 1]
		]);
	});

	test('moves an organization to a new parent while preserving its subtree', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'platform', name: '플랫폼팀', parentID: 'engineering' },
			{ id: 'sales', name: '세일즈' }
		];

		const moved = moveOrgchartOrganization(groups, 'engineering', 2, 0);

		expect(moved).toEqual([
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'sales', name: '세일즈' },
			{ id: 'engineering', name: '개발팀', parentID: '' },
			{ id: 'platform', name: '플랫폼팀', parentID: 'engineering' }
		]);
	});

	test('clamps a requested depth to the previous organization depth plus one', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'sales', name: '세일즈' }
		];

		const moved = moveOrgchartOrganization(groups, 'sales', 1, 5);

		expect(moved).toEqual([
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'sales', name: '세일즈', parentID: 'product' }
		]);
	});

	test('describes the parent and depth represented by the preview line', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'sales', name: '세일즈' }
		];

		const preview = orgchartOrganizationMovePreview(groups, 'sales', 2, 3);

		expect(preview).toEqual({ insertionIndex: 2, depth: 2, parentID: 'engineering' });
	});

	test('snaps a top-level insertion behind the preceding organization subtree', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'sales', name: '세일즈' }
		];

		const preview = orgchartOrganizationMovePreview(groups, 'sales', 1, 0);
		const moved = moveOrgchartOrganization(groups, 'sales', 1, 0);

		expect(preview).toEqual({ insertionIndex: 2, depth: 0, parentID: '' });
		expect(moved.map((group) => group.id)).toEqual(['product', 'engineering', 'sales']);
	});

	test('indexes parent and child relationships for repeated tree lookups', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'platform', name: '플랫폼팀', parentID: 'engineering' },
			{ id: 'sales', name: '세일즈' }
		];
		const tree = orgchartOrganizationTree(groups, [], '');

		const index = orgchartOrganizationTreeIndex(tree.nodes);

		expect(index.nodeByID.get('platform')?.parentID).toBe('engineering');
		expect(index.groupIDsWithChildren).toEqual(new Set(['product', 'engineering']));
		expect(orgchartOrganizationSubtreeIDs(tree.nodes, 'engineering')).toEqual(new Set(['engineering', 'platform']));
	});
});
