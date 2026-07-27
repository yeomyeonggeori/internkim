import { describe, expect, test } from 'bun:test';
import type { OrgGroup, UserRecord } from '../../../src/lib/organization/types';
import { organizationOrganizationSections } from '../../../src/routes/organization/organization-model';

function record(userID: string, groupID = ''): UserRecord {
	return {
		userID,
		handle: userID,
		name: userID,
		email: `${userID}@example.com`,
		groupID
	};
}

describe('organization organization model', () => {
	test('renders the synthetic root and every organization in hierarchy and priority order', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'design', name: '디자인팀', parentID: 'product' },
			{ id: 'sales', name: '세일즈' }
		];
		const records = [record('ceo'), record('product-lead', 'product'), record('engineer', 'engineering'), record('designer', 'design'), record('sales', 'sales')];

		const sections = organizationOrganizationSections(records, groups, '전체 조직');

		expect(sections.map((section) => [section.id, section.depth, section.memberCount, section.records.map((item) => item.userID)])).toEqual([
			['', 0, 5, ['ceo']],
			['sales', 1, 1, ['sales']],
			['product', 1, 3, ['product-lead']],
			['engineering', 2, 1, ['engineer']],
			['design', 2, 1, ['designer']]
		]);
	});

	test('keeps executive organization priority when its member is filtered out', () => {
		const groups: OrgGroup[] = [
			{ id: 'development', name: '개발팀' },
			{ id: 'taskforce', name: '태스크포스' },
			{ id: 'otok', name: '오토케팀' },
			{ id: 'management', name: '경영팀' }
		];
		const allRecords = [
			{ ...record('cto', 'development'), jobTitle: 'CTO' },
			record('taskforce-member', 'taskforce'),
			record('otok-member', 'otok'),
			{ ...record('ceo', 'management'), jobTitle: 'CEO' }
		];
		const visibleRecords = allRecords.filter((item) => item.userID !== 'ceo');

		const sections = organizationOrganizationSections(visibleRecords, groups, '전체 조직', '', allRecords);

		expect(sections.map((section) => section.id)).toEqual(['', 'management', 'development', 'otok', 'taskforce']);
	});

	test('places people with unknown organizations in the root without creating a group', () => {
		const records = [record('known', 'product'), record('unknown', 'missing')];

		const sections = organizationOrganizationSections(records, [{ id: 'product', name: '제품' }], '전체 조직');

		expect(sections.map((section) => section.id)).toEqual(['', 'product']);
		expect(sections[0]?.records.map((item) => item.userID)).toEqual(['unknown']);
	});

	test('shows only the selected organization subtree with rebased depth', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'platform', name: '플랫폼팀', parentID: 'engineering' },
			{ id: 'sales', name: '세일즈' }
		];

		const sections = organizationOrganizationSections([], groups, '전체 조직', 'engineering');

		expect(sections.map((section) => [section.id, section.depth])).toEqual([
			['engineering', 0],
			['platform', 1]
		]);
	});

	test('keeps organization member counts based on all records while search filters visible people', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];
		const allRecords = [record('lead', 'product'), record('engineer', 'product')];

		const sections = organizationOrganizationSections([allRecords[1]], groups, '전체 조직', '', allRecords);

		expect(sections.map((section) => [section.id, section.memberCount, section.records.map((item) => item.userID)])).toEqual([
			['', 2, []],
			['product', 2, ['engineer']]
		]);
	});

	test('places the unique organization hierarchy root before earlier employees and marks it responsible', () => {
		const leader = { ...record('leader', 'product'), hireDate: '2026-02-01' };
		const employee = { ...record('employee', 'product'), hireDate: '2026-01-01', supervisorID: 'leader' };

		const sections = organizationOrganizationSections([leader, employee], [{ id: 'product', name: '제품팀' }], '전체 조직');
		const productSection = sections.find((section) => section.id === 'product');

		expect(productSection?.records.map((item) => item.userID)).toEqual(['leader', 'employee']);
		expect(productSection?.responsibleUserID).toBe('leader');
	});

	test('carries company responsibility into the organization containing the company leader', () => {
		const companyLeader = record('company-leader', 'leadership');
		const productLeader = { ...record('product-leader', 'product'), supervisorID: 'company-leader' };
		const employee = { ...record('employee', 'product'), supervisorID: 'product-leader' };
		const groups: OrgGroup[] = [
			{ id: 'leadership', name: '경영' },
			{ id: 'product', name: '제품팀' }
		];

		const sections = organizationOrganizationSections([companyLeader, productLeader, employee], groups, '전체 조직');
		const leadershipSection = sections.find((section) => section.id === 'leadership');
		const productSection = sections.find((section) => section.id === 'product');

		expect(leadershipSection?.companyResponsibleUserID).toBe('company-leader');
		expect(productSection?.companyResponsibleUserID).toBe('company-leader');
		expect(leadershipSection?.responsibleUserID).toBe('company-leader');
		expect(productSection?.responsibleUserID).toBe('product-leader');
	});

	test('keeps the company representative when someone reports to a person outside the directory', () => {
		const companyLeader = record('company-leader', 'leadership');
		const hiddenReport = { ...record('hidden-report', 'product'), supervisorID: 'resigned-leader' };
		const groups: OrgGroup[] = [
			{ id: 'leadership', name: '경영' },
			{ id: 'product', name: '제품팀' }
		];

		const sections = organizationOrganizationSections([companyLeader, hiddenReport], groups, '전체 조직');

		expect(sections[0]?.companyResponsibleUserID).toBe('company-leader');
	});

	test('does not name a company representative when nobody or several people lack a supervisor', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];
		const firstLeader = record('first-leader', 'product');
		const secondLeader = record('second-leader', 'product');

		const sections = organizationOrganizationSections([firstLeader, secondLeader], groups, '전체 조직');

		expect(sections[0]?.companyResponsibleUserID).toBe(undefined);
	});

	test('does not assign responsibility when an organization has multiple hierarchy roots', () => {
		const firstRoot = record('first-root', 'product');
		const secondRoot = record('second-root', 'product');

		const sections = organizationOrganizationSections([firstRoot, secondRoot], [{ id: 'product', name: '제품팀' }], '전체 조직');
		const productSection = sections.find((section) => section.id === 'product');

		expect(productSection?.responsibleUserID).toBe(undefined);
	});
});
