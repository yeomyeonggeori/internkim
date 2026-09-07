import { describe, expect, test } from 'bun:test';
import type { CRMContact, CRMOrganization } from '../../../src/routes/crm/crm-types';

Object.assign(globalThis, {
	$state<Value>(value: Value): Value {
		return value;
	}
});

const { crmSearch } = await import('../../../src/lib/components/crm-search.svelte');

function organization(id: string, name: string, tags: string[] = []): CRMOrganization {
	return {
		id,
		name,
		types: ['customer'],
		status: 'active',
		importance: 'medium',
		ownerName: '이샘플',
		ownerEmail: 'owner@example.com',
		team: '',
		tags,
		description: '',
		lastContactDate: '2026-08-01',
		nextActionDate: '',
		openOpportunityCount: 0,
		expectedValues: {}
	};
}

function contact(id: string, name: string, email: string, title: string): CRMContact {
	return { id, organizationID: 'organization-one', name, title, email };
}

crmSearch.organizations = [
	organization('organization-one', '샘플 임팩트 랩', ['우선']),
	organization('organization-two', '예시 벤처스'),
	organization('organization-three', 'Sample Cloud', ['partner'])
];
crmSearch.contacts = [
	contact('contact-one', '박예시', 'yesi@example.com', '운영 리드'),
	contact('contact-two', '최견본', 'gyeonbon@example.com', '파트너십 디렉터')
];

describe('CRM command palette search', () => {
	test('finds an organization by name, by a half-typed syllable, and by tag', () => {
		expect(crmSearch.search('임팩트').organizations.map((match) => match.id)).toEqual(['organization-one']);
		expect(crmSearch.search('임ㅍ').organizations.map((match) => match.id)).toEqual(['organization-one']);
		expect(crmSearch.search('우선').organizations.map((match) => match.id)).toEqual(['organization-one']);
	});

	test('matches a contact by name, email and title', () => {
		expect(crmSearch.search('박예시').contacts.map((match) => match.id)).toEqual(['contact-one']);
		expect(crmSearch.search('gyeonbon').contacts.map((match) => match.id)).toEqual(['contact-two']);
		expect(crmSearch.search('디렉터').contacts.map((match) => match.id)).toEqual(['contact-two']);
	});

	test('returns nothing at all for a blank query', () => {
		expect(crmSearch.search('   ')).toEqual({ organizations: [], contacts: [] });
	});

	test('names the organization a contact belongs to', () => {
		expect(crmSearch.organizationNameOf('organization-one')).toBe('샘플 임팩트 랩');
		expect(crmSearch.organizationNameOf('missing')).toBe('');
	});
});
