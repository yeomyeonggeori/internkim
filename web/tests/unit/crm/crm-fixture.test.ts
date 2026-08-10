import { describe, expect, test } from 'bun:test';
import {
	crmAccounts,
	crmActivities,
	crmNextActions,
	crmOpportunities
} from '../../../src/routes/crm/dev-crm-fixture';
import { sumOpportunityMoney } from '../../../src/routes/crm/crm-money';
import type {
	CRMCurrency,
	CRMOpportunityStage,
	CRMProgressKind
} from '../../../src/routes/crm/crm-types';

const openStages = new Set<CRMOpportunityStage>([
	'lead',
	'qualified',
	'proposal',
	'negotiation',
	'on_hold'
]);

describe('CRM fixture integrity', () => {
	test('contains thirty opportunities across every supported classification', () => {
		const currencies = new Set<CRMCurrency>(crmOpportunities.map((opportunity) => opportunity.currency));
		const stages = new Set<CRMOpportunityStage>(crmOpportunities.map((opportunity) => opportunity.stage));
		const kinds = new Set<CRMProgressKind | undefined>(
			crmOpportunities.map((opportunity) => opportunity.kind)
		);

		expect(crmOpportunities.length).toBe(30);
		expect(currencies).toEqual(new Set<CRMCurrency>(['KRW', 'USD', 'JPY', 'EUR']));
		expect(stages).toEqual(
			new Set<CRMOpportunityStage>([
				'lead',
				'qualified',
				'proposal',
				'negotiation',
				'won',
				'lost',
				'on_hold'
			])
		);
		expect(kinds).toEqual(
			new Set<CRMProgressKind>([
				'sales',
				'investment',
				'sponsorship',
				'partnership',
				'procurement'
			])
		);
	});

	test('references existing accounts and next actions', () => {
		const accountsByID = new Map(crmAccounts.map((account) => [account.id, account]));
		const opportunityIDs = new Set(crmOpportunities.map((opportunity) => opportunity.id));
		const nextActionsByID = new Map(crmNextActions.map((action) => [action.id, action]));

		for (const opportunity of crmOpportunities) {
			const account = accountsByID.get(opportunity.accountID);
			expect(account !== undefined).toBe(true);
			expect(opportunity.ownerName).toBe(account?.ownerName);
			expect(opportunity.business).not.toBe('');

			if (opportunity.nextActionID) {
				const nextAction = nextActionsByID.get(opportunity.nextActionID);
				expect(nextAction?.accountID).toBe(opportunity.accountID);
				expect(nextAction?.opportunityID).toBe(opportunity.id);
			}
		}

		for (const activity of crmActivities) {
			expect(accountsByID.has(activity.accountID)).toBe(true);
			expect(activity.business).not.toBe('');
			if (activity.opportunityID) {
				const opportunity = crmOpportunities.find((candidate) => candidate.id === activity.opportunityID);
				expect(opportunity?.accountID).toBe(activity.accountID);
				expect(opportunity?.business).toBe(activity.business);
			}
		}

		for (const nextAction of crmNextActions) {
			expect(accountsByID.has(nextAction.accountID)).toBe(true);
			if (nextAction.opportunityID) {
				expect(opportunityIDs.has(nextAction.opportunityID)).toBe(true);
			}
		}
	});

	test('keeps account opportunity counts and currency totals synchronized', () => {
		for (const account of crmAccounts) {
			const opportunities = crmOpportunities.filter(
				(opportunity) => opportunity.accountID === account.id
			);
			const openOpportunityCount = opportunities.filter((opportunity) =>
				openStages.has(opportunity.stage)
			).length;

			expect(account.openOpportunityCount).toBe(openOpportunityCount);
			expect(account.expectedValues).toEqual(sumOpportunityMoney(opportunities));
		}
	});
});
