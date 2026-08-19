import type {
	CRMOrganization,
	CRMOrganizationStatus,
	CRMOrganizationType,
	CRMActionUrgency,
	CRMActivity,
	CRMNextAction,
	CRMNextActionStatus,
	CRMOpportunity,
	CRMOpportunityStage,
	CRMProgressKind
} from './crm-types';
import { crmOrganizationTypes } from './crm-types';
import type { CRMText } from './text';
export { formatMoney, formatMoneyTotals } from './crm-money';

export type CRMTab = 'relationships' | 'contacts' | 'pipeline' | 'activities' | 'reports' | 'definitions';
export type CRMOrganizationStatusFilter = CRMOrganizationStatus | 'all';
export type CRMOrganizationTypeFilter = CRMOrganizationType | 'all';
export type CRMBadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline';

export const crmNextActionStatusOrder: CRMNextActionStatus[] = ['todo', 'in_progress', 'waiting', 'done'];
export const crmOrganizationStatusOptions: CRMOrganizationStatusFilter[] = ['all', 'prospect', 'active', 'paused'];
export const crmOrganizationTypeOptions: CRMOrganizationTypeFilter[] = ['all', ...crmOrganizationTypes];
export const crmPrototypeToday = '2026-07-22';

export function organizationMatchesFilters(
	organization: CRMOrganization,
	query: string,
	statusFilter: CRMOrganizationStatusFilter,
	typeFilter: CRMOrganizationTypeFilter
): boolean {
	const normalizedQuery = query.trim().toLowerCase();
	const matchesStatus = statusFilter === 'all' || organization.status === statusFilter;
	const matchesType = typeFilter === 'all' || organization.types.includes(typeFilter);
	if (!matchesStatus || !matchesType) return false;
	if (normalizedQuery === '') return true;
	return [organization.name, organization.ownerName, organization.team, organization.description, ...organization.tags, ...organization.types].some((value) =>
		value.toLowerCase().includes(normalizedQuery)
	);
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

export function formatCRMDate(date: string): string {
	const parsed = new Date(`${date}T00:00:00`);
	if (Number.isNaN(parsed.getTime())) return date;
	return new Intl.DateTimeFormat('ko-KR', { month: 'short', day: 'numeric' }).format(parsed);
}

export function formatCRMDateTime(dateTime: string): string {
	const parsed = new Date(dateTime);
	if (Number.isNaN(parsed.getTime())) return dateTime;
	return new Intl.DateTimeFormat('ko-KR', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(parsed);
}

export function daysLabel(days: number, text: CRMText): string {
	return text.daysAgo.replace('{days}', String(days));
}

export function getStatusVariant(status: CRMOrganizationStatus): CRMBadgeVariant {
	if (status === 'active') return 'default';
	if (status === 'paused') return 'outline';
	return 'secondary';
}

export function getStageVariant(stage: CRMOpportunityStage): CRMBadgeVariant {
	if (stage === 'won') return 'default';
	if (stage === 'lost') return 'destructive';
	if (stage === 'on_hold') return 'outline';
	return 'secondary';
}

export function opportunityStageLabel(stage: string, text: CRMText): string {
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
	if (action.status === 'done') return 'done';
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
