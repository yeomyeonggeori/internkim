import { describe, expect, test } from 'bun:test';
import {
	activityReferenceAfterOrganizationChange,
	activityReferenceAfterOpportunityChange,
	type CRMActivityOpportunityReference
} from '../../../src/routes/crm/crm-activity-reference';

const opportunities: CRMActivityOpportunityReference[] = [
	{ id: 'opportunity-a', organizationID: 'organization-a', business: 'business-a' },
	{ id: 'opportunity-b', organizationID: 'organization-b', business: 'business-b' }
];

describe('CRM activity references', () => {
	test('clears an opportunity that does not belong to the selected organization', () => {
		expect(activityReferenceAfterOrganizationChange(opportunities, 'organization-b', 'opportunity-a', 'general')).toEqual({
			organizationID: 'organization-b',
			opportunityID: '',
			business: 'general'
		});
	});

	test('derives organization and business from the selected opportunity', () => {
		expect(activityReferenceAfterOpportunityChange(opportunities, 'opportunity-b', 'organization-a', 'general')).toEqual({
			organizationID: 'organization-b',
			opportunityID: 'opportunity-b',
			business: 'business-b'
		});
	});
});
