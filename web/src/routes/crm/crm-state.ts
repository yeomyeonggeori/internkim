import type { CRMOrganization, CRMOpportunity } from './crm-types';
import { sumOpportunityMoney } from './crm-money';

const closedStages = new Set(['won', 'lost']);

export function synchronizeOrganizationPipeline(organizations: CRMOrganization[], opportunities: CRMOpportunity[]): CRMOrganization[] {
	return organizations.map((organization) => {
		const openOpportunities = opportunities.filter(
			(opportunity) => opportunity.organizationID === organization.id && !closedStages.has(opportunity.stage)
		);
		return {
			...organization,
			openOpportunityCount: openOpportunities.length,
			expectedValues: sumOpportunityMoney(openOpportunities)
		};
	});
}
