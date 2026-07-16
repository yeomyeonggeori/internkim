import { describe, expect, test } from 'bun:test';
import type { OrgGroup, UserRecord } from '../../../src/lib/orgchart/types';
import { orgchartOrganizationSections, type OrgchartOrganizationMemberNode } from '../../../src/routes/orgchart/orgchart-organization-model';

function userRecord(overrides: Partial<UserRecord>): UserRecord {
	return {
		userID: '',
		handle: '',
		name: '',
		email: '',
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

	test('orders organizations by normalized executive job titles across every membership', () => {
		const groups: OrgGroup[] = [
			{ id: 'normal-zulu', name: 'Zulu' },
			{ id: 'c-level', name: 'Zulu C-level' },
			{ id: 'co-founder', name: 'Alpha Co-Founder' },
			{ id: 'co-ceo', name: 'Zulu Co-CEO' },
			{ id: 'founder', name: 'Alpha Founder' },
			{ id: 'ceo-primary', name: 'Zulu CEO' },
			{ id: 'ceo-secondary', name: 'Secondary CEO' },
			{ id: 'normal-echo', name: 'Echo' }
		];
		const records = [
			userRecord({ userID: 'ceo', jobTitle: ' C-E o ', primaryGroupID: 'ceo-primary', groupIDs: ['ceo-primary', 'ceo-secondary'] }),
			userRecord({ userID: 'ceo-secondary-member', primaryGroupID: 'ceo-secondary', groupIDs: ['ceo-secondary'] }),
			userRecord({ userID: 'founder', jobTitle: ' FOUN-DER ', primaryGroupID: 'founder', groupIDs: ['founder'] }),
			userRecord({ userID: 'co-ceo', jobTitle: ' co - C E O ', primaryGroupID: 'co-ceo', groupIDs: ['co-ceo'] }),
			userRecord({ userID: 'co-founder', jobTitle: ' Co- FOUNDer ', primaryGroupID: 'co-founder', groupIDs: ['co-founder'] }),
			userRecord({ userID: 'c-level', jobTitle: ' C-T o ', primaryGroupID: 'c-level', groupIDs: ['c-level'] }),
			userRecord({ userID: 'normal-zulu', primaryGroupID: 'normal-zulu', groupIDs: ['normal-zulu'] }),
			userRecord({ userID: 'normal-echo', jobTitle: 'Engineering Lead', primaryGroupID: 'normal-echo', groupIDs: ['normal-echo'] }),
			userRecord({ userID: 'unassigned' })
		];

		const sections = orgchartOrganizationSections(records, groups, '팀 미지정', records, 'en');

		expect(sections.map((section) => section.id)).toEqual([
			'founder',
			'ceo-secondary',
			'ceo-primary',
			'co-founder',
			'co-ceo',
			'c-level',
			'normal-echo',
			'normal-zulu',
			'__unassigned__'
		]);
	});

	test('treats another C-level abbreviation as executive priority', () => {
		const groups: OrgGroup[] = [
			{ id: 'general', name: 'Alpha' },
			{ id: 'c-level', name: 'Zulu' }
		];
		const records = [
			userRecord({ userID: 'general', primaryGroupID: 'general', groupIDs: ['general'] }),
			userRecord({ userID: 'c-level', jobTitle: 'CFO', primaryGroupID: 'c-level', groupIDs: ['c-level'] })
		];

		const sections = orgchartOrganizationSections(records, groups, 'Unassigned');

		expect(sections.map((section) => section.id)).toEqual(['c-level', 'general']);
	});

	test('keeps general C and O phrases below short C-suite acronyms', () => {
		const groups: OrgGroup[] = [
			{ id: 'general', name: 'Alpha' },
			{ id: 'c-level', name: 'Zulu' }
		];
		const records = [
			userRecord({ userID: 'general', jobTitle: 'Content SEO', primaryGroupID: 'general', groupIDs: ['general'] }),
			userRecord({ userID: 'c-level', jobTitle: 'CHRO', primaryGroupID: 'c-level', groupIDs: ['c-level'] })
		];

		const sections = orgchartOrganizationSections(records, groups, 'Unassigned', records, 'en');

		expect(sections.map((section) => section.id)).toEqual(['c-level', 'general']);
	});

	test('treats non-executive job titles as general and orders them by localized name', () => {
		const groups: OrgGroup[] = [
			{ id: 'zero-zulu', name: 'Zulu' },
			{ id: 'general-alpha', name: 'Alpha' }
		];
		const records = [
			userRecord({ userID: 'zero', jobTitle: 'Vice President', primaryGroupID: 'zero-zulu', groupIDs: ['zero-zulu'] }),
			userRecord({ userID: 'general', primaryGroupID: 'general-alpha', groupIDs: ['general-alpha'] })
		];

		const sections = orgchartOrganizationSections(records, groups, 'Unassigned', records, 'en');

		expect(sections.map((section) => section.id)).toEqual(['general-alpha', 'zero-zulu']);
	});

	test('keeps organization priority from all records when visible records are filtered', () => {
		const groups: OrgGroup[] = [
			{ id: 'leadership', name: 'Zulu' },
			{ id: 'product', name: 'Alpha' }
		];
		const allRecords = [
			userRecord({ userID: 'ceo', jobTitle: 'CEO', primaryGroupID: 'leadership', groupIDs: ['leadership'] }),
			userRecord({ userID: 'leadership-member', primaryGroupID: 'leadership', groupIDs: ['leadership'] }),
			userRecord({ userID: 'product-member', primaryGroupID: 'product', groupIDs: ['product'] })
		];
		const visibleRecords = allRecords.filter((record) => record.userID !== 'ceo');

		const sections = orgchartOrganizationSections(visibleRecords, groups, 'Unassigned', allRecords, 'en');

		expect(sections.map((section) => section.id)).toEqual(['leadership', 'product']);
	});

	test('uses the active locale when organizations share the same priority', () => {
		const groups: OrgGroup[] = [
			{ id: 'korean', name: '가' },
			{ id: 'english', name: 'A' }
		];
		const records = [
			userRecord({ userID: 'korean', primaryGroupID: 'korean', groupIDs: ['korean'] }),
			userRecord({ userID: 'english', primaryGroupID: 'english', groupIDs: ['english'] })
		];

		const koreanSections = orgchartOrganizationSections(records, groups, 'Unassigned', records, 'ko');
		const englishSections = orgchartOrganizationSections(records, groups, 'Unassigned', records, 'en');

		expect(koreanSections.map((section) => section.id)).toEqual(['korean', 'english']);
		expect(englishSections.map((section) => section.id)).toEqual(['english', 'korean']);
	});
});
