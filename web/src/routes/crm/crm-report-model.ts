import type {
	CRMMoneyTotals,
	CRMNextAction,
	CRMOpportunity,
	CRMOrganization,
	CRMPipeline,
	CRMPipelineStage
} from './crm-types';
import { sumOpportunityMoney } from './crm-money';
import { isTaskStatusFinished } from '../task/task-status';
import { findOrganizationByID, getProgressKind } from './crm-view-model';

export type CRMReportPeriod = 'quarter' | 'next_90_days' | 'all';

export type CRMReportRange = { start: string; end: string } | null;

export type MoneyConverter = (value: number | undefined, currency: string) => number;

export type CRMMonthBucket = {
	month: string;
	openTotals: CRMMoneyTotals;
	wonTotals: CRMMoneyTotals;
	openAmount: number;
	wonAmount: number;
};

export type CRMStageRow = {
	stage: string;
	label: string;
	outcome: CRMPipelineStage['outcome'];
	count: number;
	totals: CRMMoneyTotals;
	amount: number;
};

export type CRMPipelineRow = {
	pipeline: string;
	label: string;
	color: string;
	count: number;
	totals: CRMMoneyTotals;
	amount: number;
};

export type CRMQuietAccount = {
	id: string;
	name: string;
	ownerName: string;
	ownerEmail: string;
	ownerSeed: string;
	lastContactDate: string;
	daysSinceContact: number;
};

export type CRMOwnerRow = {
	name: string;
	email: string;
	seed: string;
	organizationCount: number;
	openCount: number;
	openTotals: CRMMoneyTotals;
	openAmount: number;
	wonTotals: CRMMoneyTotals;
	wonAmount: number;
	missingActionCount: number;
};

export function crmReportRange(
	period: CRMReportPeriod,
	bounds: { today: string; next90DaysEnd: string; quarterStart: string; quarterEnd: string }
): CRMReportRange {
	if (period === 'all') return null;
	if (period === 'next_90_days') return { start: bounds.today, end: bounds.next90DaysEnd };
	return { start: bounds.quarterStart, end: bounds.quarterEnd };
}

export function opportunitiesInRange(opportunities: CRMOpportunity[], range: CRMReportRange): CRMOpportunity[] {
	if (!range) return opportunities;
	return opportunities.filter(
		(opportunity) => opportunity.targetDate >= range.start && opportunity.targetDate <= range.end
	);
}

export function outcomeOf(
	stages: CRMPipelineStage[],
	opportunity: CRMOpportunity
): CRMPipelineStage['outcome'] | undefined {
	return stages.find((stage) => stage.stage === opportunity.stage)?.outcome;
}

function withOutcome(
	opportunities: CRMOpportunity[],
	stages: CRMPipelineStage[],
	outcome: CRMPipelineStage['outcome']
): CRMOpportunity[] {
	return opportunities.filter((opportunity) => outcomeOf(stages, opportunity) === outcome);
}

export function convertedTotal(opportunities: CRMOpportunity[], convert: MoneyConverter): number {
	return opportunities.reduce((total, opportunity) => total + convert(opportunity.expectedValue, opportunity.currency), 0);
}

export function winRate(wonCount: number, lostCount: number): number | null {
	const decided = wonCount + lostCount;
	if (decided === 0) return null;
	return wonCount / decided;
}

export type CRMPeriodOutcome = {
	openTotals: CRMMoneyTotals;
	openAmount: number;
	wonTotals: CRMMoneyTotals;
	wonAmount: number;
	wonCount: number;
	lostCount: number;
	winRate: number | null;
};

export function periodOutcome(
	opportunities: CRMOpportunity[],
	stages: CRMPipelineStage[],
	convert: MoneyConverter
): CRMPeriodOutcome {
	const open = withOutcome(opportunities, stages, 'open');
	const won = withOutcome(opportunities, stages, 'won');
	const lost = withOutcome(opportunities, stages, 'lost');
	return {
		openTotals: sumOpportunityMoney(open),
		openAmount: convertedTotal(open, convert),
		wonTotals: sumOpportunityMoney(won),
		wonAmount: convertedTotal(won, convert),
		wonCount: won.length,
		lostCount: lost.length,
		winRate: winRate(won.length, lost.length)
	};
}

export function monthsBetween(firstMonth: string, lastMonth: string): string[] {
	if (!firstMonth || !lastMonth || firstMonth > lastMonth) return [];
	const months: string[] = [];
	let [year, month] = firstMonth.split('-').map(Number);
	const [lastYear, lastMonthNumber] = lastMonth.split('-').map(Number);
	while (year < lastYear || (year === lastYear && month <= lastMonthNumber)) {
		months.push(`${year}-${String(month).padStart(2, '0')}`);
		month += 1;
		if (month > 12) {
			month = 1;
			year += 1;
		}
	}
	return months;
}

export function monthBuckets(
	opportunities: CRMOpportunity[],
	stages: CRMPipelineStage[],
	range: CRMReportRange,
	convert: MoneyConverter
): CRMMonthBucket[] {
	const dated = opportunities.filter((opportunity) => opportunity.targetDate !== '');
	if (dated.length === 0) return [];
	const closeMonths = dated.map((opportunity) => opportunity.targetDate.slice(0, 7)).sort();
	const first = range ? range.start.slice(0, 7) : closeMonths[0];
	const last = range ? range.end.slice(0, 7) : closeMonths[closeMonths.length - 1];
	return monthsBetween(first, last).map((month) => {
		const matching = dated.filter((opportunity) => opportunity.targetDate.slice(0, 7) === month);
		const open = withOutcome(matching, stages, 'open');
		const won = withOutcome(matching, stages, 'won');
		return {
			month,
			openTotals: sumOpportunityMoney(open),
			wonTotals: sumOpportunityMoney(won),
			openAmount: convertedTotal(open, convert),
			wonAmount: convertedTotal(won, convert)
		};
	});
}

export function stageRows(
	opportunities: CRMOpportunity[],
	stages: CRMPipelineStage[],
	labelOf: (stage: string) => string,
	convert: MoneyConverter
): CRMStageRow[] {
	return [...stages]
		.sort((left, right) => left.position - right.position)
		.map((stage) => {
			const matching = opportunities.filter((opportunity) => opportunity.stage === stage.stage);
			return {
				stage: stage.stage,
				label: labelOf(stage.stage),
				outcome: stage.outcome,
				count: matching.length,
				totals: sumOpportunityMoney(matching),
				amount: convertedTotal(matching, convert)
			};
		});
}

export function pipelineRows(
	opportunities: CRMOpportunity[],
	organizations: CRMOrganization[],
	pipelines: CRMPipeline[],
	labelOf: (kind: string) => string,
	convert: MoneyConverter
): CRMPipelineRow[] {
	const byKind = new Map<string, CRMOpportunity[]>();
	for (const opportunity of opportunities) {
		const kind = getProgressKind(opportunity, findOrganizationByID(organizations, opportunity.organizationID));
		byKind.set(kind, [...(byKind.get(kind) ?? []), opportunity]);
	}
	return [...byKind.entries()]
		.map(([kind, matching]) => {
			const definition = pipelines.find((pipeline) => pipeline.pipeline === kind);
			return {
				pipeline: kind,
				label: definition?.label ?? labelOf(kind),
				color: definition?.color ?? '',
				count: matching.length,
				totals: sumOpportunityMoney(matching),
				amount: convertedTotal(matching, convert)
			};
		})
		.sort((left, right) => right.count - left.count);
}

export function daysBetween(earlier: string, later: string): number {
	const from = Date.parse(`${earlier}T00:00:00Z`);
	const to = Date.parse(`${later}T00:00:00Z`);
	if (Number.isNaN(from) || Number.isNaN(to)) return 0;
	return Math.max(0, Math.round((to - from) / 86400000));
}

export function quietAccounts(organizations: CRMOrganization[], today: string, limit: number): CRMQuietAccount[] {
	return [...organizations]
		.filter((organization) => organization.lastContactDate !== '')
		.sort((left, right) => left.lastContactDate.localeCompare(right.lastContactDate))
		.slice(0, limit)
		.map((organization) => ({
			id: organization.id,
			name: organization.name,
			ownerName: organization.ownerName,
			ownerEmail: organization.ownerEmail,
			ownerSeed: organization.ownerPersonID || organization.ownerEmail || organization.ownerName,
			lastContactDate: organization.lastContactDate,
			daysSinceContact: daysBetween(organization.lastContactDate, today)
		}));
}

export function ownerRows(
	opportunities: CRMOpportunity[],
	organizations: CRMOrganization[],
	nextActions: CRMNextAction[],
	stages: CRMPipelineStage[],
	convert: MoneyConverter
): CRMOwnerRow[] {
	const names = [
		...new Set([
			...organizations.map((organization) => organization.ownerName),
			...opportunities.map((opportunity) => opportunity.ownerName)
		])
	].filter(Boolean);

	return names.map((name) => {
		const owned = organizations.filter((organization) => organization.ownerName === name);
		const theirs = opportunities.filter((opportunity) => opportunity.ownerName === name);
		const open = withOutcome(theirs, stages, 'open');
		const won = withOutcome(theirs, stages, 'won');
		const email =
			owned.find((organization) => organization.ownerEmail)?.ownerEmail ??
			theirs.find((opportunity) => opportunity.ownerEmail)?.ownerEmail ??
			'';
		const personID =
			owned.find((organization) => organization.ownerPersonID)?.ownerPersonID ??
			theirs.find((opportunity) => opportunity.ownerPersonID)?.ownerPersonID ??
			'';
		return {
			name,
			email,
			seed: personID || email || name,
			organizationCount: owned.length,
			openCount: open.length,
			openTotals: sumOpportunityMoney(open),
			openAmount: convertedTotal(open, convert),
			wonTotals: sumOpportunityMoney(won),
			wonAmount: convertedTotal(won, convert),
			missingActionCount: open.filter(
				(opportunity) =>
					!opportunity.nextActionID ||
					!nextActions.some((action) => action.id === opportunity.nextActionID && !isTaskStatusFinished(action.status))
			).length
		};
	});
}
