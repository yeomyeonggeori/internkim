import { describe, expect, test } from 'bun:test';
import {
	opportunityContactLinks,
	opportunityContactSelection,
	opportunityContactSelectionForAccount,
	toggleOpportunityContact
} from '../../../src/routes/crm/crm-opportunity-contact-links';
import type { CRMContact } from '../../../src/routes/crm/crm-types';

const contacts = [
	{ id: 'contact-a', accountID: 'account-a' },
	{ id: 'contact-b', accountID: 'account-b' }
] as CRMContact[];

describe('CRM opportunity contact links', () => {
	test('removes links that do not belong to a newly selected account', () => {
		const selection = opportunityContactSelection([{ contactID: 'contact-a', isPrimary: true }]);
		expect(opportunityContactSelectionForAccount(selection, contacts, 'account-b')).toEqual({
			contactIDs: [],
			primaryContactID: ''
		});
	});

	test('keeps selected contacts and one explicit primary contact', () => {
		let selection = opportunityContactSelection(undefined);
		selection = toggleOpportunityContact(selection, 'contact-a', true);
		selection = toggleOpportunityContact(selection, 'contact-b', true);
		selection = { ...selection, primaryContactID: 'contact-b' };
		expect(opportunityContactLinks(selection)).toEqual([
			{ contactID: 'contact-a', isPrimary: false },
			{ contactID: 'contact-b', isPrimary: true }
		]);
	});
});
