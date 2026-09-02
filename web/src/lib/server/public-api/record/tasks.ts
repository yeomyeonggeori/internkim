import type { SupabaseClient } from '@supabase/supabase-js';
import { centralStatusFromWord, compatibilityOwnerOf } from '$lib/task/central-task';
import { titleNearness } from './hint-nearness';
import { HintRefused, normalized, resolveHint, type HintMatcher, type HintSubject } from './hint-resolution';
import { peopleOfHints, personOfHint, type RecordPerson } from './people';

export type TaskRow = {
	id: string;
	title: string;
	status: string;
	note: string | null;
	location: unknown;
	business: string | null;
	type: string | null;
	size: string | null;
	is_event: boolean;
	is_whole_day: boolean;
	notify_minutes_before: number | null;
	starts_at: string | null;
	ends_at: string | null;
	updated_at: string;
	task_participant: { member_id: string }[];
};

export const taskSelection =
	'id, title, status, note, location, business, type, size, is_event, is_whole_day, notify_minutes_before, starts_at, ends_at, updated_at, task_participant (member_id)';

// Postgres says why it refused, and the three answers are different things: a
// permission the caller does not hold, a row that moved under them, and a rule
// of the record their input broke.
export class RecordRefusedTheWrite extends Error {
	constructor(
		reason: string,
		readonly status: number
	) {
		super(reason);
		this.name = 'RecordRefusedTheWrite';
	}
}

export class WriteNotReadBack extends Error {
	readonly errorCode = 'record_write_not_read_back';
	readonly failureStage = 'result_verification';
	readonly retryable = true;
	readonly safeRetry = false;

	constructor(subject: 'task' | 'event') {
		super(`the record saved this ${subject} and did not answer with it`);
		this.name = 'WriteNotReadBack';
	}
}

export function rowOfSavedID(rows: TaskRow[], savedID: string, subject: 'task' | 'event'): TaskRow {
	const saved = rows.find((row) => row.id === savedID);
	if (!saved) throw new WriteNotReadBack(subject);
	return saved;
}

const insufficientPrivilege = '42501';
const serializationFailure = '40001';

export function statusOfPostgresCode(code: string | undefined): number {
	if (code === insufficientPrivilege) return 403;
	if (code === serializationFailure) return 409;
	return 422;
}

export async function tasksOfCompany(caller: SupabaseClient, areEvents: boolean): Promise<TaskRow[]> {
	const { data, error } = await caller
		.from('task')
		.select(taskSelection)
		.eq('is_event', areEvents)
		.order('updated_at', { ascending: false })
		.returns<TaskRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

const taskMatcher: HintMatcher<TaskRow> = {
	identifiersOf: (task) => [task.id],
	titleOf: (task) => task.title,
	nearnessTo: (task, hint) => titleNearness(normalized(hint), normalized(task.title))
};

export function taskOfHint(
	tasks: TaskRow[],
	hint: string,
	subject: HintSubject = 'task',
	requesterID = ''
): TaskRow {
	const resolution = resolveHint(hint, tasks, {
		...taskMatcher,
		...(requesterID ? { isPreferred: isOwnedBy(requesterID) } : {})
	});
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		subject,
		hint.trim(),
		resolution.outcome,
		resolution.candidates.map((task) => ({ id: task.id, label: task.title }))
	);
}

function isOwnedBy(requesterID: string): (task: TaskRow) => boolean {
	return (task) =>
		compatibilityOwnerOf(task.task_participant.map(({ member_id }) => ({ id: member_id, name: '' }))).id ===
		requesterID;
}

// task_save writes every field it is given, so a patch that named only what
// changed would blank the rest. The row as it stands is the base; the input
// names the difference.
export function taskWriteArguments(
	row: TaskRow | null,
	written: {
		title?: string;
		status?: string;
		note?: string | null;
		business?: string | null;
		type?: string | null;
		size?: string | null;
		startsAt?: string | null;
		endsAt?: string | null;
		participantIDs?: string[];
	}
): Record<string, unknown> {
	const held = (name: keyof TaskRow) => (row ? row[name] : null);
	const writesDates = written.startsAt !== undefined || written.endsAt !== undefined || !row;
	return {
		target_task_id: row?.id ?? null,
		target_title: written.title ?? row?.title ?? '',
		target_status: centralStatusFromWord(written.status ?? row?.status ?? 'planned'),
		target_note: written.note !== undefined ? written.note : (held('note') as string | null),
		target_business: written.business !== undefined ? written.business : (held('business') as string | null),
		target_type: written.type !== undefined ? written.type : (held('type') as string | null),
		target_size: written.size !== undefined ? written.size : (held('size') as string | null),
		target_starts_at: written.startsAt !== undefined ? written.startsAt : row?.starts_at ?? null,
		target_ends_at: written.endsAt !== undefined ? written.endsAt : row?.ends_at ?? null,
		target_write_dates: writesDates,
		target_is_event: false,
		target_participant_ids: written.participantIDs ?? row?.task_participant.map((one) => one.member_id) ?? [],
		...(row ? { target_expected_updated_at: row.updated_at } : {})
	};
}

export async function saveTask(caller: SupabaseClient, argumentsOfSave: Record<string, unknown>): Promise<string> {
	const { data, error } = await caller.rpc('task_save', argumentsOfSave);
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	if (typeof data !== 'string') throw new RecordRefusedTheWrite('the record named no task it saved', 502);
	return data;
}

// PostgREST answers a delete row level security forbids with an empty list and
// no error, so the representation is the only proof anything went.
export async function deleteTask(caller: SupabaseClient, taskID: string): Promise<void> {
	const { data, error } = await caller.from('task').delete().eq('id', taskID).select('id').returns<{ id: string }[]>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	if ((data ?? []).length === 0) {
		throw new RecordRefusedTheWrite('this deleted nothing, which its permissions do not allow', 403);
	}
}

// A task nobody is named on has no owner, and the board reads the first
// participant as one. The requester standing alone is the sensible default.
export function participantsOfHints(
	people: RecordPerson[],
	hints: string[] | undefined,
	requesterID: string
): string[] | undefined {
	if (hints === undefined) return undefined;
	const named = peopleOfHints(people, hints, 'participant').map((person) => person.personID);
	return named.length > 0 ? named : [requesterID];
}

export function ownerOfScope(
	people: RecordPerson[],
	scope: string | undefined,
	hint: string | undefined,
	requesterID: string
): string | null {
	if (hint) return personOfHint(people, hint).personID;
	if (scope === 'all') return null;
	return requesterID;
}
