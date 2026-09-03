import { compatibilityOwnerOf } from '$lib/task/central-task';
import { taskWeekCodeForDateISO } from '$lib/task/task-week-code';
import { WorkspaceTaskSize, WorkspaceTaskStatus } from '../catalog/tools';
import {
	crmColumnsWritten,
	opportunityOfCRMHint,
	organizationOfCRMHint,
	writeCRMColumns
} from './crm-tools';
import { dayOfInstant, instantWritten, isTheSameMoment, weekWindow } from './days';
import { labelOf } from './labels';
import { displayNameOf, mentionOf, personOfHint, type RecordPerson } from './people';
import type { RecordContext } from './company';
import {
	deleteTask,
	ownerOfScope,
	participantsOfHints,
	RecordRefusedTheWrite,
	rowOfSavedID,
	saveTask,
	statusOfPostgresCode,
	taskOfHint,
	tasksOfCompany,
	taskWriteArguments,
	type TaskRow
} from './tasks';

type TaskWritten = {
	title?: string;
	status?: string;
	size?: string;
	business?: string;
	type?: string;
	startsAt?: string;
	endsAt?: string;
	participantPersonHints?: string[];
	note?: string;
	organizationHint?: string;
	opportunityHint?: string;
	contactHint?: string;
	dueAt?: string;
};

export type AnsweredPerson = {
	personID: string;
	displayName?: string;
	email?: string;
	mention?: string;
};

export type AnsweredTask = {
	taskID: string;
	organizationID: string;
	opportunityID: string;
	content: string;
	ownerID: string;
	ownerName: string;
	participantIDs: string[];
	participantNames: string[];
	participantPresentations: AnsweredPerson[];
	business: string;
	type: string;
	size: string;
	status: string;
	startDate: string;
	endDate: string;
	weekCode: string;
};

function presentationOf(personID: string, person: RecordPerson | undefined): AnsweredPerson {
	if (!person) return { personID };
	const mention = mentionOf(person.name);
	return {
		personID,
		displayName: person.name,
		...(person.email ? { email: person.email } : {}),
		...(mention ? { mention } : {})
	};
}

function participantsOf(context: RecordContext, row: TaskRow): AnsweredPerson[] {
	const personOf = new Map(context.people.map((person) => [person.personID, person]));
	return row.task_participant.map(({ member_id }) => presentationOf(member_id, personOf.get(member_id)));
}

function ownerOf(context: RecordContext, row: TaskRow): { id: string; name: string } {
	const personOf = new Map(context.people.map((person) => [person.personID, person]));
	return compatibilityOwnerOf(
		row.task_participant.map(({ member_id }) => ({
			id: member_id,
			name: personOf.get(member_id)?.name ?? ''
		}))
	);
}

function answeredTask(context: RecordContext, row: TaskRow): AnsweredTask {
	const participants = participantsOf(context, row);
	const owner = ownerOf(context, row);
	const endDate = dayOfInstant(context.labels.timezone, row.ends_at);
	return {
		taskID: row.id,
		organizationID: row.organization_id ?? '',
		opportunityID: row.opportunity_id ?? '',
		content: row.title,
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map((participant) => participant.personID),
		participantNames: participants.map((participant) => participant.displayName ?? ''),
		participantPresentations: participants,
		business: row.business ?? '',
		type: row.type ?? '',
		size: row.size ?? '',
		status: row.status,
		startDate: dayOfInstant(context.labels.timezone, row.starts_at),
		endDate,
		weekCode: endDate ? taskWeekCodeForDateISO(endDate) : ''
	};
}

type WrittenTaskFields = ReturnType<typeof writtenFields>;

export async function taskRowOfHint(context: RecordContext, hint: string): Promise<TaskRow> {
	const tasks = await tasksOfCompany(context.caller, false);
	return taskOfHint(tasks, hint, 'task', context.requesterID);
}

async function taskByID(context: RecordContext, taskID: string): Promise<TaskRow> {
	return rowOfSavedID(await tasksOfCompany(context.caller, false), taskID, 'task');
}

function writtenFields(context: RecordContext, written: TaskWritten, row: TaskRow | null) {
	return {
		title: written.title,
		status: written.status,
		size: written.size,
		note: written.note,
		business: labelOf(context.labels.businesses, written.business, row ? row.business : null),
		type: labelOf(context.labels.types, written.type, row ? row.type : null),
		startsAt:
			written.startsAt === undefined ? undefined : instantWritten(context.labels.timezone, written.startsAt),
		endsAt:
			written.endsAt === undefined
				? undefined
				: instantWritten(context.labels.timezone, written.endsAt, true),
		participantIDs: participantsOfHints(
			context.people,
			written.participantPersonHints,
			context.requesterID
		)
	};
}

export async function taskAdd(context: RecordContext, input: TaskWritten): Promise<AnsweredTask> {
	if (!input.title?.trim()) throw new Error('a task needs a title');
	const written = writtenFields(context, input, null);
	const participantIDs = written.participantIDs ?? [context.requesterID];

	const tasks = await tasksOfCompany(context.caller, false);
	const duplicate = duplicateJustAdded(context, tasks, participantIDs, input.title.trim());
	const saved = duplicate
		? (await mergedIntoDuplicate(context, duplicate, input)).id
		: await saveTask(context.caller, taskWriteArguments(null, { ...written, participantIDs }));

	await writeCRMColumns(context, saved, await crmColumnsWritten(context, input, duplicate ?? null));
	return answeredTask(context, await taskByID(context, saved));
}

const duplicateWindowMilliseconds = 10 * 60 * 1000;

function duplicateJustAdded(
	context: RecordContext,
	tasks: TaskRow[],
	participantIDs: string[],
	title: string
): TaskRow | undefined {
	const owner = compatibilityOwnerOf(participantIDs.map((id) => ({ id, name: '' }))).id;
	if (!owner) return undefined;
	return tasks.find(
		(row) =>
			row.title === title &&
			ownerOf(context, row).id === owner &&
			addedWithinTheDuplicateWindow(row.created_at, context.now)
	);
}

export function addedWithinTheDuplicateWindow(createdAt: string, now: Date): boolean {
	const since = now.getTime() - new Date(createdAt).getTime();
	return Math.abs(since) <= duplicateWindowMilliseconds;
}

async function mergedIntoDuplicate(
	context: RecordContext,
	duplicate: TaskRow,
	input: TaskWritten
): Promise<TaskRow> {
	const written = writtenFields(context, input, duplicate);
	if (!carriesSomethingNew(written, duplicate)) return duplicate;
	const saved = await saveTask(
		context.caller,
		taskWriteArguments(duplicate, { ...written, participantIDs: undefined })
	);
	return taskByID(context, saved);
}

function carriesSomethingNew(written: WrittenTaskFields, row: TaskRow): boolean {
	return (
		isNewValue(written.status, row.status) ||
		isNewValue(written.size, row.size) ||
		isNewValue(written.business, row.business) ||
		isNewValue(written.type, row.type) ||
		isNewMoment(written.startsAt, row.starts_at) ||
		isNewMoment(written.endsAt, row.ends_at)
	);
}

function isNewValue(written: string | null | undefined, held: string | null): boolean {
	return written !== undefined && written !== null && written !== held;
}

function isNewMoment(written: string | null | undefined, held: string | null): boolean {
	if (written === undefined || written === null) return false;
	return held === null || !isTheSameMoment(held, written);
}

export async function taskUpdate(
	context: RecordContext,
	input: TaskWritten & { taskHint?: string }
): Promise<AnsweredTask> {
	if (!input.taskHint) throw new Error('an update names the task it changes');
	const row = await taskRowOfHint(context, input.taskHint);
	const written = writtenFields(context, input, row);
	const saved = await saveTask(context.caller, taskWriteArguments(row, written));
	await writeCRMColumns(context, saved, await crmColumnsWritten(context, input, row));
	const patched = await taskByID(context, saved);
	refuseUnlessPatched(written, patched);
	return answeredTask(context, patched);
}

// supabase/migrations/20260831000011_dates_drive_the_status_and_guard_it.sql
// derives status from the days a task spans and pulls a completed task's end
// back to today, so status, starts_at and ends_at are the record's to settle.
function refuseUnlessPatched(written: WrittenTaskFields, row: TaskRow): void {
	const unwritten = [
		unwrittenField('title', written.title, row.title),
		unwrittenField('size', written.size, row.size),
		unwrittenField('business', written.business, row.business),
		unwrittenField('type', written.type, row.type),
		unwrittenParticipants(written.participantIDs, row)
	].filter((field): field is string => field !== null);
	if (unwritten.length === 0) return;
	throw new RecordRefusedTheWrite(
		`the record saved this task without ${unwritten.join(', ')} as asked`,
		502,
		'task_result_invalid'
	);
}

function unwrittenField(name: string, written: string | null | undefined, held: string | null): string | null {
	if (written === undefined) return null;
	return written === held ? null : name;
}


function unwrittenParticipants(written: string[] | undefined, row: TaskRow): string | null {
	if (written === undefined) return null;
	const held = new Set(row.task_participant.map(({ member_id }) => member_id));
	if (held.size !== written.length) return 'participantPersonHints';
	return written.every((personID) => held.has(personID)) ? null : 'participantPersonHints';
}

export async function taskDelete(
	context: RecordContext,
	input: { taskHint?: string }
): Promise<{ taskID: string; deleted: true }> {
	if (!input.taskHint) throw new Error('a deletion names the task it removes');
	const row = await taskRowOfHint(context, input.taskHint);
	await deleteTask(context.caller, row.id);
	return { taskID: row.id, deleted: true };
}

const rollsForwardToTheWeekRead = new Set(['planned', 'paused']);
const staysInTheWeekItEnded = new Set(['completed', 'rejected', 'stopped']);

function weekCodeOfOffset(context: RecordContext, offset: number): string {
	return taskWeekCodeForDateISO(weekWindow(context.labels.timezone, context.now, offset, offset).from);
}

function weeksAsked(context: RecordContext, weekFrom: number, weekTo: number): Set<string> {
	const [earlier, later] = weekFrom <= weekTo ? [weekFrom, weekTo] : [weekTo, weekFrom];
	const asked = new Set<string>();
	for (let offset = earlier; offset <= later; offset += 1) {
		asked.add(weekCodeOfOffset(context, offset));
	}
	return asked;
}

function weekCodeOfDay(context: RecordContext, instant: string | null): string {
	const day = dayOfInstant(context.labels.timezone, instant);
	return day ? taskWeekCodeForDateISO(day) : '';
}

function weekCodeOfTask(context: RecordContext, row: TaskRow, thisWeek: string): string {
	const startWeek = weekCodeOfDay(context, row.starts_at);
	const endWeek = weekCodeOfDay(context, row.ends_at);
	if (row.status === 'paused') return thisWeek;
	if (rollsForwardToTheWeekRead.has(row.status)) {
		if (!startWeek || isCurrentOrEarlier(startWeek, thisWeek)) return thisWeek;
		return startWeek;
	}
	if (staysInTheWeekItEnded.has(row.status)) return endWeek || startWeek;
	return endWeek;
}

function isCurrentOrEarlier(weekCode: string, thisWeek: string): boolean {
	const asked = weekOrdinalOf(weekCode);
	const current = weekOrdinalOf(thisWeek);
	if (asked === null || current === null) return false;
	return asked <= current;
}

function weekOrdinalOf(weekCode: string): number | null {
	const read = /^(\d{2})W(\d{1,2})$/.exec(weekCode.trim().toUpperCase());
	if (!read) return null;
	return Number(read[1]) * 100 + Number(read[2]);
}

function searchableText(value: string): string {
	return value.trim().toLowerCase().split(/\s+/).join('');
}

function matchesQuery(context: RecordContext, row: TaskRow, query: string | undefined): boolean {
	const asked = searchableText(query ?? '');
	if (!asked) return true;
	const searched = [row.title, row.business ?? '', row.type ?? '', ownerOf(context, row).name, row.status];
	return searched.some((value) => searchableText(value).includes(asked));
}

export type TaskListInput = {
	query?: string;
	participantPersonHint?: string;
	scope?: string;
	weekFrom?: number;
	weekTo?: number;
	status?: string;
	organizationHint?: string;
	opportunityHint?: string;
	limit?: number;
};

export async function taskList(context: RecordContext, input: TaskListInput) {
	const ownerID = ownerOfScope(context.people, input.scope, input.participantPersonHint, context.requesterID);
	const weeks = weeksAsked(context, input.weekFrom ?? 0, input.weekTo ?? input.weekFrom ?? 0);
	const thisWeek = weekCodeOfOffset(context, 0);
	const deal = input.opportunityHint ? await opportunityOfCRMHint(context, input.opportunityHint) : null;
	const organization = input.organizationHint
		? await organizationOfCRMHint(context, input.organizationHint)
		: null;
	const everyWeek = Boolean(deal || organization) && input.weekFrom === undefined && input.weekTo === undefined;

	const rows = (await tasksOfCompany(context.caller, false)).filter((row) => {
		if (ownerID && !row.task_participant.some(({ member_id }) => member_id === ownerID)) return false;
		if (input.status && row.status !== input.status) return false;
		if (deal && row.opportunity_id !== deal.id) return false;
		if (organization && row.organization_id !== organization.id) return false;
		if (!matchesQuery(context, row, input.query)) return false;
		return everyWeek || weeks.has(weekCodeOfTask(context, row, thisWeek));
	});

	const kept = input.limit && input.limit > 0 ? rows.slice(0, input.limit) : rows;
	return {
		scope: ownerID ? 'person' : 'everyone',
		ownerID: ownerID ?? '',
		weekFrom: input.weekFrom ?? 0,
		weekTo: input.weekTo ?? input.weekFrom ?? 0,
		statusFilter: input.status ?? '',
		count: kept.length,
		tasks: kept.map((row) => answeredTask(context, row)),
		registeredLabels: {
			businesses: context.labels.businesses,
			types: context.labels.types,
			sizes: Object.values(WorkspaceTaskSize),
			statuses: Object.values(WorkspaceTaskStatus)
		}
	};
}

export { displayNameOf, personOfHint };
