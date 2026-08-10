import type { CRMAccount, CRMOpportunity } from './crm-types';
import { sumOpportunityMoney } from './crm-money';

const closedStages = new Set(['won', 'lost']);

export function synchronizeAccountPipeline(accounts: CRMAccount[], opportunities: CRMOpportunity[]): CRMAccount[] {
	return accounts.map((account) => {
		const openOpportunities = opportunities.filter(
			(opportunity) => opportunity.accountID === account.id && !closedStages.has(opportunity.stage)
		);
		return {
			...account,
			openOpportunityCount: openOpportunities.length,
			expectedValues: sumOpportunityMoney(openOpportunities)
		};
	});
}
