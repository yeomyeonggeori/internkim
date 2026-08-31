import { dayOfInstant, instantWritten, weekWindow, windowHoldsDay } from './days';
import { firstRegistered, labelOf } from './labels';
import { displayNameOf, personOfHint } from './people';
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

export type AnsweredTask = {
	taskID: string;
	title: string;
	status: string;
	size: string;
	business: string | null;
	type: string | null;
	startDate: string;
	endDate: string;
	participants: string[];
};

function answeredTask(context: RecordContext, row: TaskRow): AnsweredTask {
	const nameOf = new Map(context.people.map((person) => [person.personID, person.name]));
	return {
		taskID: row.id,
		title: row.title,
		status: row.status,
		size: row.size ?? '',
		business: row.business,
		type: row.type,
		startDate: dayOfInstant(context.labels.timezone, row.starts_at),
		endDate: dayOfInstant(context.labels.timezone, row.ends_at),
		participants: row.task_participant.map(({ member_id }) => nameOf.get(member_id) ?? member_id)
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
): Promise<{ taskID: string; title: string; deleted: true }> {
	if (!input.taskHint) throw new Error('a deletion names the task it removes');
	const tasks = await tasksOfCompany(context.caller, false);
	const row = taskOfHint(tasks, input.taskHint);
	await deleteTask(context.caller, row.id);
	return { taskID: row.id, title: row.title, deleted: true };
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
		personID: ownerID,
		personName: ownerID ? context.people.find((one) => one.personID === ownerID)?.name ?? '' : '',
		weekFrom: input.weekFrom ?? null,
		weekTo: input.weekTo ?? null,
		statusFilter: input.status ?? null,
		count: kept.length,
		tasks: kept.map((row) => answeredTask(context, row)),
		registeredLabels: { businesses: context.labels.businesses, types: context.labels.types }
	};
}

export function personList(context: RecordContext) {
	return {
		count: context.people.length,
		people: context.people.map((person) => ({
			personID: person.personID,
			name: person.name,
			email: person.email
		}))
	};
}

export { displayNameOf, personOfHint };
