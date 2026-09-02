import { compatibilityOwnerOf } from '$lib/task/central-task';
import { taskWeekCodeForDateISO } from '$lib/task/task-week-code';
import { WorkspaceTaskSize, WorkspaceTaskStatus } from '../catalog/tools';
import { dayOfInstant, instantWritten, weekWindow, windowHoldsDay } from './days';
import { firstRegistered, labelOf } from './labels';
import { displayNameOf, mentionOf, personOfHint, type RecordPerson } from './people';
import type { RecordContext } from './company';
import {
	deleteTask,
	ownerOfScope,
	participantsOfHints,
	saveTask,
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
};

export type AnsweredPerson = {
	personID: string;
	displayName?: string;
	email?: string;
	mention?: string;
};

export type AnsweredTask = {
	taskID: string;
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

function answeredTask(context: RecordContext, row: TaskRow): AnsweredTask {
	const participants = participantsOf(context, row);
	const owner = compatibilityOwnerOf(
		participants.map((participant) => ({ id: participant.personID, name: participant.displayName ?? '' }))
	);
	const endDate = dayOfInstant(context.labels.timezone, row.ends_at);
	return {
		taskID: row.id,
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

async function taskByID(context: RecordContext, taskID: string): Promise<TaskRow> {
	const tasks = await tasksOfCompany(context.caller, false);
	return taskOfHint(tasks, taskID);
}

function writtenFields(context: RecordContext, written: TaskWritten, row: TaskRow | null) {
	return {
		title: written.title,
		status: written.status,
		size: written.size,
		business: labelOf(
			context.labels.businesses,
			written.business,
			row ? row.business : firstRegistered(context.labels.businesses)
		),
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
	const saved = await saveTask(context.caller, {
		...taskWriteArguments(null, {
			...written,
			participantIDs: written.participantIDs ?? [context.requesterID]
		})
	});
	return answeredTask(context, await taskByID(context, saved));
}

export async function taskUpdate(
	context: RecordContext,
	input: TaskWritten & { taskHint?: string }
): Promise<AnsweredTask> {
	if (!input.taskHint) throw new Error('an update names the task it changes');
	const tasks = await tasksOfCompany(context.caller, false);
	const row = taskOfHint(tasks, input.taskHint);
	const saved = await saveTask(context.caller, taskWriteArguments(row, writtenFields(context, input, row)));
	return answeredTask(context, await taskByID(context, saved));
}

export async function taskDelete(
	context: RecordContext,
	input: { taskHint?: string }
): Promise<{ taskID: string; deleted: true }> {
	if (!input.taskHint) throw new Error('a deletion names the task it removes');
	const tasks = await tasksOfCompany(context.caller, false);
	const row = taskOfHint(tasks, input.taskHint);
	await deleteTask(context.caller, row.id);
	return { taskID: row.id, deleted: true };
}

export type TaskListInput = {
	query?: string;
	participantPersonHint?: string;
	scope?: string;
	weekFrom?: number;
	weekTo?: number;
	status?: string;
	limit?: number;
};

export async function taskList(context: RecordContext, input: TaskListInput) {
	const ownerID = ownerOfScope(context.people, input.scope, input.participantPersonHint, context.requesterID);
	const window =
		input.weekFrom === undefined && input.weekTo === undefined
			? null
			: weekWindow(context.labels.timezone, context.now, input.weekFrom ?? 0, input.weekTo ?? input.weekFrom ?? 0);

	const rows = (await tasksOfCompany(context.caller, false)).filter((row) => {
		if (ownerID && !row.task_participant.some(({ member_id }) => member_id === ownerID)) return false;
		if (input.status && row.status !== input.status) return false;
		if (input.query && !row.title.includes(input.query)) return false;
		if (!window) return true;
		const day =
			dayOfInstant(context.labels.timezone, row.starts_at) ||
			dayOfInstant(context.labels.timezone, row.ends_at);
		return day !== '' && windowHoldsDay(window, day);
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

function listedPerson(person: RecordPerson) {
	const mention = mentionOf(person.name);
	return {
		personID: person.personID,
		name: person.name,
		email: person.email,
		...(mention ? { mention } : {})
	};
}

export function personList(context: RecordContext) {
	return { count: context.people.length, people: context.people.map(listedPerson) };
}

export { displayNameOf, personOfHint };
