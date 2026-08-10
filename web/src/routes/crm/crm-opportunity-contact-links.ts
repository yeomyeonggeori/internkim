import type { CRMContact, CRMOpportunityContact } from './crm-types';

export type CRMOpportunityContactSelection = {
	contactIDs: string[];
	primaryContactID: string;
};

export function opportunityContactSelection(
	contacts: CRMOpportunityContact[] | undefined
): CRMOpportunityContactSelection {
	return {
		contactIDs: contacts?.map((contact) => contact.contactID) ?? [],
		primaryContactID: contacts?.find((contact) => contact.isPrimary)?.contactID ?? ''
	};
}

export function opportunityContactSelectionForAccount(
	selection: CRMOpportunityContactSelection,
	contacts: CRMContact[],
	accountID: string
): CRMOpportunityContactSelection {
	const allowedContactIDs = new Set(
		contacts.filter((contact) => contact.accountID === accountID).map((contact) => contact.id)
	);
	const contactIDs = selection.contactIDs.filter((contactID) => allowedContactIDs.has(contactID));
	return {
		contactIDs,
		primaryContactID: contactIDs.includes(selection.primaryContactID) ? selection.primaryContactID : ''
	};
}

export function toggleOpportunityContact(
	selection: CRMOpportunityContactSelection,
	contactID: string,
	checked: boolean
): CRMOpportunityContactSelection {
	const contactIDs = checked
		? [...new Set([...selection.contactIDs, contactID])]
		: selection.contactIDs.filter((candidate) => candidate !== contactID);
	return {
		contactIDs,
		primaryContactID: contactIDs.includes(selection.primaryContactID) ? selection.primaryContactID : ''
	};
}

export function opportunityContactLinks(
	selection: CRMOpportunityContactSelection
): CRMOpportunityContact[] {
	return selection.contactIDs.map((contactID) => ({
		contactID,
		isPrimary: contactID === selection.primaryContactID
	}));
}
