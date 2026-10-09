import type { CRMOrganization, CRMCurrency, CRMNextAction, CRMOpportunity, CRMPipeline, CRMPipelineStage } from './crm-types';
import { currentCRMDate, shiftCRMDate } from './crm-date';
import { isTaskStatusFinished } from '../task/task-status';
import { collapsedViewMoneyTotal, formatMoney, formatMoneyTotals, formatViewMoney, sumOpportunityMoney } from './crm-money';
import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
import type { Locale } from '$lib/i18n/locale.svelte';
import { crmLabel } from './crm-labels';
import { findOrganizationByID, getActionUrgency, getProgressKind } from './crm-view-model';
import type { CRMText } from './text';
import type { CRMViewCurrencyReader } from './crm-view-currency.svelte';

export type CRMKPIMoneyDetail = {
	currency: CRMCurrency;
	displayValue: string;
};

export type CRMKPITone = 'neutral' | 'attention';

export type CRMKPISegment = {
	label: string;
	value: number;
	displayValue: string;
	tone: CRMKPITone;
	color?: string;
	moneyDetails?: CRMKPIMoneyDetail[];
};

export type CRMKPICardData = {
	id: string;
	totalValue: string;
	totalLabel: string;
	totalMoneyDetails?: CRMKPIMoneyDetail[];
	segments: CRMKPISegment[];
};

const recentContactWindowDays = 30;
const namedCompositionSegments = 3;

function countSegment(label: string, value: number, tone: CRMKPITone, suffix: string): CRMKPISegment {
	return { label, value, displayValue: `${value}${suffix}`, tone };
}

function buildMoneySummary(
	catalogue: CurrencyCatalogue,
	opportunities: CRMOpportunity[],
	text: CRMText,
	locale: Locale,
	view?: CRMViewCurrencyReader
): { displayValue: string; moneyDetails?: CRMKPIMoneyDetail[] } {
	const totals = sumOpportunityMoney(opportunities);
	const collapsedValue = view ? collapsedViewMoneyTotal(totals, view) : undefined;
	if (view && collapsedValue !== undefined) {
		return {
			displayValue: formatViewMoney({ value: collapsedValue, currency: view.selected, isConverted: true }, text.nothingToTotal, locale)
		};
	}
	const moneyDetails = catalogue.reduce<CRMKPIMoneyDetail[]>((details, entry) => {
		const amount = totals[entry.code];
		if (amount === undefined) return details;
		return [
			...details,
			{ currency: entry.code, displayValue: formatMoney(amount, entry.code, text.nothingToTotal, locale) }
		];
	}, []);
	if (moneyDetails.length <= 1) return { displayValue: formatMoneyTotals(totals, catalogue, text.nothingToTotal, locale) };
	const primary = moneyDetails.find((detail) => detail.currency === view?.selected) ?? moneyDetails[0];
	return { displayValue: primary.displayValue, moneyDetails };
}

function amountSegment(catalogue: CurrencyCatalogue, label: string, opportunities: CRMOpportunity[], tone: CRMKPITone, text: CRMText, locale: Locale, view?: CRMViewCurrencyReader): CRMKPISegment {
	const moneySummary = buildMoneySummary(catalogue, opportunities, text, locale, view);
	return {
		label,
		value: opportunities.length,
		displayValue: moneySummary.moneyDetails
			? `${opportunities.length}${text.kpiCountSuffix}`
			: moneySummary.displayValue,
		tone,
		moneyDetails: moneySummary.moneyDetails
	};
}

function opportunityOutcome(stages: CRMPipelineStage[], opportunity: CRMOpportunity): CRMPipelineStage['outcome'] | undefined {
	return stages.find((stage) => stage.stage === opportunity.stage)?.outcome;
}

function buildPipelineHealth(catalogue: CurrencyCatalogue, opportunities: CRMOpportunity[], stages: CRMPipelineStage[], text: CRMText, locale: Locale, view?: CRMViewCurrencyReader): CRMKPICardData {
	const openOpportunities = opportunities.filter((opportunity) => opportunityOutcome(stages, opportunity) === 'open');
	const onHold = opportunities.filter((opportunity) => opportunityOutcome(stages, opportunity) === 'on_hold');
	const stalled = openOpportunities.filter((opportunity) => opportunity.staleDays >= 14);
	const moving = openOpportunities.filter((opportunity) => opportunity.staleDays < 14);
	const totalMoneySummary = buildMoneySummary(catalogue, [...openOpportunities, ...onHold], text, locale, view);

	return {
		id: 'pipeline-health',
		totalValue: totalMoneySummary.displayValue,
		totalLabel: text.kpiOpenValue,
		totalMoneyDetails: totalMoneySummary.moneyDetails,
		segments: [
			amountSegment(catalogue, text.kpiMoving, moving, 'neutral', text, locale, view),
			amountSegment(catalogue, text.kpiStalled, stalled, 'neutral', text, locale, view),
			amountSegment(catalogue, text.kpiOnHold, onHold, 'attention', text, locale, view)
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
		totalValue: String(organizations.length),
		totalLabel: text.kpiRelationships,
		segments: [
			countSegment(text.kpiRecentlyContacted, recentlyContacted.length, 'neutral', text.kpiCountSuffix),
			countSegment(text.kpiStable, stable.length, 'neutral', text.kpiCountSuffix),
			countSegment(text.kpiNeedsAttention, needsAttention.length, 'attention', text.kpiCountSuffix)
		]
	};
}

function buildFollowUpHealth(nextActions: CRMNextAction[], text: CRMText, today: string): CRMKPICardData {
	const openActions = nextActions.filter((action) => !isTaskStatusFinished(action.status));
	const overdue = openActions.filter((action) => getActionUrgency(action, today) === 'overdue');
	const dueSoon = openActions.filter((action) => ['today', 'due_soon'].includes(getActionUrgency(action, today)));
	const scheduled = openActions.filter((action) => getActionUrgency(action, today) === 'scheduled');

	return {
		id: 'follow-up-health',
		totalValue: String(openActions.length),
		totalLabel: text.kpiOpenActions,
		segments: [
			countSegment(text.kpiScheduled, scheduled.length, 'neutral', text.kpiCountSuffix),
			countSegment(text.kpiDueSoon, dueSoon.length, 'neutral', text.kpiCountSuffix),
			countSegment(text.kpiOverdue, overdue.length, 'attention', text.kpiCountSuffix)
		]
	};
}

function compositionSegment(kind: string, count: number, pipelines: CRMPipeline[], text: CRMText): CRMKPISegment {
	const definition = pipelines.find((pipeline) => pipeline.pipeline === kind);
	return {
		label: definition?.label ?? crmLabel(text.progressKinds, kind),
		value: count,
		displayValue: `${count}${text.kpiCountSuffix}`,
		tone: 'neutral',
		color: definition?.color
	};
}

function buildPipelineComposition(
	organizations: CRMOrganization[],
	opportunities: CRMOpportunity[],
	pipelines: CRMPipeline[],
	stages: CRMPipelineStage[],
	text: CRMText
): CRMKPICardData {
	const openOpportunities = opportunities.filter((opportunity) => opportunityOutcome(stages, opportunity) === 'open');
	const countsByKind = new Map<string, number>();
	for (const opportunity of openOpportunities) {
		const kind = getProgressKind(opportunity, findOrganizationByID(organizations, opportunity.organizationID));
		countsByKind.set(kind, (countsByKind.get(kind) ?? 0) + 1);
	}
	const ranked = [...countsByKind.entries()].sort(([, left], [, right]) => right - left);
	const named = ranked.slice(0, namedCompositionSegments);
	const remainder = ranked.slice(namedCompositionSegments).reduce((sum, [, count]) => sum + count, 0);
	const segments = named.map(([kind, count]) => compositionSegment(kind, count, pipelines, text));
	if (remainder > 0) segments.push(countSegment(text.kpiOtherProgressKinds, remainder, 'neutral', text.kpiCountSuffix));

	return {
		id: 'pipeline-composition',
		totalValue: String(openOpportunities.length),
		totalLabel: text.kpiOpenProgress,
		segments
	};
}

export function buildCRMKPICards(
	catalogue: CurrencyCatalogue,
	organizations: CRMOrganization[],
	opportunities: CRMOpportunity[],
	nextActions: CRMNextAction[],
	pipelines: CRMPipeline[],
	stages: CRMPipelineStage[],
	text: CRMText,
	today = currentCRMDate(),
	locale: Locale = 'ko',
	view?: CRMViewCurrencyReader
): CRMKPICardData[] {
	return [
		buildPipelineHealth(catalogue, opportunities, stages, text, locale, view),
		buildRelationshipHealth(organizations, text, today),
		buildFollowUpHealth(nextActions, text, today),
		buildPipelineComposition(organizations, opportunities, pipelines, stages, text)
	];
}
