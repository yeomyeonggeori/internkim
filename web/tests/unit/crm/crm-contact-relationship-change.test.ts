import { describe, expect, test } from 'bun:test';
import { incompatibleCRMOpportunityNames } from '../../../src/routes/crm/crm-contact-relationship-change';
import type { CRMOpportunity } from '../../../src/routes/crm/crm-types';

describe('CRM contact relationship changes', () => {
	test('lists organization-backed opportunities that would no longer match', () => {
		const opportunities = [
			opportunity('organization-a', '연결 진행 건', 'contact-a'),
			opportunity('', '개인 고객 진행 건', 'contact-a'),
			opportunity('organization-b', '다른 담당자 진행 건', 'contact-b')
		];

		expect(incompatibleCRMOpportunityNames('contact-a', 'organization-b', opportunities)).toEqual(['연결 진행 건']);
	});

	test('allows moving a contact when every organization-backed opportunity matches', () => {
		expect(incompatibleCRMOpportunityNames('contact-a', 'organization-a', [
			opportunity('organization-a', '연결 진행 건', 'contact-a')
		])).toEqual([]);
	});
});

function opportunity(organizationID: string, name: string, contactID: string): CRMOpportunity {
	return {
		id: name,
		organizationID,
		business: 'general',
		name,
		pipeline: 'sales',
		stage: 'lead',
		ownerName: '담당자',
		currency: 'KRW',
		importance: 'medium',
		targetDate: '',
		staleDays: 0,
		contacts: [{ contactID }]
	};
}
