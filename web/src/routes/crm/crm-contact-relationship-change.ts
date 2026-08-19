import type { CRMOpportunity } from './crm-types';

export function incompatibleCRMOpportunityNames(
	contactID: string,
	nextOrganizationID: string,
	opportunities: CRMOpportunity[]
): string[] {
	return opportunities
		.filter((opportunity) => opportunity.organizationID !== ''
			&& opportunity.organizationID !== nextOrganizationID
			&& (opportunity.contacts ?? []).some((contact) => contact.contactID === contactID))
		.map((opportunity) => opportunity.name);
}
