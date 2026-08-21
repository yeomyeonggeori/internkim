import type { CRMOpportunity } from './crm-types';

export type CRMActivityReferenceState = {
	organizationID: string;
	opportunityID: string;
	business: string;
};

export type CRMActivityOpportunityReference = Pick<CRMOpportunity, 'id' | 'organizationID' | 'business'>;

export function activityReferenceAfterOrganizationChange(
	opportunities: CRMActivityOpportunityReference[],
	organizationID: string,
	opportunityID: string,
	business: string
): CRMActivityReferenceState {
	const opportunity = opportunities.find((candidate) => candidate.id === opportunityID);
	if (!opportunity || opportunity.organizationID !== organizationID) return { organizationID, opportunityID: '', business };
	return { organizationID, opportunityID: opportunity.id, business: opportunity.business };
}

export function activityReferenceAfterOpportunityChange(
	opportunities: CRMActivityOpportunityReference[],
	opportunityID: string,
	organizationID: string,
	business: string
): CRMActivityReferenceState {
	const opportunity = opportunities.find((candidate) => candidate.id === opportunityID);
	if (!opportunity) return { organizationID, opportunityID: '', business };
	return {
		organizationID: opportunity.organizationID,
		opportunityID: opportunity.id,
		business: opportunity.business
	};
}
