import { describe, expect, test } from 'bun:test';
import {
	activityReferenceAfterAccountChange,
	activityReferenceAfterOpportunityChange,
	type CRMActivityOpportunityReference
} from '../../../src/routes/crm/crm-activity-reference';

const opportunities: CRMActivityOpportunityReference[] = [
	{ id: 'opportunity-a', accountID: 'account-a', business: 'business-a' },
	{ id: 'opportunity-b', accountID: 'account-b', business: 'business-b' }
];

describe('CRM activity references', () => {
	test('clears an opportunity that does not belong to the selected account', () => {
		expect(activityReferenceAfterAccountChange(opportunities, 'account-b', 'opportunity-a', 'general')).toEqual({
			accountID: 'account-b',
			opportunityID: '',
			business: 'general'
		});
	});

	test('derives account and business from the selected opportunity', () => {
		expect(activityReferenceAfterOpportunityChange(opportunities, 'opportunity-b', 'account-a', 'general')).toEqual({
			accountID: 'account-b',
			opportunityID: 'opportunity-b',
			business: 'business-b'
		});
	});
});
