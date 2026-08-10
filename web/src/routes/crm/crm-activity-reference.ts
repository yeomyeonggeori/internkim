import type { CRMOpportunity } from './crm-types';

export type CRMActivityReferenceState = {
	accountID: string;
	opportunityID: string;
	business: string;
};

export type CRMActivityOpportunityReference = Pick<CRMOpportunity, 'id' | 'accountID' | 'business'>;

export function activityReferenceAfterAccountChange(
	opportunities: CRMActivityOpportunityReference[],
	accountID: string,
	opportunityID: string,
	business: string
): CRMActivityReferenceState {
	const opportunity = opportunities.find((candidate) => candidate.id === opportunityID);
	if (!opportunity || opportunity.accountID !== accountID) return { accountID, opportunityID: '', business };
	return { accountID, opportunityID: opportunity.id, business: opportunity.business };
}

export function activityReferenceAfterOpportunityChange(
	opportunities: CRMActivityOpportunityReference[],
	opportunityID: string,
	accountID: string,
	business: string
): CRMActivityReferenceState {
	const opportunity = opportunities.find((candidate) => candidate.id === opportunityID);
	if (!opportunity) return { accountID, opportunityID: '', business };
	return {
		accountID: opportunity.accountID,
		opportunityID: opportunity.id,
		business: opportunity.business
	};
}
