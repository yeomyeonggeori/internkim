import type { CRMOrganization, CRMCurrency, CRMNextAction, CRMOpportunity, CRMPipelineStage } from './crm-types';
import { currentCRMDate, shiftCRMDate } from './crm-date';
import { crmInterimCurrencyCatalogue, formatMoney, formatMoneyTotals, sumOpportunityMoney } from './crm-money';
import { findOrganizationByID, getActionUrgency, getProgressKind } from './crm-view-model';
import type { CRMText } from './text';

export type CRMKPIMoneyDetail = {
	currency: CRMCurrency;
	displayValue: string;
};

export type CRMKPISegment = {
	label: string;
	value: number;
	displayValue: string;
	color: string;
	moneyDetails?: CRMKPIMoneyDetail[];
};

export type CRMKPICardData = {
	id: string;
	title: string;
	description: string;
	totalValue: string;
	totalLabel: string;
	totalMoneyDetails?: CRMKPIMoneyDetail[];
	segments: CRMKPISegment[];
};

const movingColor = '#0f9f8f';
const neutralColor = '#2495c9';
const attentionColor = '#ef6351';
const recentContactWindowDays = 30;

function countSegment(label: string, value: number, color: string, suffix: string): CRMKPISegment {
	return { label, value, displayValue: `${value}${suffix}`, color };
}

function buildMoneySummary(
	opportunities: CRMOpportunity[],
	text: CRMText
): { displayValue: string; moneyDetails?: CRMKPIMoneyDetail[] } {
	const totals = sumOpportunityMoney(opportunities);
	const moneyDetails = crmInterimCurrencyCatalogue.reduce<CRMKPIMoneyDetail[]>((details, entry) => {
		const amount = totals[entry.code];
		if (amount === undefined) return details;
		return [
			...details,
			{ currency: entry.code, displayValue: formatMoney(amount, entry.code, crmInterimCurrencyCatalogue, text.noValue) }
		];
	}, []);
	if (moneyDetails.length <= 1) return { displayValue: formatMoneyTotals(totals, crmInterimCurrencyCatalogue, text.noValue) };
	return {
		displayValue: text.currencyCount.replace('{count}', String(moneyDetails.length)),
		moneyDetails
	};
}

function amountSegment(label: string, opportunities: CRMOpportunity[], color: string, text: CRMText): CRMKPISegment {
	const moneySummary = buildMoneySummary(opportunities, text);
	return {
		label,
		value: opportunities.length,
		displayValue: moneySummary.moneyDetails
			? `${opportunities.length}${text.kpiCountSuffix}`
			: moneySummary.displayValue,
		color,
		moneyDetails: moneySummary.moneyDetails
	};
}

function opportunityOutcome(stages: CRMPipelineStage[], opportunity: CRMOpportunity): CRMPipelineStage['outcome'] | undefined {
	return stages.find(
		(stage) => stage.pipeline === (opportunity.pipeline ?? opportunity.kind) && stage.stage === opportunity.stage
	)?.outcome;
}

function buildPipelineHealth(opportunities: CRMOpportunity[], stages: CRMPipelineStage[], text: CRMText): CRMKPICardData {
	const openOpportunities = opportunities.filter((opportunity) => opportunityOutcome(stages, opportunity) === 'open');
	const onHold = opportunities.filter((opportunity) => opportunityOutcome(stages, opportunity) === 'on_hold');
	const stalled = openOpportunities.filter((opportunity) => opportunity.staleDays >= 14);
	const moving = openOpportunities.filter((opportunity) => opportunity.staleDays < 14);
	const totalMoneySummary = buildMoneySummary([...openOpportunities, ...onHold], text);

	return {
		id: 'pipeline-health',
		title: text.kpiPipelineHealth,
		description: text.kpiPipelineHealthDescription,
		totalValue: totalMoneySummary.displayValue,
		totalLabel: text.kpiOpenValue,
		totalMoneyDetails: totalMoneySummary.moneyDetails,
		segments: [
			amountSegment(text.kpiMoving, moving, movingColor, text),
			amountSegment(text.kpiStalled, stalled, neutralColor, text),
			amountSegment(text.kpiOnHold, onHold, attentionColor, text)
		]
	};
}

function buildRelationshipHealth(organizations: CRMOrganization[], text: CRMText, today: string): CRMKPICardData {
	const activeOrganizations = organizations.filter((organization) => organization.status !== 'paused');
	const needsAttention = organizations.filter((organization) => organization.status === 'paused' || organization.importance === 'low');
	const attentionIDs = new Set(needsAttention.map((organization) => organization.id));
	const recentContactThreshold = shiftCRMDate(today, -recentContactWindowDays);
	const recentlyContacted = activeOrganizations.filter((organization) => !attentionIDs.has(organization.id) && organization.lastContactDate >= recentContactThreshold);
	const recentIDs = new Set(recentlyContacted.map((organization) => organization.id));
	const stable = activeOrganizations.filter((organization) => !attentionIDs.has(organization.id) && !recentIDs.has(organization.id));

	return {
		id: 'relationship-health',
		title: text.kpiRelationshipHealth,
		description: text.kpiRelationshipHealthDescription,
		totalValue: String(activeOrganizations.length),
		totalLabel: text.kpiActiveRelationships,
		segments: [
			countSegment(text.kpiRecentlyContacted, recentlyContacted.length, movingColor, text.kpiCountSuffix),
			countSegment(text.kpiStable, stable.length, neutralColor, text.kpiCountSuffix),
			countSegment(text.kpiNeedsAttention, needsAttention.length, attentionColor, text.kpiCountSuffix)
		]
	};
}

function buildFollowUpHealth(nextActions: CRMNextAction[], text: CRMText): CRMKPICardData {
	const openActions = nextActions.filter((action) => action.status !== 'done');
	const overdue = openActions.filter((action) => getActionUrgency(action) === 'overdue');
	const dueSoon = openActions.filter((action) => ['today', 'due_soon'].includes(getActionUrgency(action)));
	const scheduled = openActions.filter((action) => getActionUrgency(action) === 'scheduled');

	return {
		id: 'follow-up-health',
		title: text.kpiFollowUpHealth,
		description: text.kpiFollowUpHealthDescription,
		totalValue: String(openActions.length),
		totalLabel: text.kpiOpenActions,
		segments: [
			countSegment(text.kpiScheduled, scheduled.length, movingColor, text.kpiCountSuffix),
			countSegment(text.kpiDueSoon, dueSoon.length, neutralColor, text.kpiCountSuffix),
			countSegment(text.kpiOverdue, overdue.length, attentionColor, text.kpiCountSuffix)
		]
	};
}

function buildPipelineComposition(organizations: CRMOrganization[], opportunities: CRMOpportunity[], stages: CRMPipelineStage[], text: CRMText): CRMKPICardData {
	const openOpportunities = opportunities.filter((opportunity) => opportunityOutcome(stages, opportunity) === 'open');
	const sales = openOpportunities.filter((opportunity) => getProgressKind(opportunity, findOrganizationByID(organizations, opportunity.organizationID)) === 'sales');
	const investment = openOpportunities.filter((opportunity) => getProgressKind(opportunity, findOrganizationByID(organizations, opportunity.organizationID)) === 'investment');
	const collaboration = openOpportunities.filter((opportunity) => {
		const kind = getProgressKind(opportunity, findOrganizationByID(organizations, opportunity.organizationID));
		return ['sponsorship', 'partnership', 'procurement'].includes(kind);
	});

	return {
		id: 'pipeline-composition',
		title: text.kpiPipelineComposition,
		description: text.kpiPipelineCompositionDescription,
		totalValue: String(openOpportunities.length),
		totalLabel: text.kpiOpenProgress,
		segments: [
			countSegment(text.kpiSales, sales.length, movingColor, text.kpiCountSuffix),
			countSegment(text.kpiInvestment, investment.length, neutralColor, text.kpiCountSuffix),
			countSegment(text.kpiCollaboration, collaboration.length, attentionColor, text.kpiCountSuffix)
		]
	};
}

export function buildCRMKPICards(
	organizations: CRMOrganization[],
	opportunities: CRMOpportunity[],
	nextActions: CRMNextAction[],
	stages: CRMPipelineStage[],
	text: CRMText,
	today = currentCRMDate()
): CRMKPICardData[] {
	return [
		buildPipelineHealth(opportunities, stages, text),
		buildRelationshipHealth(organizations, text, today),
		buildFollowUpHealth(nextActions, text),
		buildPipelineComposition(organizations, opportunities, stages, text)
	];
}
