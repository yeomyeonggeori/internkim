import { majorAmountOf, minorAmountOf, type CurrencyCatalogue } from '$lib/currency/currency-catalogue';
import type { OrgGroup, UserRecord } from '$lib/organization/types';
import type {
	CRMOrganizationPayload,
	CRMOrganizationResponse,
	CRMActivityPayload,
	CRMActivityResponse,
	CRMContactPayload,
	CRMContactResponse,
	CRMDataResponse,
	CRMOpportunityPayload,
	CRMOpportunityResponse
} from './crm-api-types';
import type { CRMVocabulary } from './crm-api-types';
import type { TaskVocabulary } from '$lib/flow/task-vocabulary';
import { crmStages } from './crm-stages';
import type {
	CRMOrganization,
	CRMActivity,
	CRMContact,
	CRMCreateDraft,
	CRMCurrency,
	CRMNextAction,
	CRMOpportunity,
	CRMPipeline,
	CRMPipelineStage
} from './crm-types';

export type CRMViewData = {
	organizations: CRMOrganization[];
	contacts: CRMContact[];
	opportunities: CRMOpportunity[];
	activities: CRMActivity[];
	nextActions: CRMNextAction[];
	pipelines: CRMPipeline[];
	stages: CRMPipelineStage[];
	vocabulary: CRMVocabulary;
	taskVocabulary: TaskVocabulary;
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
	catalogue: CurrencyCatalogue,
	timeZone = browserTimeZone(),
	groups: OrgGroup[] = []
): CRMViewData {
	const stages = crmStages;
	const opportunities = data.opportunities.map((opportunity) => mapOpportunity(opportunity, people, catalogue));
	const activities = data.activities.map((activity) => mapActivity(activity));
	const organizations = data.organizations.map((organization) => mapOrganization(organization, people, opportunities, activities, stages, timeZone, groups));
	return {
		organizations,
		contacts: data.contacts.map((contact) => mapContact(contact)),
		opportunities,
		activities,
		nextActions: nextActionsOf(activities, timeZone),
		pipelines: data.pipelines.map((pipeline) => ({ ...pipeline })),
		stages,
		vocabulary: structuredClone(data.vocabulary),
		taskVocabulary: structuredClone(data.taskVocabulary)
	};
}

export function organizationPayload(organization: CRMOrganization): CRMOrganizationPayload {
	return {
		name: organization.name,
		status: organization.status,
		types: organization.types,
		tags: organization.tags,
		importance: organization.importance,
		ownerPersonID: organization.ownerPersonID ?? '',
		ownerCircleID: organization.ownerCircleID,
		address: organization.address,
		description: organization.description
	};
}

export function organizationPayloadFromDraft(
	draft: Extract<CRMCreateDraft, { kind: 'relationship' }>,
	owner: UserRecord
): CRMOrganizationPayload {
	return {
		name: draft.name,
		status: draft.status,
		types: draft.types,
		tags: draft.tags,
		importance: draft.importance,
		ownerPersonID: owner.userID,
		ownerCircleID: owner.groupID ?? '',
		address: draft.address,
		description: draft.description
	};
}

export function contactPayload(contact: CRMContact): CRMContactPayload {
	return {
		organizationID: contact.organizationID,
		name: contact.name,
		email: contact.email,
		phone: contact.phone,
		title: contact.title,
		department: contact.department,
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
		organizationID: draft.organizationID,
		name: draft.name,
		email: draft.email,
		phone: draft.phone,
		title: draft.title,
		department: '',
		ownerPersonID: owner.userID,
		ownerCircleID: owner.groupID ?? '',
		description: draft.note
	};
}

export function opportunityPayload(opportunity: CRMOpportunity, catalogue: CurrencyCatalogue): CRMOpportunityPayload {
	return {
		organizationID: opportunity.organizationID,
		business: opportunity.business,
		name: opportunity.name,
		pipeline: opportunity.pipeline ?? opportunity.kind ?? 'sales',
		ownerPersonID: opportunity.ownerPersonID ?? '',
		ownerCircleID: opportunity.ownerCircleID ?? '',
		amountMinor: majorToMinor(opportunity.expectedValue, opportunity.currency, catalogue),
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
	timeZone: string,
	catalogue: CurrencyCatalogue
): CRMOpportunityPayload {
	return {
		organizationID: draft.organizationID,
		business: draft.business,
		name: draft.name,
		pipeline: draft.progressKind,
		ownerPersonID: owner.userID,
		ownerCircleID: owner.groupID ?? '',
		amountMinor: majorToMinor(draft.amount, draft.currency, catalogue),
		currencyCode: draft.amount === undefined ? '' : draft.currency,
		importance: draft.importance,
		dueAt: draft.targetDate ? localDateToUTC(draft.targetDate, timeZone) : '',
		dueTimeZone: draft.targetDate ? timeZone : '',
		description: draft.description,
		contacts: draft.contacts.map((contact) => ({ ...contact }))
	};
}

export function activityPayload(activity: CRMActivity): CRMActivityPayload {
	return {
		organizationID: activity.organizationID,
		contactID: activity.contactID ?? '',
		opportunityID: activity.opportunityID ?? '',
		business: activity.business,
		kind: activity.kind,
		title: activity.title,
		occurredAt: activity.occurredAt,
		content: activity.summary,
		taskStatus: activity.taskStatus ?? 'todo',
		taskOwnerID: activity.taskOwnerID ?? '',
		isEvent: Boolean(activity.calendarEventID),
		isWholeDay: activity.isWholeDay ?? false,
		startsAt: activity.calendarEventDate ?? '',
		endsAt: activity.calendarEndsAt ?? activity.calendarEventDate ?? '',
		notifyMinutesBefore: null,
		location: activity.calendarLocation ?? ''
	};
}

export function activityPayloadFromDraft(draft: Extract<CRMCreateDraft, { kind: 'activity' }>): CRMActivityPayload {
	return {
		organizationID: draft.organizationID,
		contactID: draft.contactID ?? '',
		opportunityID: draft.opportunityID ?? '',
		business: draft.business,
		kind: draft.activityKind === 'stage_change' ? 'event' : draft.activityKind,
		title: draft.title,
		occurredAt: new Date(draft.occurredAt).toISOString(),
		content: draft.summary,
		taskStatus: draft.taskStatus || 'todo',
		taskOwnerID: draft.taskOwnerID,
		isEvent: draft.calendar.isRequested,
		isWholeDay: draft.calendar.isAllDay,
		startsAt: draft.calendar.isRequested ? new Date(draft.calendar.startTime || draft.occurredAt).toISOString() : '',
		endsAt: draft.calendar.isRequested ? new Date(draft.calendar.endTime || draft.calendar.startTime || draft.occurredAt).toISOString() : '',
		notifyMinutesBefore: null,
		location: draft.calendar.location
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

function mapOrganization(
	organization: CRMOrganizationResponse,
	people: UserRecord[],
	opportunities: CRMOpportunity[],
	activities: CRMActivity[],
	stages: CRMPipelineStage[],
	timeZone: string,
	groups: OrgGroup[]
): CRMOrganization {
	const owner = people.find((person) => person.userID === organization.ownerPersonID);
	const organizationOpportunities = opportunities.filter((opportunity) => opportunity.organizationID === organization.id);
	const openOpportunities = organizationOpportunities.filter((opportunity) => stageOutcome(stages, opportunity) === 'open');
	const activityDates = activities
		.filter((activity) => activity.organizationID === organization.id)
		.map((activity) => utcToLocalDate(activity.occurredAt, timeZone))
		.filter(Boolean)
		.sort();
	const dueDates = openOpportunities.map((opportunity) => opportunity.targetDate).filter(Boolean).sort();
	return {
		id: organization.id,
		name: organization.name,
		types: organization.types,
		status: organization.status,
		importance: organization.importance,
		ownerPersonID: organization.ownerPersonID,
		ownerCircleID: organization.ownerCircleID,
		ownerName: owner?.name || owner?.email || organization.ownerPersonID,
		ownerEmail: owner?.email ?? '',
		team: groups.find((group) => group.id === organization.ownerCircleID)?.name ?? organization.ownerCircleID ?? '',
		address: organization.address,
		tags: organization.tags,
		description: organization.description ?? '',
		lastContactDate: activityDates.at(-1) ?? utcToLocalDate(organization.audit.createdAt, timeZone),
		nextActionDate: dueDates[0] ?? '',
		openOpportunityCount: openOpportunities.length,
		expectedValues: moneyTotals(openOpportunities)
	};
}

function mapContact(contact: CRMContactResponse): CRMContact {
	return {
		id: contact.id,
		organizationID: contact.organizationID,
		name: contact.name,
		title: contact.title ?? '',
		department: contact.department,
		email: contact.email ?? '',
		phone: contact.phone,
		note: contact.description,
		ownerPersonID: contact.ownerPersonID,
		ownerCircleID: contact.ownerCircleID
	};
}

function mapOpportunity(
	opportunity: CRMOpportunityResponse,
	people: UserRecord[],
	catalogue: CurrencyCatalogue
): CRMOpportunity {
	const owner = people.find((person) => person.userID === opportunity.ownerPersonID);
	const timeZone = opportunity.dueTimeZone || 'UTC';
	return {
		id: opportunity.id,
		organizationID: opportunity.organizationID ?? '',
		business: opportunity.business ?? '',
		name: opportunity.name,
		pipeline: opportunity.pipeline,
		stage: opportunity.stage,
		stagePosition: opportunity.stagePosition,
		stageChangedAt: opportunity.stageChangedAt,
		ownerPersonID: opportunity.ownerPersonID,
		ownerCircleID: opportunity.ownerCircleID,
		ownerName: owner?.name || owner?.email || opportunity.ownerPersonID,
		expectedValue: minorToMajor(opportunity.amountMinor, opportunity.currencyCode, catalogue),
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

const closedTaskStatuses = ['done', 'cancelled'];

function nextActionsOf(activities: CRMActivity[], timeZone: string): CRMNextAction[] {
	return activities
		.filter((activity) => activity.organizationID !== '' && !closedTaskStatuses.includes(activity.taskStatus ?? ''))
		.map((activity) => ({
			id: activity.id,
			organizationID: activity.organizationID,
			opportunityID: activity.opportunityID,
			title: activity.title,
			ownerName: activity.taskOwnerName ?? '',
			dueDate: utcToLocalDate(activity.occurredAt, timeZone),
			status: (activity.taskStatus ?? 'todo') as CRMNextAction['status']
		}));
}

function mapActivity(activity: CRMActivityResponse): CRMActivity {
	return {
		id: activity.id,
		organizationID: activity.organizationID ?? '',
		contactID: activity.contactID,
		opportunityID: activity.opportunityID,
		business: activity.business ?? '',
		kind: activity.kind,
		title: activity.title,
		occurredAt: activity.occurredAt,
		summary: activity.content ?? '',
		taskID: activity.id,
		taskStatus: activity.taskStatus,
		taskOwnerID: activity.taskOwnerID,
		taskOwnerName: activity.taskOwnerID,
		calendarEventID: activity.isEvent ? activity.id : undefined,
		calendarEventDate: activity.startsAt,
		isWholeDay: activity.isWholeDay,
		calendarEndsAt: activity.endsAt,
		calendarLocation: activity.location,
		calendarRegistrationState: activity.isEvent ? 'registered' : undefined
	};
}

function stageOutcome(stages: CRMPipelineStage[], opportunity: CRMOpportunity): CRMPipelineStage['outcome'] | undefined {
	return stages.find((stage) => stage.stage === opportunity.stage)?.outcome;
}

function moneyTotals(opportunities: CRMOpportunity[]) {
	return opportunities.reduce<Partial<Record<CRMCurrency, number>>>((totals, opportunity) => {
		if (opportunity.expectedValue === undefined) return totals;
		return { ...totals, [opportunity.currency]: (totals[opportunity.currency] ?? 0) + opportunity.expectedValue };
	}, {});
}

function majorToMinor(value: number | undefined, currency: CRMCurrency, catalogue: CurrencyCatalogue): number | null {
	if (value === undefined) return null;
	return minorAmountOf(value, currency, catalogue);
}

function minorToMajor(
	value: number | undefined,
	currency: CRMCurrency | '',
	catalogue: CurrencyCatalogue
): number | undefined {
	if (value === undefined) return undefined;
	return majorAmountOf(value, currency || 'KRW', catalogue);
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
