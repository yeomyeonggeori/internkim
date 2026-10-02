import { expect, mock, test } from 'bun:test';
import type { CRMDataResponse } from '../../../src/routes/crm/crm-api-types';

const asked: string[] = [];
let denied = false;
mock.module('../../../src/lib/public-api-call', () => ({
	ToolRefused: class extends Error {},
	invokeTool: async (name: string) => {
		asked.push(name);
		if (denied) throw new Error('permission denied');
		if (name === 'crm_organization_list') return { organizations: [] };
		if (name === 'crm_contact_list') return { contacts: [] };
		if (name === 'crm_opportunity_list') return { opportunities: [] };
		if (name === 'crm_activity_list') return { activities: [], registeredLabels: { businesses: [], types: [] } };
		if (name === 'crm_vocabulary_get') return { organizationTypes: [], pipelines: [] };
		throw new Error(`unexpected ${name}`);
	}
}));

const { loadSupabaseCRMData } = await import('../../../src/routes/crm/crm-public-api');

function snapshot(): CRMDataResponse {
	return {
		organizations: [], contacts: [], activities: [], pipelines: [],
		vocabulary: { organization_types: [{ id: 'customer', name: 'Customer' }], pipelines: [] },
		taskVocabulary: { businesses: [{ name: 'Sales' }], types: [] },
		opportunities: [{
			id: 'deal', organizationID: 'organization', name: 'Example', pipeline: 'sales', stage: 'waiting',
			stagePosition: 1, stageChangedAt: '2026-10-01', ownerPersonID: 'member',
			amountMinor: 12345, currencyCode: 'USD', baseAmountMinor: 16450000, baseCurrencyCode: 'KRW', importance: 'medium',
			audit: { createdAt: '2026-10-01', createdByPersonID: 'member', updatedAt: '2026-10-01', updatedByPersonID: 'member' }
		}]
	};
}

test('a cold or explicit full refresh still reads all five complete sources', async () => {
	asked.length = 0;
	await loadSupabaseCRMData();
	expect(asked).toEqual(['crm_organization_list', 'crm_contact_list', 'crm_opportunity_list', 'crm_activity_list', 'crm_vocabulary_get']);
	asked.length = 0;
	await loadSupabaseCRMData(snapshot());
	expect(asked).toHaveLength(5);
});

test('contact refresh preserves loaded metrics, currency minor units and definitions without rereading unrelated sources', async () => {
	asked.length = 0;
	const before = snapshot();
	const after = await loadSupabaseCRMData(before, ['contacts']);
	expect(asked).toEqual(['crm_contact_list']);
	expect(after.opportunities).toBe(before.opportunities);
	expect(after.opportunities[0].amountMinor).toBe(12345);
	expect(after.opportunities[0].baseAmountMinor).toBe(16450000);
	expect(after.vocabulary).toBe(before.vocabulary);
	expect(after.taskVocabulary).toBe(before.taskVocabulary);
});

test('activity changes refresh both activities and opportunity counts', async () => {
	asked.length = 0;
	const before = snapshot();
	const after = await loadSupabaseCRMData(before, ['activities', 'opportunities']);
	expect(asked).toEqual(['crm_opportunity_list', 'crm_activity_list']);
	expect(after.organizations).toBe(before.organizations);
	expect(after.contacts).toBe(before.contacts);
});

test('a denied refresh leaves the prior complete snapshot untouched', async () => {
	const before = snapshot();
	denied = true;
	try {
		await expect(loadSupabaseCRMData(before, ['contacts'])).rejects.toThrow('permission denied');
		expect(before.opportunities[0].amountMinor).toBe(12345);
	} finally { denied = false; }
});
