import { describe, expect, test } from 'bun:test';
import {
	crmOrganizations,
	crmActivities,
	crmOpportunities
} from '../../../src/routes/crm/dev-crm-fixture';
import { nextActionsOf, withNextAction } from '../../../src/routes/crm/crm-mappers';
import { sumOpportunityMoney } from '../../../src/routes/crm/crm-money';
import type {
	CRMCurrency,
	CRMOpportunityStage,
	CRMProgressKind
} from '../../../src/routes/crm/crm-types';

const openStages = new Set<CRMOpportunityStage>([
	'waiting',
	'in_progress',
	'review',
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
				'waiting',
				'in_progress',
				'review',
				'done',
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

	test('references existing organizations and next actions', () => {
		const nextActions = nextActionsOf(crmActivities, 'Asia/Seoul');
		const organizationsByID = new Map(crmOrganizations.map((organization) => [organization.id, organization]));
		const opportunityIDs = new Set(crmOpportunities.map((opportunity) => opportunity.id));
		const nextActionsByID = new Map(nextActions.map((action) => [action.id, action]));

		for (const stored of crmOpportunities) {
			const opportunity = withNextAction(stored, nextActions);
			const organization = organizationsByID.get(opportunity.organizationID);
			if (organization === undefined) {
				throw new Error(`opportunity ${opportunity.id} names organization ${opportunity.organizationID}, which no fixture declares`);
			}
			expect(opportunity.ownerName).toBe(organization.ownerName);
			expect(opportunity.business).not.toBe('');

			if (opportunity.nextActionID) {
				const nextAction = nextActionsByID.get(opportunity.nextActionID);
				expect(nextAction?.organizationID).toBe(opportunity.organizationID);
				expect(nextAction?.opportunityID).toBe(opportunity.id);
			}
		}

		for (const activity of crmActivities) {
			expect(organizationsByID.has(activity.organizationID)).toBe(true);
			expect(activity.business).not.toBe('');
			if (activity.opportunityID) {
				const opportunity = crmOpportunities.find((candidate) => candidate.id === activity.opportunityID);
				expect(opportunity?.organizationID).toBe(activity.organizationID);
				expect(opportunity?.business).toBe(activity.business);
			}
		}

		for (const nextAction of nextActions) {
			expect(organizationsByID.has(nextAction.organizationID)).toBe(true);
			if (nextAction.opportunityID) {
				expect(opportunityIDs.has(nextAction.opportunityID)).toBe(true);
			}
		}
	});

	test('keeps organization opportunity counts and currency totals synchronized', () => {
		for (const organization of crmOrganizations) {
			const opportunities = crmOpportunities.filter(
				(opportunity) => opportunity.organizationID === organization.id
			);
			const openOpportunityCount = opportunities.filter((opportunity) =>
				openStages.has(opportunity.stage)
			).length;

			expect(organization.openOpportunityCount).toBe(openOpportunityCount);
			expect(organization.expectedValues).toEqual(sumOpportunityMoney(opportunities));
		}
	});
});
