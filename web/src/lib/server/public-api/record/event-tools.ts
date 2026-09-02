import { sizeOfHours, sizeOfWholeDays } from '$lib/task/task-sizes';
import { instantWritten, weekWindow } from './days';
import { peopleOfHints } from './people';
import type { RecordContext } from './company';
import { deleteTask, rowOfSavedID, saveTask, taskOfHint, tasksOfCompany, type TaskRow } from './tasks';

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

export function sizeOfEvent(startsAt: string, endsAt: string, isWholeDay: boolean): string {
	const hours = (new Date(endsAt).getTime() - new Date(startsAt).getTime()) / millisecondsPerHour;
	if (!isWholeDay) return sizeOfHours(hours);
	return sizeOfWholeDays(Math.max(1, Math.round(hours / 24)));
}

function locationNameOf(location: unknown): string {
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
		startsAt: row.starts_at ?? '',
		endsAt: row.ends_at ?? '',
		isWholeDay: row.is_whole_day,
		...(row.notify_minutes_before ? { notifyMinutesBefore: row.notify_minutes_before } : {}),
		participants: attendeesOfRow(context, row),
		updatedAt: row.updated_at
	};
}

// Everyone attending is the company, so it is read here rather than left as a
// flag nothing acts on.
function attendeesOf(context: RecordContext, written: EventWritten, row: TaskRow | null): string[] {
	if (written.everyoneAttends) return context.people.map((person) => person.personID);
	if (written.participantPersonHints !== undefined) {
		const named = peopleOfHints(context.people, written.participantPersonHints, 'participant').map(
			(person) => person.personID
		);
		return named.length > 0 ? named : [context.requesterID];
	}
	if (row) return row.task_participant.map(({ member_id }) => member_id);
	return [context.requesterID];
}

function eventWriteArguments(
	context: RecordContext,
	written: EventWritten,
	row: TaskRow | null
): Record<string, unknown> {
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
	return {
		target_task_id: row?.id ?? null,
		target_size: sizeOfEvent(startsAt, endsAt, isWholeDay),
		target_title: written.title ?? row?.title ?? '',
		target_note: written.note !== undefined ? written.note || null : row?.note ?? null,
		target_location: location ? { name: location } : null,
		target_starts_at: startsAt,
		target_ends_at: endsAt,
		target_is_whole_day: isWholeDay,
		target_is_event: true,
		target_notify_minutes_before: notify !== null && notify > 0 ? notify : null,
		target_participant_ids: attendeesOf(context, written, row),
		...(row ? { target_expected_updated_at: row.updated_at } : {})
	};
}

async function eventOfHint(context: RecordContext, hint: string): Promise<TaskRow> {
	const events = await tasksOfCompany(context.caller, true);
	return taskOfHint(events, hint, 'event', context.requesterID);
}

async function eventByID(context: RecordContext, eventID: string): Promise<TaskRow> {
	return rowOfSavedID(await tasksOfCompany(context.caller, true), eventID, 'event');
}

export async function eventAdd(context: RecordContext, input: EventWritten): Promise<AnsweredEvent> {
	if (!input.title?.trim()) throw new Error('an event needs a title');
	const saved = await saveTask(context.caller, eventWriteArguments(context, input, null));
	return answeredEvent(context, await eventByID(context, saved));
}

export async function eventUpdate(
	context: RecordContext,
	input: EventWritten & { eventHint?: string }
): Promise<AnsweredEvent> {
	if (!input.eventHint) throw new Error('an update names the event it changes');
	const row = await eventOfHint(context, input.eventHint);
	const saved = await saveTask(context.caller, eventWriteArguments(context, input, row));
	return answeredEvent(context, await eventByID(context, saved));
}

export async function eventDelete(
	context: RecordContext,
	input: { eventHint?: string }
): Promise<{ eventID: string; deleted: true }> {
	if (!input.eventHint) throw new Error('a deletion names the event it removes');
	const row = await eventOfHint(context, input.eventHint);
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
	const from = input.startsAt
		? instantWritten(context.labels.timezone, input.startsAt)
		: context.now.toISOString();
	const to = input.endsAt
		? instantWritten(context.labels.timezone, input.endsAt, true)
		: new Date(new Date(from).getTime() + 30 * 24 * 60 * 60 * 1000).toISOString();
	return { from, to };
}

export async function eventList(context: RecordContext, input: EventListInput) {
	const window = eventWindowOf(context, input);
	const { data, error } = await context.caller
		.from('task')
		.select(
			'id, title, status, note, location, business, type, size, is_event, is_whole_day, notify_minutes_before, starts_at, ends_at, updated_at, task_participant (member_id)'
		)
		.eq('is_event', true)
		.neq('status', 'rejected')
		.lt('starts_at', window.to)
		.gte('ends_at', window.from)
		.order('starts_at')
		.returns<TaskRow[]>();
	if (error) throw new Error(error.message);

	const found = (data ?? []).filter((row) => {
		if (!input.query) return true;
		const searched = `${row.title} ${row.note ?? ''} ${locationNameOf(row.location)}`;
		return searched.includes(input.query);
	});
	const kept = input.limit && input.limit > 0 ? found.slice(0, input.limit) : found;
	return { events: kept.map((row) => answeredEvent(context, row)) };
}
