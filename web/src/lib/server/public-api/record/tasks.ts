import type { SupabaseClient } from '@supabase/supabase-js';
import { centralStatusFromWord } from '$lib/task/central-task';
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

const insufficientPrivilege = '42501';
const serializationFailure = '40001';

export function statusOfPostgresCode(code: string | undefined): number {
	if (code === insufficientPrivilege) return 403;
	if (code === serializationFailure) return 409;
	return 422;
}

export class NothingMatchesTheHint extends Error {
	constructor(
		readonly hint: string,
		readonly candidates: string[]
	) {
		super(
			candidates.length === 0
				? `nothing here goes by ${hint}`
				: `${hint} could be ${candidates.join(' / ')}; name one of them exactly`
		);
		this.name = 'NothingMatchesTheHint';
	}
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

export function taskOfHint(tasks: TaskRow[], hint: string): TaskRow {
	const asked = hint.trim();
	if (!asked) throw new NothingMatchesTheHint(hint, []);

	const byID = tasks.find((task) => task.id === asked);
	if (byID) return byID;

	const titled = tasks.filter((task) => task.title.trim() === asked);
	if (titled.length === 1) return titled[0];

	const contained = titled.length > 0 ? titled : tasks.filter((task) => task.title.includes(asked));
	if (contained.length === 1) return contained[0];
	throw new NothingMatchesTheHint(hint, contained.map((task) => task.title));
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
	const named = peopleOfHints(people, hints).map((person) => person.personID);
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
