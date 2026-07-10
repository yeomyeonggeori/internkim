import { describe, expect, test } from 'bun:test';
import type { OrgGroup, UserRecord } from '../../../src/routes/admin/admin-types';
import { orgchartOrganizationSections, type OrgchartOrganizationMemberNode } from '../../../src/routes/orgchart/orgchart-organization-model';

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

function nodeIDs(node: OrgchartOrganizationMemberNode): unknown {
	return {
		id: node.record.userID,
		children: node.children.map(nodeIDs)
	};
}

describe('orgchart organization model', () => {
	test('stacks one organization leader over direct and nested members', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];
		const records = [
			userRecord({ userID: 'lead', name: 'Lead', hireDate: '2026-01-01', primaryGroupID: 'product', groupIDs: ['product'] }),
			userRecord({ userID: 'designer', name: 'Designer', hireDate: '2026-02-01', primaryGroupID: 'product', groupIDs: ['product'], supervisorID: 'lead' }),
			userRecord({ userID: 'engineer', name: 'Engineer', hireDate: '2026-02-02', primaryGroupID: 'product', groupIDs: ['product'], supervisorID: 'lead' }),
			userRecord({ userID: 'intern', name: 'Intern', hireDate: '2026-03-01', primaryGroupID: 'product', groupIDs: ['product'], supervisorID: 'designer' })
		];

		const [section] = orgchartOrganizationSections(records, groups, '팀 미지정');

		expect(section?.leader.userID).toBe('lead');
		expect(section?.memberCount).toBe(4);
		expect(section?.memberNodes.map(nodeIDs)).toEqual([
			{ id: 'designer', children: [{ id: 'intern', children: [] }] },
			{ id: 'engineer', children: [] }
		]);
	});

	test('keeps extra roots under the first organization leader', () => {
		const groups: OrgGroup[] = [{ id: 'shared', name: '공유팀' }];
		const records = [
			userRecord({ userID: 'first', name: '김첫째', hireDate: '2026-01-01', primaryGroupID: 'shared', groupIDs: ['shared'] }),
			userRecord({ userID: 'second', name: '이둘째', hireDate: '2026-01-01', primaryGroupID: 'shared', groupIDs: ['shared'] }),
			userRecord({ userID: 'third', name: '박셋째', hireDate: '2026-02-01', primaryGroupID: 'shared', groupIDs: ['shared'] })
		];

		const [section] = orgchartOrganizationSections(records, groups, '팀 미지정');

		expect(section?.leader.userID).toBe('first');
		expect(section?.memberNodes.map((node) => node.record.userID)).toEqual(['second', 'third']);
	});

	test('includes unknown and unassigned organizations after known groups', () => {
		const groups: OrgGroup[] = [{ id: 'known', name: '기존 조직' }];
		const records = [
			userRecord({ userID: 'known-user', primaryGroupID: 'known', groupIDs: ['known'] }),
			userRecord({ userID: 'unknown-user', primaryGroupID: 'unknown', groupIDs: ['unknown'] }),
			userRecord({ userID: 'unassigned-user' })
		];

		const sections = orgchartOrganizationSections(records, groups, '팀 미지정');

		expect(sections.map((section) => [section.id, section.name, section.isUnassigned])).toEqual([
			['known', '기존 조직', false],
			['unknown', 'unknown', false],
			['__unassigned__', '팀 미지정', true]
		]);
	});
});
