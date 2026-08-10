import type {
	CRMAccount,
	CRMAccountStatus,
	CRMAccountType,
	CRMActionUrgency,
	CRMActivity,
	CRMNextAction,
	CRMNextActionStatus,
	CRMOpportunity,
	CRMOpportunityStage,
	CRMProgressKind
} from './crm-types';
import { crmAccountTypes } from './crm-types';
import type { CRMText } from './text';
export { formatMoney, formatMoneyTotals } from './crm-money';

export type CRMTab = 'relationships' | 'contacts' | 'pipeline' | 'activities' | 'reports';
export type CRMAccountStatusFilter = CRMAccountStatus | 'all';
export type CRMAccountTypeFilter = CRMAccountType | 'all';
export type CRMBadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline';

export const crmNextActionStatusOrder: CRMNextActionStatus[] = ['todo', 'in_progress', 'waiting', 'done'];
export const crmAccountStatusOptions: CRMAccountStatusFilter[] = ['all', 'prospect', 'active', 'paused'];
export const crmAccountTypeOptions: CRMAccountTypeFilter[] = ['all', ...crmAccountTypes];
export const crmPrototypeToday = '2026-07-22';

export function accountMatchesFilters(
	account: CRMAccount,
	query: string,
	statusFilter: CRMAccountStatusFilter,
	typeFilter: CRMAccountTypeFilter
): boolean {
	const normalizedQuery = query.trim().toLowerCase();
	const matchesStatus = statusFilter === 'all' || account.status === statusFilter;
	const matchesType = typeFilter === 'all' || account.types.includes(typeFilter);
	if (!matchesStatus || !matchesType) return false;
	if (normalizedQuery === '') return true;
	return [account.name, account.ownerName, account.team, account.description, ...account.tags, ...account.types].some((value) =>
		value.toLowerCase().includes(normalizedQuery)
	);
}

export function findAccountByID(accounts: CRMAccount[], accountID: string): CRMAccount | undefined {
	return accounts.find((account) => account.id === accountID);
}

export function findNextActionByID(nextActions: CRMNextAction[], actionID: string | undefined): CRMNextAction | undefined {
	if (!actionID) return undefined;
	return nextActions.find((action) => action.id === actionID);
}

export function findPrimaryContactLabel(accountID: string, contacts: Array<{ accountID: string; isPrimary: boolean; name: string; title: string }>): string {
	const contact = contacts.find((candidate) => candidate.accountID === accountID && candidate.isPrimary);
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

export function getStatusVariant(status: CRMAccountStatus): CRMBadgeVariant {
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

export function getActivitiesByAccount(accountID: string, activities: CRMActivity[]): CRMActivity[] {
	return activities.filter((activity) => activity.accountID === accountID);
}

export function getOpportunitiesByAccount(accountID: string, opportunities: CRMOpportunity[]): CRMOpportunity[] {
	return opportunities.filter((opportunity) => opportunity.accountID === accountID);
}

export function getNextActionsByAccount(accountID: string, nextActions: CRMNextAction[]): CRMNextAction[] {
	return nextActions.filter((action) => action.accountID === accountID);
}

export function getProgressKind(opportunity: CRMOpportunity, account: CRMAccount | undefined): CRMProgressKind {
	if (opportunity.pipeline) return opportunity.pipeline;
	if (opportunity.kind) return opportunity.kind;
	if (account?.types.includes('investor')) return 'investment';
	if (account?.types.includes('sponsor')) return 'sponsorship';
	if (account?.types.includes('partner')) return 'partnership';
	if (account?.types.includes('vendor')) return 'procurement';
	return 'sales';
}
