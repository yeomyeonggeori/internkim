import type {
	CRMOrganization,
	CRMOrganizationStatus,
	CRMOrganizationType,
	CRMActionUrgency,
	CRMActivity,
	CRMNextAction,
	CRMOpportunity,
	CRMOpportunityStage,
	CRMPipelineStage,
	CRMProgressKind
} from './crm-types';
import type { CRMImportance } from './crm-types';
import type { CRMContact } from './crm-types';
import { crmOrganizationTypes } from './crm-types';
import type { UserRecord } from '$lib/organization/types';
import { shiftCRMDate } from './crm-date';
import { crmStageOutcomes } from '$lib/crm/crm-stage';
import { taskStatus } from '$lib/task/central-task';
import {
	isTaskStatusCompleted,
	isTaskStatusInProgress,
	isTaskStatusPaused,
	isTaskStatusRejected,
	isTaskStatusStopped
} from '../task/task-status';
import type { CRMText } from './text';
import type { Locale } from '$lib/i18n/locale.svelte';
export { formatMoney, formatMoneyTotals } from './crm-money';
export { interimCurrencyCatalogue } from '$lib/currency/currency-catalogue';

export type CRMTab = 'relationships' | 'contacts' | 'pipeline' | 'activities' | 'reports' | 'definitions';
export type CRMOrganizationStatusFilter = CRMOrganizationStatus | 'all';
export type CRMOrganizationTypeFilter = CRMOrganizationType | 'all';
export type CRMImportanceFilter = CRMImportance | 'all';
export type CRMLastContactWindow = 'all' | 'within_7' | 'within_30' | 'within_90' | 'over_90';
export type CRMBadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline';

export const crmOrganizationStatusOptions: CRMOrganizationStatusFilter[] = ['all', 'prospect', 'active', 'paused'];
export const crmOrganizationTypeOptions: CRMOrganizationTypeFilter[] = ['all', ...crmOrganizationTypes];
export const crmPrototypeToday = '2026-07-22';
export const crmImportanceOptions: CRMImportance[] = ['high', 'medium', 'low'];
export const crmLastContactWindowOptions: Exclude<CRMLastContactWindow, 'all'>[] = ['within_7', 'within_30', 'within_90', 'over_90'];

const lastContactWindowDays: Record<Exclude<CRMLastContactWindow, 'all' | 'over_90'>, number> = {
	within_7: 7,
	within_30: 30,
	within_90: 90
};

export type CRMRelationshipFacets = {
	status: CRMOrganizationStatusFilter;
	type: CRMOrganizationTypeFilter;
	importance: CRMImportanceFilter;
	lastContact: CRMLastContactWindow;
};

export function matchesLastContactWindow(lastContactDate: string, window: CRMLastContactWindow, today: string): boolean {
	if (window === 'all') return true;
	if (window === 'over_90') return lastContactDate < shiftCRMDate(today, -90);
	return lastContactDate >= shiftCRMDate(today, -lastContactWindowDays[window]);
}

export function organizationMatchesFacets(
	organization: CRMOrganization,
	facets: CRMRelationshipFacets,
	today: string
): boolean {
	if (facets.status !== 'all' && organization.status !== facets.status) return false;
	if (facets.type !== 'all' && !organization.types.includes(facets.type)) return false;
	if (facets.importance !== 'all' && organization.importance !== facets.importance) return false;
	return matchesLastContactWindow(organization.lastContactDate, facets.lastContact, today);
}

export function effectiveContactOwner(
	contact: CRMContact,
	organizations: CRMOrganization[],
	people: UserRecord[]
): CRMPersonChip | undefined {
	const own = crmPersonChip(people, contact.ownerPersonID);
	if (own) return own;
	const organization = findOrganizationByID(organizations, contact.organizationID);
	if (!organization?.ownerName) return undefined;
	return {
		name: organization.ownerName,
		email: organization.ownerEmail,
		seed: organization.ownerPersonID || organization.ownerEmail || organization.ownerName
	};
}

export type CRMPersonChip = {
	name: string;
	email: string;
	seed: string;
};

export function crmPersonChip(
	people: UserRecord[],
	personID: string | undefined,
	recordedName = ''
): CRMPersonChip | undefined {
	const person = personID ? people.find((candidate) => candidate.memberID === personID) : undefined;
	const name = person?.name || person?.email || recordedName;
	if (!name) return undefined;
	const email = person?.email ?? '';
	return { name, email, seed: personID || email || name };
}

export function findOrganizationByID(organizations: CRMOrganization[], organizationID: string): CRMOrganization | undefined {
	return organizations.find((organization) => organization.id === organizationID);
}

export function findNextActionByID(nextActions: CRMNextAction[], actionID: string | undefined): CRMNextAction | undefined {
	if (!actionID) return undefined;
	return nextActions.find((action) => action.id === actionID);
}

export function findOrganizationContactLabel(organizationID: string, contacts: Array<{ organizationID: string; name: string; title: string }>): string {
	const contact = contacts.find((candidate) => candidate.organizationID === organizationID);
	if (!contact) return '';
	return `${contact.name} · ${contact.title}`;
}

function intlLocaleOf(locale: Locale): string {
	return locale === 'en' ? 'en-US' : 'ko-KR';
}

export function formatCRMDate(date: string, locale: Locale = 'ko'): string {
	const parsed = new Date(`${date}T00:00:00`);
	if (Number.isNaN(parsed.getTime())) return date;
	return new Intl.DateTimeFormat(intlLocaleOf(locale), { month: 'short', day: 'numeric' }).format(parsed);
}

export function formatCRMDateTime(dateTime: string, locale: Locale = 'ko'): string {
	const parsed = new Date(dateTime);
	if (Number.isNaN(parsed.getTime())) return dateTime;
	return new Intl.DateTimeFormat(intlLocaleOf(locale), {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	}).format(parsed);
}

export function daysLabel(days: number, text: CRMText): string {
	if (days === 1) return text.daysAgoOne;
	return text.daysAgo.replace('{days}', String(days));
}

export function getStatusVariant(status: CRMOrganizationStatus): CRMBadgeVariant {
	if (status === 'active') return 'default';
	if (status === 'paused') return 'secondary';
	return 'outline';
}

const accountStatusOrder: CRMOrganizationStatus[] = ['paused', 'prospect', 'active'];

export function accountStatusRank(status: CRMOrganizationStatus): number {
	return accountStatusOrder.indexOf(status);
}

export function getStageVariant(stage: CRMOpportunityStage): CRMBadgeVariant {
	const outcome = crmStageOutcomes[stage];
	if (outcome === 'won') return 'default';
	if (outcome === 'lost') return 'destructive';
	if (outcome === 'on_hold') return 'outline';
	return 'secondary';
}

export function opportunityStageLabel(stages: readonly CRMPipelineStage[], stage: string, text: CRMText): string {
	const named = stages.find((candidate) => candidate.stage === stage);
	if (named && named.label !== named.stage) return named.label;
	const labels: Readonly<Record<string, string>> = text.opportunityStages;
	return labels[stage] ?? stage.replaceAll('_', ' ');
}

export function getActionVariant(action: CRMNextAction): CRMBadgeVariant {
	const urgency = getActionUrgency(action);
	if (urgency === 'overdue') return 'destructive';
	if (urgency === 'today') return 'default';
	if (urgency === 'done' || urgency === 'scheduled') return 'outline';
	return 'secondary';
}

export function getActionUrgency(action: CRMNextAction, today = crmPrototypeToday): CRMActionUrgency {
	if (action.status === taskStatus.completed) return 'done';
	const dueTime = Date.parse(`${action.dueDate}T00:00:00Z`);
	const todayTime = Date.parse(`${today}T00:00:00Z`);
	if (Number.isNaN(dueTime) || Number.isNaN(todayTime)) return 'scheduled';
	const daysUntilDue = Math.round((dueTime - todayTime) / 86400000);
	if (daysUntilDue < 0) return 'overdue';
	if (daysUntilDue === 0) return 'today';
	if (daysUntilDue <= 3) return 'due_soon';
	return 'scheduled';
}

export function getActivitiesByOrganization(organizationID: string, activities: CRMActivity[]): CRMActivity[] {
	return activities.filter((activity) => activity.organizationID === organizationID);
}

export function getOpportunitiesByOrganization(organizationID: string, opportunities: CRMOpportunity[]): CRMOpportunity[] {
	return opportunities.filter((opportunity) => opportunity.organizationID === organizationID);
}

export function getNextActionsByOrganization(organizationID: string, nextActions: CRMNextAction[]): CRMNextAction[] {
	return nextActions.filter((action) => action.organizationID === organizationID);
}

export function getProgressKind(opportunity: CRMOpportunity, organization: CRMOrganization | undefined): CRMProgressKind {
	if (opportunity.pipeline) return opportunity.pipeline;
	if (opportunity.kind) return opportunity.kind;
	if (organization?.types.includes('investor')) return 'investment';
	if (organization?.types.includes('sponsor')) return 'sponsorship';
	if (organization?.types.includes('partner')) return 'partnership';
	if (organization?.types.includes('vendor')) return 'procurement';
	return 'sales';
}
