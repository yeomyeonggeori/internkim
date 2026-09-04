import type { SupabaseClient } from '@supabase/supabase-js';
import { centralStatusFromWord, compatibilityOwnerOf } from '$lib/task/central-task';
import { titleNearness } from './hint-nearness';
import { HintRefused, normalized, resolveHint, type HintMatcher, type HintSubject } from './hint-resolution';
import { peopleOfHints, personOfHint, type RecordPerson } from './people';

export type TaskRow = {
	id: string;
	parent_task_id: string | null;
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
	created_at: string;
	updated_at: string;
	organization_id: string | null;
	opportunity_id: string | null;
	contact_id: string | null;
	due_at: string | null;
	requester_id: string | null;
	task_participant: { member_id: string }[];
};

export const taskSelection =
	'id, parent_task_id, title, status, note, location, business, type, size, is_event, is_whole_day, notify_minutes_before, starts_at, ends_at, created_at, updated_at, organization_id, opportunity_id, contact_id, due_at, requester_id, task_participant (member_id)';

// Postgres says why it refused, and the three answers are different things: a
// permission the caller does not hold, a row that moved under them, and a rule
// of the record their input broke.
export class RecordRefusedTheWrite extends Error {
	constructor(
		reason: string,
		readonly status: number,
		readonly errorCode = 'record_refused'
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
const uniqueViolation = '23505';

export function statusOfPostgresCode(code: string | undefined): number {
	if (code === insufficientPrivilege) return 403;
	if (code === serializationFailure) return 409;
	if (code === uniqueViolation) return 409;
	return 422;
}

// Postgres answers 42501 without naming the policy that refused, so which rule
// the caller broke has to come from what the write was trying to change.
export function refusedWriteOf(reason: string, code: string | undefined, changesParticipants: boolean) {
	const status = statusOfPostgresCode(code);
	if (code === uniqueViolation) return new RecordRefusedTheWrite(reason, status, 'record_duplicate');
	if (status !== 403) return new RecordRefusedTheWrite(reason, status);
	if (changesParticipants) {
		return new RecordRefusedTheWrite(
			'only the owner of this task or an admin can change who takes part in it',
			403,
			'task_assignment_forbidden'
		);
	}
	return new RecordRefusedTheWrite(
		'only the owner of this task, somebody taking part in it, or an admin can change it',
		403,
		'task_write_forbidden'
	);
}

// PostgREST caps an unbounded select and says nothing about having done it, so
// a company past the cap would quietly lose tasks. Rows come a page at a time
// ordered by id, because ordering by anything that repeats drops rows at a page
// boundary; the answer is ordered by recency once every page is in.
const rowsPerPage = 500;

export async function tasksOfCompany(caller: SupabaseClient, areEvents: boolean): Promise<TaskRow[]> {
	const rows: TaskRow[] = [];
	for (let from = 0; ; from += rowsPerPage) {
		const { data, error } = await caller
			.from('task')
			.select(taskSelection)
			.eq('is_event', areEvents)
			.order('id')
			.range(from, from + rowsPerPage - 1)
			.returns<TaskRow[]>();
		if (error) throw new Error(error.message);
		const page = data ?? [];
		rows.push(...page);
		if (page.length < rowsPerPage) break;
	}
	return rows.sort((left, right) => right.updated_at.localeCompare(left.updated_at));
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
		parentTaskID?: string | null;
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
		target_is_event: row?.is_event ?? false,
		target_location: row?.location ?? null,
		target_is_whole_day: row?.is_whole_day ?? false,
		target_notify_minutes_before: row?.notify_minutes_before ?? null,
		target_participant_ids: written.participantIDs ?? row?.task_participant.map((one) => one.member_id) ?? [],
		...(row ? { target_expected_updated_at: row.updated_at } : { target_parent_task_id: written.parentTaskID ?? null })
	};
}

export async function saveTask(
	caller: SupabaseClient,
	argumentsOfSave: Record<string, unknown>
): Promise<string> {
	const { data, error } = await caller.rpc('task_save', argumentsOfSave);
	if (error) {
		throw refusedWriteOf(error.message, error.code, argumentsOfSave.target_participant_ids !== undefined);
	}
	if (typeof data !== 'string') throw new RecordRefusedTheWrite('the record named no task it saved', 502);
	return data;
}

// PostgREST answers a delete row level security forbids with an empty list and
// no error, so the representation is the only proof anything went.
export async function deleteTask(caller: SupabaseClient, taskID: string): Promise<void> {
	const { data, error } = await caller.from('task').delete().eq('id', taskID).select('id').returns<{ id: string }[]>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	const deleted = data ?? [];
	if (deleted.length === 1 && deleted[0].id === taskID) return;
	if (deleted.length > 0) {
		throw new RecordRefusedTheWrite('the record deleted something other than what was asked for', 502);
	}
	throw await refusalForNothingDeleted(caller, taskID);
}

// PostgREST answers an empty list whether the row is gone or row level security
// refused it, so telling those apart takes a second read.
async function refusalForNothingDeleted(
	caller: SupabaseClient,
	taskID: string
): Promise<RecordRefusedTheWrite> {
	const { data } = await caller.from('task').select('id').eq('id', taskID).returns<{ id: string }[]>();
	if ((data ?? []).length > 0) {
		return new RecordRefusedTheWrite(
			'only the owner of this task, somebody taking part in it, or an admin can delete it',
			403,
			'task_write_forbidden'
		);
	}
	return new RecordRefusedTheWrite(
		'this task is gone; it was deleted after the list that named it',
		404,
		'task_not_found'
	);
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
