import { describe, expect, test } from 'bun:test';
import {
	matchesLastContactWindow,
	organizationMatchesFacets,
	type CRMRelationshipFacets
} from '../../../src/routes/crm/crm-view-model';
import type { CRMOrganization } from '../../../src/routes/crm/crm-types';

const today = '2026-08-03';

const clearedFacets: CRMRelationshipFacets = {
	status: 'all',
	type: 'all',
	importance: 'all',
	lastContact: 'all'
};

function organization(lastContactDate: string, overrides: Partial<CRMOrganization> = {}): CRMOrganization {
	return {
		id: lastContactDate,
		name: '샘플 회사',
		types: ['customer'],
		status: 'active',
		importance: 'medium',
		ownerName: '이샘플',
		ownerEmail: 'owner@example.com',
		team: '',
		tags: [],
		description: '',
		lastContactDate,
		nextActionDate: '',
		openOpportunityCount: 0,
		expectedValues: {},
		...overrides
	};
}

describe('last-contact windows', () => {
	test('counts the boundary day as inside the window it names', () => {
		expect(matchesLastContactWindow('2026-07-27', 'within_7', today)).toBe(true);
		expect(matchesLastContactWindow('2026-07-26', 'within_7', today)).toBe(false);
		expect(matchesLastContactWindow('2026-07-04', 'within_30', today)).toBe(true);
		expect(matchesLastContactWindow('2026-07-03', 'within_30', today)).toBe(false);
		expect(matchesLastContactWindow('2026-05-05', 'within_90', today)).toBe(true);
		expect(matchesLastContactWindow('2026-05-04', 'within_90', today)).toBe(false);
	});

	test('reads 90일 넘음 as everything the 90-day window leaves out', () => {
		expect(matchesLastContactWindow('2026-05-04', 'over_90', today)).toBe(true);
		expect(matchesLastContactWindow('2026-05-05', 'over_90', today)).toBe(false);
	});

	test('lets every date through while the window is cleared', () => {
		expect(matchesLastContactWindow('2020-01-01', 'all', today)).toBe(true);
	});
});

describe('relationship facets', () => {
	test('keeps every organization while all four facets are cleared', () => {
		expect(organizationMatchesFacets(organization('2020-01-01'), clearedFacets, today)).toBe(true);
	});

	test('narrows by status, type, importance and last contact together', () => {
		const paused = organization('2026-08-01', { status: 'paused', types: ['partner'], importance: 'high' });

		expect(organizationMatchesFacets(paused, { ...clearedFacets, status: 'paused' }, today)).toBe(true);
		expect(organizationMatchesFacets(paused, { ...clearedFacets, status: 'active' }, today)).toBe(false);
		expect(organizationMatchesFacets(paused, { ...clearedFacets, type: 'partner' }, today)).toBe(true);
		expect(organizationMatchesFacets(paused, { ...clearedFacets, type: 'customer' }, today)).toBe(false);
		expect(organizationMatchesFacets(paused, { ...clearedFacets, importance: 'high' }, today)).toBe(true);
		expect(organizationMatchesFacets(paused, { ...clearedFacets, importance: 'low' }, today)).toBe(false);
		expect(organizationMatchesFacets(paused, { ...clearedFacets, lastContact: 'within_7' }, today)).toBe(true);
		expect(organizationMatchesFacets(paused, { ...clearedFacets, lastContact: 'over_90' }, today)).toBe(false);
	});
});
