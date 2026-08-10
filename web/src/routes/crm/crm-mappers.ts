import type { UserRecord } from '$lib/organization/types';
import type {
	CRMAccountPayload,
	CRMAccountResponse,
	CRMActivityPayload,
	CRMActivityResponse,
	CRMContactPayload,
	CRMContactResponse,
	CRMDataResponse,
	CRMOpportunityPayload,
	CRMOpportunityResponse
} from './crm-api-types';
import type {
	CRMAccount,
	CRMActivity,
	CRMContact,
	CRMCreateDraft,
	CRMCurrency,
	CRMNextAction,
	CRMOpportunity,
	CRMPipeline,
	CRMPipelineStage,
	CRMLostReason
} from './crm-types';

export type CRMViewData = {
	accounts: CRMAccount[];
	contacts: CRMContact[];
	opportunities: CRMOpportunity[];
	activities: CRMActivity[];
	nextActions: CRMNextAction[];
	pipelines: CRMPipeline[];
	stages: CRMPipelineStage[];
	lostReasons: CRMLostReason[];
};

export type CRMOwnerResolutionErrorCode = 'owner_not_found' | 'owner_ambiguous';

export class CRMOwnerResolutionError extends Error {
	constructor(readonly code: CRMOwnerResolutionErrorCode) {
		super(code);
		this.name = 'CRMOwnerResolutionError';
	}
}

export function mapCRMViewData(
	data: CRMDataResponse,
	people: UserRecord[],
	timeZone = browserTimeZone()
): CRMViewData {
	const stages = data.stages.map((stage) => ({ ...stage }));
	const opportunities = data.opportunities.map((opportunity) => mapOpportunity(opportunity, people));
	const activities = data.activities.map((activity) => mapActivity(activity));
	const accounts = data.accounts.map((account) => mapAccount(account, people, opportunities, activities, stages, timeZone));
	return {
		accounts,
		contacts: data.contacts.map((contact) => mapContact(contact)),
		opportunities,
		activities,
		nextActions: [],
		pipelines: data.pipelines.map((pipeline) => ({ ...pipeline })),
		stages,
		lostReasons: data.lostReasons.map((reason) => ({ ...reason }))
	};
}

export function accountPayload(account: CRMAccount): CRMAccountPayload {
	return {
		name: account.name,
		status: account.status,
		types: account.types,
		tags: account.tags,
		importance: account.importance,
		ownerPersonID: account.ownerPersonID ?? '',
		ownerCircleID: account.ownerCircleID,
		address: account.address,
		description: account.description
	};
}

export function accountPayloadFromDraft(
	draft: Extract<CRMCreateDraft, { kind: 'relationship' }>,
	owner: UserRecord
): CRMAccountPayload {
	return {
		name: draft.name,
		status: draft.status,
		types: draft.types,
		tags: draft.tags,
		importance: draft.importance,
		ownerPersonID: owner.userID,
		ownerCircleID: '',
		address: draft.address,
		description: draft.description
	};
}

export function contactPayload(contact: CRMContact): CRMContactPayload {
	return {
		accountID: contact.accountID,
		name: contact.name,
		email: contact.email,
		phone: contact.phone,
		title: contact.title,
		department: contact.department,
		isPrimary: contact.isPrimary,
		ownerPersonID: contact.ownerPersonID ?? '',
		ownerCircleID: contact.ownerCircleID,
		description: contact.note
	};
}

export function contactPayloadFromDraft(
	draft: Extract<CRMCreateDraft, { kind: 'contact' }>,
	owner: UserRecord
): CRMContactPayload {
	return {
		accountID: draft.accountID,
		name: draft.name,
		email: draft.email,
		phone: draft.phone,
		title: draft.title,
		department: '',
		isPrimary: draft.isPrimary,
		ownerPersonID: owner.userID,
		ownerCircleID: '',
		description: draft.note
	};
}

export function opportunityPayload(opportunity: CRMOpportunity): CRMOpportunityPayload {
	return {
		accountID: opportunity.accountID,
		business: opportunity.business,
		name: opportunity.name,
		pipeline: opportunity.pipeline ?? opportunity.kind ?? 'sales',
		ownerPersonID: opportunity.ownerPersonID ?? '',
		ownerCircleID: opportunity.ownerCircleID ?? '',
		amountMinor: majorToMinor(opportunity.expectedValue, opportunity.currency),
		currencyCode: opportunity.expectedValue === undefined ? '' : opportunity.currency,
		importance: opportunity.importance,
		dueAt: opportunity.targetDate ? localDateToUTC(opportunity.targetDate, opportunity.dueTimeZone ?? browserTimeZone()) : '',
		dueTimeZone: opportunity.targetDate ? opportunity.dueTimeZone ?? browserTimeZone() : '',
		description: opportunity.description ?? '',
		contacts: (opportunity.contacts ?? []).map((contact) => ({ ...contact }))
	};
}

export function opportunityPayloadFromDraft(
	draft: Extract<CRMCreateDraft, { kind: 'progress' }>,
	owner: UserRecord,
	timeZone: string
): CRMOpportunityPayload {
	return {
		accountID: draft.accountID,
		business: draft.business,
		name: draft.name,
		pipeline: draft.progressKind,
		ownerPersonID: owner.userID,
		ownerCircleID: '',
		amountMinor: majorToMinor(draft.amount, draft.currency),
		currencyCode: draft.amount === undefined ? '' : draft.currency,
		importance: draft.importance,
		dueAt: draft.targetDate ? localDateToUTC(draft.targetDate, timeZone) : '',
		dueTimeZone: draft.targetDate ? timeZone : '',
		description: draft.description,
		contacts: []
	};
}

export function activityPayload(activity: CRMActivity): CRMActivityPayload {
	return {
		accountID: activity.accountID,
		contactID: activity.contactID ?? '',
		opportunityID: activity.opportunityID ?? '',
		business: activity.business,
		kind: activity.kind === 'stage_change' ? 'event' : activity.kind,
		title: activity.title,
		occurredAt: activity.occurredAt,
		content: activity.summary
	};
}

export function activityPayloadFromDraft(draft: Extract<CRMCreateDraft, { kind: 'activity' }>): CRMActivityPayload {
	return {
		accountID: draft.accountID,
		contactID: '',
		opportunityID: draft.opportunityID ?? '',
		business: draft.business,
		kind: draft.activityKind === 'stage_change' ? 'event' : draft.activityKind,
		title: draft.title,
		occurredAt: new Date(draft.occurredAt).toISOString(),
		content: draft.summary
	};
}

export function resolveOwner(people: UserRecord[], hint: string, currentEmail: string): UserRecord {
	const normalizedHint = hint.trim().toLowerCase();
	const current = people.find((person) => person.email.toLowerCase() === currentEmail.toLowerCase());
	if (!normalizedHint && current) return current;
	const candidates = people.filter((person) => [person.userID, person.name ?? '', person.email]
		.some((value) => value.trim().toLowerCase() === normalizedHint));
	if (candidates.length === 1) return candidates[0];
	throw new CRMOwnerResolutionError(candidates.length === 0 ? 'owner_not_found' : 'owner_ambiguous');
}

export function browserTimeZone(): string {
	return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
}

export function localDateToUTC(date: string, timeZone: string): string {
	const [year, month, day] = date.split('-').map(Number);
	if (!year || !month || !day) return '';
	const desiredAsUTC = Date.UTC(year, month - 1, day, 23, 59, 59);
	let instant = desiredAsUTC;
	for (let attempt = 0; attempt < 3; attempt += 1) {
		const parts = dateParts(new Date(instant), timeZone);
		const representedAsUTC = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second);
		instant += desiredAsUTC - representedAsUTC;
	}
	const resolved = dateParts(new Date(instant), timeZone);
	if (resolved.year !== year || resolved.month !== month || resolved.day !== day || resolved.hour !== 23 || resolved.minute !== 59 || resolved.second !== 59) {
		throw new RangeError(`CRM due date ${date} does not exist in ${timeZone}`);
	}
	return new Date(instant).toISOString();
}

export function utcToLocalDate(dateTime: string | undefined, timeZone: string): string {
	if (!dateTime) return '';
	const date = new Date(dateTime);
	if (Number.isNaN(date.getTime())) return '';
	const parts = dateParts(date, timeZone);
	return `${parts.year}-${String(parts.month).padStart(2, '0')}-${String(parts.day).padStart(2, '0')}`;
}

function mapAccount(
	account: CRMAccountResponse,
	people: UserRecord[],
	opportunities: CRMOpportunity[],
	activities: CRMActivity[],
	stages: CRMPipelineStage[],
	timeZone: string
): CRMAccount {
	const owner = people.find((person) => person.userID === account.ownerPersonID);
	const accountOpportunities = opportunities.filter((opportunity) => opportunity.accountID === account.id);
	const openOpportunities = accountOpportunities.filter((opportunity) => stageOutcome(stages, opportunity) === 'open');
	const activityDates = activities
		.filter((activity) => activity.accountID === account.id)
		.map((activity) => utcToLocalDate(activity.occurredAt, timeZone))
		.filter(Boolean)
		.sort();
	const dueDates = openOpportunities.map((opportunity) => opportunity.targetDate).filter(Boolean).sort();
	return {
		id: account.id,
		name: account.name,
		types: account.types,
		status: account.status,
		importance: account.importance,
		ownerPersonID: account.ownerPersonID,
		ownerCircleID: account.ownerCircleID,
		ownerName: owner?.name || owner?.email || account.ownerPersonID,
		ownerEmail: owner?.email ?? '',
		team: account.ownerCircleID ?? '',
		address: account.address,
		tags: account.tags,
		description: account.description ?? '',
		lastContactDate: activityDates.at(-1) ?? utcToLocalDate(account.audit.createdAt, timeZone),
		nextActionDate: dueDates[0] ?? '',
		openOpportunityCount: openOpportunities.length,
		expectedValues: moneyTotals(openOpportunities)
	};
}

function mapContact(contact: CRMContactResponse): CRMContact {
	return {
		id: contact.id,
		accountID: contact.accountID,
		name: contact.name,
		title: contact.title ?? '',
		department: contact.department,
		email: contact.email ?? '',
		phone: contact.phone,
		isPrimary: contact.isPrimary,
		note: contact.description,
		ownerPersonID: contact.ownerPersonID,
		ownerCircleID: contact.ownerCircleID
	};
}

function mapOpportunity(opportunity: CRMOpportunityResponse, people: UserRecord[]): CRMOpportunity {
	const owner = people.find((person) => person.userID === opportunity.ownerPersonID);
	const timeZone = opportunity.dueTimeZone || 'UTC';
	return {
		id: opportunity.id,
		accountID: opportunity.accountID ?? '',
		business: opportunity.business ?? '',
		name: opportunity.name,
		pipeline: opportunity.pipeline,
		stage: opportunity.stage,
		stagePosition: opportunity.stagePosition,
		stageChangedAt: opportunity.stageChangedAt,
		ownerPersonID: opportunity.ownerPersonID,
		ownerCircleID: opportunity.ownerCircleID,
		ownerName: owner?.name || owner?.email || opportunity.ownerPersonID,
		expectedValue: minorToMajor(opportunity.amountMinor, opportunity.currencyCode),
		currency: opportunity.currencyCode || 'KRW',
		baseAmountMinor: opportunity.baseAmountMinor,
		baseCurrencyCode: opportunity.baseCurrencyCode,
		importance: opportunity.importance,
		targetDate: utcToLocalDate(opportunity.dueAt, timeZone),
		dueTimeZone: timeZone,
		staleDays: daysSince(opportunity.stageChangedAt),
		kind: opportunity.pipeline,
		description: opportunity.description,
		lostReason: opportunity.lostReason,
		contacts: (opportunity.contacts ?? []).map((contact) => ({ ...contact }))
	};
}

function mapActivity(activity: CRMActivityResponse): CRMActivity {
	return {
		id: activity.id,
		accountID: activity.accountID ?? '',
		contactID: activity.contactID,
		opportunityID: activity.opportunityID,
		business: activity.business ?? '',
		kind: activity.kind,
		title: activity.title,
		occurredAt: activity.occurredAt,
		summary: activity.content ?? '',
		taskID: ''
	};
}

function stageOutcome(stages: CRMPipelineStage[], opportunity: CRMOpportunity): CRMPipelineStage['outcome'] | undefined {
	return stages.find((stage) => stage.pipeline === (opportunity.pipeline ?? opportunity.kind) && stage.stage === opportunity.stage)?.outcome;
}

function moneyTotals(opportunities: CRMOpportunity[]) {
	return opportunities.reduce<Partial<Record<CRMCurrency, number>>>((totals, opportunity) => {
		if (opportunity.expectedValue === undefined) return totals;
		return { ...totals, [opportunity.currency]: (totals[opportunity.currency] ?? 0) + opportunity.expectedValue };
	}, {});
}

function majorToMinor(value: number | undefined, currency: CRMCurrency): number | null {
	if (value === undefined) return null;
	return Math.round(value * currencyMultiplier(currency));
}

function minorToMajor(value: number | undefined, currency: CRMCurrency | ''): number | undefined {
	if (value === undefined) return undefined;
	return value / currencyMultiplier(currency || 'KRW');
}

function currencyMultiplier(currency: CRMCurrency): number {
	return currency === 'USD' || currency === 'EUR' ? 100 : 1;
}

function daysSince(dateTime: string): number {
	const date = Date.parse(dateTime);
	if (Number.isNaN(date)) return 0;
	return Math.max(0, Math.floor((Date.now() - date) / 86400000));
}

function dateParts(date: Date, timeZone: string) {
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(date);
	const value = (type: Intl.DateTimeFormatPartTypes): number => Number(parts.find((part) => part.type === type)?.value ?? 0);
	return { year: value('year'), month: value('month'), day: value('day'), hour: value('hour'), minute: value('minute'), second: value('second') };
}
