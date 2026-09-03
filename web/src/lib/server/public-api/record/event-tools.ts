import { sizeOfHours, sizeOfWholeDays } from '$lib/task/task-sizes';
import { crmColumnsWritten, writeCRMColumns } from './crm-tools';
import { dayIn, instantOfDay, instantWritten, isTheSameMoment, momentIn, weekWindow } from './days';
import { labelOf } from './labels';
import { peopleOfHints } from './people';
import type { RecordContext } from './company';
import {
	deleteTask,
	RecordRefusedTheWrite,
	rowOfSavedID,
	saveTask,
	taskOfHint,
	tasksOfCompany,
	taskSelection,
	type TaskRow
} from './tasks';

type EventWritten = {
	title?: string;
	startsAt?: string;
	endsAt?: string;
	note?: string;
	location?: string;
	isWholeDay?: boolean;
	everyoneAttends?: boolean;
	notifyMinutesBefore?: number;
	participantPersonHints?: string[];
	business?: string;
	type?: string;
	organizationHint?: string;
	opportunityHint?: string;
	contactHint?: string;
};

export type AnsweredAttendee = {
	personID: string;
	name: string;
	email?: string;
};

export type AnsweredEvent = {
	eventID: string;
	title: string;
	note: string;
	location: string;
	startsAt: string;
	endsAt: string;
	isWholeDay: boolean;
	notifyMinutesBefore?: number;
	participants: AnsweredAttendee[];
	updatedAt: string;
};

const millisecondsPerHour = 60 * 60 * 1000;

export class CalendarEventVersionConflict extends Error {
	readonly errorCode = 'calendar_event_version_conflict';
	readonly failureStage = 'resolution';
	readonly retryable = true;
	readonly safeRetry = false;

	constructor(readonly updatedAt: string) {
		super('this event changed since the version named here was read');
		this.name = 'CalendarEventVersionConflict';
	}
}

function versionOfEvent(row: TaskRow): string {
	const version = row.updated_at.trim();
	if (Number.isNaN(Date.parse(version))) {
		throw new Error(`the record answered event ${row.id} with an updatedAt that is not a moment`);
	}
	return version;
}

function refuseAVersionThatMovedOn(current: string, expectedUpdatedAt: string | undefined): void {
	if (expectedUpdatedAt === undefined) return;
	const named = Date.parse(expectedUpdatedAt);
	if (Number.isNaN(named)) throw new Error('expectedUpdatedAt is not a moment');
	if (named === Date.parse(current)) return;
	throw new CalendarEventVersionConflict(current);
}

export function sizeOfEvent(startsAt: string, endsAt: string, isWholeDay: boolean): string {
	const hours = (new Date(endsAt).getTime() - new Date(startsAt).getTime()) / millisecondsPerHour;
	if (!isWholeDay) return sizeOfHours(hours);
	return sizeOfWholeDays(Math.max(1, Math.round(hours / 24)));
}

export function locationNameOf(location: unknown): string {
	if (typeof location === 'string') return location;
	if (typeof location === 'object' && location !== null) {
		const named = (location as { name?: unknown }).name;
		if (typeof named === 'string') return named;
	}
	return '';
}

function attendeesOfRow(context: RecordContext, row: TaskRow): AnsweredAttendee[] {
	const personOf = new Map(context.people.map((person) => [person.personID, person]));
	return row.task_participant.map(({ member_id }) => {
		const person = personOf.get(member_id);
		if (!person) return { personID: member_id, name: '' };
		return { personID: member_id, name: person.name, ...(person.email ? { email: person.email } : {}) };
	});
}

function answeredEvent(context: RecordContext, row: TaskRow): AnsweredEvent {
	return {
		eventID: row.id,
		title: row.title,
		note: row.note ?? '',
		location: locationNameOf(row.location),
		startsAt: row.starts_at ? momentIn(context.labels.timezone, row.starts_at) : '',
		endsAt: row.ends_at ? momentIn(context.labels.timezone, row.ends_at) : '',
		isWholeDay: row.is_whole_day,
		...(row.notify_minutes_before ? { notifyMinutesBefore: row.notify_minutes_before } : {}),
		participants: attendeesOfRow(context, row),
		updatedAt: row.updated_at
	};
}

// Everyone attending is the company, so it is read here rather than left as a
// flag nothing acts on.
function attendeesOf(context: RecordContext, written: EventWritten, row: TaskRow | null): string[] {
	if (written.everyoneAttends) return [];
	if (written.participantPersonHints !== undefined) {
		return peopleOfHints(context.people, written.participantPersonHints, 'participant').map(
			(person) => person.personID
		);
	}
	if (row) return row.task_participant.map(({ member_id }) => member_id);
	return [context.requesterID];
}

function isRequestedOfSomebodyElse(requesterID: string, attendees: string[]): boolean {
	if (attendees.length === 0) return false;
	return !attendees.includes(requesterID);
}

function eventWriteArguments(
	context: RecordContext,
	written: EventWritten,
	row: TaskRow | null
): Record<string, unknown> {
	const business = labelOf(context.labels.businesses, written.business, row ? row.business : null);
	const type = labelOf(context.labels.types, written.type, row ? row.type : null);
	const timezone = context.labels.timezone;
	const startsAt =
		written.startsAt !== undefined ? instantWritten(timezone, written.startsAt) : row?.starts_at ?? '';
	const endsAt =
		written.endsAt !== undefined ? instantWritten(timezone, written.endsAt, true) : row?.ends_at ?? '';
	if (!startsAt || !endsAt) throw new Error('an event runs from a moment to a moment');
	if (endsAt <= startsAt) throw new Error('an event ends after it starts');

	const location = written.location !== undefined ? written.location.trim() : locationNameOf(row?.location);
	const notify =
		written.notifyMinutesBefore !== undefined
			? written.notifyMinutesBefore
			: row?.notify_minutes_before ?? null;
	const isWholeDay = written.isWholeDay ?? row?.is_whole_day ?? false;
	const attendees = attendeesOf(context, written, row);
	return {
		target_task_id: row?.id ?? null,
		...(row
			? {}
			: {
					target_status: isRequestedOfSomebodyElse(context.requesterID, attendees)
						? 'requested'
						: 'planned'
				}),
		target_size: sizeOfEvent(startsAt, endsAt, isWholeDay),
		target_title: written.title ?? row?.title ?? '',
		target_note: written.note !== undefined ? written.note || null : row?.note ?? null,
		target_business: business,
		target_type: type,
		target_location: location ? { name: location } : null,
		target_starts_at: startsAt,
		target_ends_at: endsAt,
		target_is_whole_day: isWholeDay,
		target_is_event: true,
		target_notify_minutes_before: notify !== null && notify > 0 ? notify : null,
		target_participant_ids: attendees,
		...(row ? { target_expected_updated_at: row.updated_at } : {})
	};
}

export async function eventOfHint(context: RecordContext, hint: string): Promise<TaskRow> {
	const events = await tasksOfCompany(context.caller, true);
	return taskOfHint(events, hint, 'event', context.requesterID);
}

async function eventByID(context: RecordContext, eventID: string): Promise<TaskRow> {
	return rowOfSavedID(await tasksOfCompany(context.caller, true), eventID, 'event');
}

export class CalendarEventDuplicate extends Error {
	readonly errorCode = 'calendar_event_duplicate';
	readonly failureStage = 'resolution';
	readonly retryable = false;
	readonly safeRetry = false;

	constructor(readonly eventID: string) {
		super('this company already holds this event at this time with these people');
		this.name = 'CalendarEventDuplicate';
	}
}

// The record's duplicate trigger names the clashing event by title and not by
// id, so which event it was has to be looked up rather than read off the error.
async function savedOrRefusedAsADuplicate(
	context: RecordContext,
	writeArguments: Record<string, unknown>
): Promise<string> {
	try {
		return await saveTask(context.caller, writeArguments);
	} catch (refusal) {
		const isDuplicate = refusal instanceof RecordRefusedTheWrite && refusal.errorCode === 'record_duplicate';
		if (!isDuplicate) throw refusal;
		throw new CalendarEventDuplicate(await eventAlreadyHeld(context, writeArguments));
	}
}

async function eventAlreadyHeld(
	context: RecordContext,
	writeArguments: Record<string, unknown>
): Promise<string> {
	const { data } = await context.caller
		.from('task')
		.select('id, starts_at, ends_at, task_participant (member_id)')
		.eq('is_event', true)
		.eq('title', writeArguments.target_title)
		.neq('id', writeArguments.target_task_id ?? '00000000-0000-0000-0000-000000000000')
		.returns<Pick<TaskRow, 'id' | 'starts_at' | 'ends_at' | 'task_participant'>[]>();

	const attendees = new Set(writeArguments.target_participant_ids as string[]);
	const clashing = (data ?? []).find(
		(row) =>
			isTheSameHeldMoment(row.starts_at, writeArguments.target_starts_at) &&
			isTheSameHeldMoment(row.ends_at, writeArguments.target_ends_at) &&
			row.task_participant.length === attendees.size &&
			row.task_participant.every(({ member_id }) => attendees.has(member_id))
	);
	return clashing?.id ?? '';
}

function isTheSameHeldMoment(held: string | null, written: unknown): boolean {
	if (!held || typeof written !== 'string') return false;
	return isTheSameMoment(held, written);
}

export async function eventAdd(context: RecordContext, input: EventWritten): Promise<AnsweredEvent> {
	if (!input.title?.trim()) throw new Error('an event needs a title');
	const saved = await savedOrRefusedAsADuplicate(context, eventWriteArguments(context, input, null));
	await writeCRMColumns(context, saved, await crmColumnsWritten(context, input, null));
	return answeredEvent(context, await eventByID(context, saved));
}

export async function eventUpdate(
	context: RecordContext,
	input: EventWritten & { eventHint?: string; expectedUpdatedAt?: string }
): Promise<AnsweredEvent> {
	if (!input.eventHint) throw new Error('an update names the event it changes');
	const row = await eventOfHint(context, input.eventHint);
	refuseAVersionThatMovedOn(versionOfEvent(row), input.expectedUpdatedAt);
	const saved = await savedOrRefusedAsADuplicate(context, eventWriteArguments(context, input, row));
	await writeCRMColumns(context, saved, await crmColumnsWritten(context, input, row));
	return answeredEvent(context, await eventByID(context, saved));
}

export async function eventDelete(
	context: RecordContext,
	input: { eventHint?: string; expectedUpdatedAt?: string }
): Promise<{ eventID: string; deleted: true }> {
	if (!input.eventHint) throw new Error('a deletion names the event it removes');
	const row = await eventOfHint(context, input.eventHint);
	refuseAVersionThatMovedOn(versionOfEvent(row), input.expectedUpdatedAt);
	await deleteTask(context.caller, row.id);
	return { eventID: row.id, deleted: true };
}

export type EventListInput = {
	startsAt?: string;
	endsAt?: string;
	weekFrom?: number;
	weekTo?: number;
	query?: string;
	limit?: number;
};

export function eventWindowOf(context: RecordContext, input: EventListInput): { from: string; to: string } {
	if (input.weekFrom !== undefined || input.weekTo !== undefined) {
		const window = weekWindow(
			context.labels.timezone,
			context.now,
			input.weekFrom ?? 0,
			input.weekTo ?? input.weekFrom ?? 0
		);
		return {
			from: instantWritten(context.labels.timezone, window.from),
			to: instantWritten(context.labels.timezone, window.to, true)
		};
	}
	if (!input.startsAt && !input.endsAt) return upcomingWindow(context);
	if (input.startsAt) {
		const from = instantWritten(context.labels.timezone, input.startsAt);
		const to = input.endsAt
			? instantWritten(context.labels.timezone, input.endsAt, true)
			: shiftedByDays(from, 1);
		return { from, to };
	}
	const to = instantWritten(context.labels.timezone, input.endsAt as string, true);
	return { from: shiftedByDays(to, -1), to };
}

function upcomingWindow(context: RecordContext): { from: string; to: string } {
	const today = dayIn(context.labels.timezone, context.now);
	const from = instantOfDay(context.labels.timezone, today);
	return { from, to: shiftedByDays(from, 7) };
}

function shiftedByDays(instant: string, days: number): string {
	return new Date(Date.parse(instant) + days * 24 * 60 * 60 * 1000).toISOString();
}

export async function eventList(context: RecordContext, input: EventListInput) {
	const window = eventWindowOf(context, input);
	const { data, error } = await context.caller
		.from('task')
		.select(taskSelection)
		.eq('is_event', true)
		.neq('status', 'rejected')
		.lt('starts_at', window.to)
		.gte('ends_at', window.from)
		.order('starts_at')
		.returns<TaskRow[]>();
	if (error) throw new Error(error.message);

	const asked = (input.query ?? '').trim().toLowerCase();
	const found = (data ?? []).filter((row) => {
		if (!asked) return true;
		const searched = `${row.title} ${row.note ?? ''} ${locationNameOf(row.location)}`;
		return searched.toLowerCase().includes(asked);
	});
	const kept = input.limit && input.limit > 0 ? found.slice(0, input.limit) : found;
	return { events: kept.map((row) => answeredEvent(context, row)) };
}
