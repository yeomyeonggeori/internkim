import { compatibilityOwnerOf, taskStatus } from '$lib/task/central-task';
import { personName } from '$lib/person-name';
import { taskWeekCodeForDateISO } from '$lib/task/task-week-code';
import {
	WorkspaceTaskSize,
	WorkspaceTaskStatus,
	type TaskBoardInput,
	type TaskLabelVocabulary,
	type TaskListInput,
	type TaskVocabularySetInput
} from '../catalog/tools';
import {
	crmColumnsWritten,
	opportunityOfCRMHint,
	organizationOfCRMHint,
	writeCRMColumns
} from './crm-tools';
import { dayOfInstant, dayShifted, endWritten, instantOfDay, isTheSameMoment, startWritten, weekWindow } from './days';
import { labelOf } from './labels';
import { displayNameOf, mentionOf, personOfHint, type RecordPerson } from './people';
import type { RecordContext } from './company';
import { missesATaskLabel, withDecidedTaskLabels } from './task-labels';
import { deleteTask, participantsOfHints, saveTask, saveTaskCompletingChildren, saveTaskWithChildren, savedTaskByID, taskOfHint, taskFromHint, tasksOfCompany, taskWriteArguments, type TaskRow } from './tasks';
import { ownerNamedBy, whoseRecords } from './whose';
import { labelsOfVocabulary, type CompanyLabels } from './labels';
import { taskPage, type TaskSearch } from './task-page';
import { taskChangeOf, type TaskChangeAnnouncer } from './task-change';
import { lastDayBetween, taskBoardDatePredicate, taskBoardWindow, taskChildProgressForBoard, type TaskBoardWindow, type TaskCardRow } from './task-board';
import { Refusal, InputRefused, RecordAlreadyHolds, ServiceUnanswered, refusalOfPostgresError, type PostgresError } from '../refusal';

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
	parentTaskHint?: string;
	isEvent?: boolean;
	isWholeDay?: boolean;
	location?: string;
	notifyMinutesBefore?: number;
};

export type AnsweredPerson = {
	personID: string;
	displayName?: string;
	email?: string;
	mention?: string;
};

export type AnsweredTask = {
	taskID: string;
	parentTaskID: string;
	organizationID?: string;
	opportunityID?: string;
	requesterID: string;
	requesterName: string;
	createdAt: string;
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
	canDelete: boolean;
	canUpdate: boolean;
};

function presentationOf(personID: string, person: RecordPerson | undefined, locale: RecordContext['locale']): AnsweredPerson {
	if (!person) return { personID };
	const displayName = personName(person.name, locale);
	const mention = mentionOf(displayName);
	return {
		personID,
		displayName,
		...(person.email ? { email: person.email } : {}),
		...(mention ? { mention } : {})
	};
}

function participantsOf(context: RecordContext, row: TaskCardRow, personOf = peopleByID(context)): AnsweredPerson[] {
	return row.task_participant.map(({ member_id }) => presentationOf(member_id, personOf.get(member_id), context.locale));
}

function peopleByID(context: RecordContext): Map<string, RecordPerson> {
	return new Map(context.people.map((person) => [person.personID, person]));
}

function ownerOf(context: RecordContext, row: TaskCardRow, personOf = peopleByID(context)): { id: string; name: string } {
	return compatibilityOwnerOf(
		row.task_participant.map(({ member_id }) => ({
			id: member_id,
			name: personName(personOf.get(member_id)?.name ?? '', context.locale)
		}))
	);
}

function answeredTask(context: RecordContext, row: TaskCardRow, people = peopleByID(context)): AnsweredTask {
	const participants = participantsOf(context, row, people);
	const owner = ownerOf(context, row, people);
	const endDate = row.task_last_day ?? '';
	const requesterID = row.requester_id ?? '';
	return {
		taskID: row.id,
		parentTaskID: row.parent_task_id ?? '',
		...(row.organization_id !== undefined ? { organizationID: row.organization_id ?? '' } : {}),
		...(row.opportunity_id !== undefined ? { opportunityID: row.opportunity_id ?? '' } : {}),
		requesterID,
		requesterName: personName(people.get(requesterID)?.name ?? '', context.locale),
		createdAt: row.created_at,
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
		weekCode: endDate ? taskWeekCodeForDateISO(endDate) : '',
		canDelete: row.task_deletable,
		canUpdate: row.task_updatable
	};
}

type WrittenTaskFields = ReturnType<typeof writtenFields>;

function instantOrNothing(written: string | undefined, instantOf: (written: string) => string): string | null | undefined {
	if (written === undefined) return undefined;
	return written.trim() ? instantOf(written) : null;
}

function locationWritten(written: string | undefined): { name: string } | null | undefined {
	if (written === undefined) return undefined;
	return written.trim() ? { name: written.trim() } : null;
}

function reminderWritten(written: number | undefined): number | null | undefined {
	if (written === undefined) return undefined;
	return written > 0 ? written : null;
}

export async function taskRowOfHint(context: RecordContext, hint: string): Promise<TaskRow> {
	return taskFromHint(context.caller, hint, false, context.requesterID);
}

async function taskByID(context: RecordContext, taskID: string): Promise<TaskRow> {
	return savedTaskByID(context.caller, taskID, false);
}

function writtenFields(context: RecordContext, written: TaskWritten, row: TaskRow | null) {
	const timezone = context.labels.timezone;
	const isWholeDay = written.isWholeDay ?? row?.is_whole_day ?? false;
	return {
		title: written.title,
		status: written.status,
		size: written.size,
		note: written.note,
		business: labelOf(context.labels.businesses, written.business, row ? row.business : null),
		type: labelOf(context.labels.types, written.type, row ? row.type : null),
		startsAt: instantOrNothing(written.startsAt, (edge) => startWritten(timezone, edge, isWholeDay)),
		endsAt: instantOrNothing(written.endsAt, (edge) => endWritten(timezone, edge, isWholeDay)),
		isEvent: written.isEvent,
		isWholeDay: written.isWholeDay,
		location: locationWritten(written.location),
		notifyMinutesBefore: reminderWritten(written.notifyMinutesBefore),
		participantIDs: participantsOfHints(
			context.people,
			written.participantPersonHints,
			context.requesterID
		)
	};
}

type TaskAddInput = TaskWritten & { childTaskHints?: string[] };

export async function taskAdd(
	context: RecordContext,
	asked: TaskAddInput,
	announceTaskChange?: TaskChangeAnnouncer
): Promise<AnsweredTask> {
	if (!asked.title?.trim()) throw new InputRefused('a task needs a title');
	const input = missesATaskLabel(asked)
		? withDecidedTaskLabels(asked, await context.decideTaskLabels({ title: asked.title, note: asked.note ?? '' }))
		: asked;
	const written = writtenFields(context, input, null);
	const participantIDs = written.participantIDs ?? [context.requesterID];
	const status = statusOfAssignedWork(written.status, participantIDs, context.requesterID);

	const tasks = await tasksOfCompany(context.caller, false);
	const parentTaskID = input.parentTaskHint?.trim() ? taskOfHint(tasks, input.parentTaskHint, 'task').id : null;
	const duplicate = duplicateJustAdded(context, tasks, participantIDs, asked.title.trim());
	const links = await crmColumnsWritten(context, input, duplicate ?? null);
	const childTaskIDs = (input.childTaskHints ?? []).map((hint) => taskOfHint(tasks, hint, 'task').id);
	const saved = duplicate
		? (await mergedIntoDuplicate(context, duplicate, input)).id
		: await saveNewTask(context, taskWriteArguments(null, { ...written, status, participantIDs, parentTaskID, links }), childTaskIDs);

	if (duplicate) await writeCRMColumns(context, saved, links);
	if (duplicate && childTaskIDs.length > 0) await linkTaskChildren(context, duplicate, childTaskIDs);
	const added = await taskByID(context, saved);
	await announceTaskChange?.(taskChangeOf(duplicate ?? null, added, context.requesterID));
	return answeredTask(context, added);
}

function saveNewTask(context: RecordContext, argumentsOfSave: Record<string, unknown>, childTaskIDs: string[]): Promise<string> {
	if (childTaskIDs.length === 0) return saveTask(context.caller, argumentsOfSave);
	return saveTaskWithChildren(context.caller, argumentsOfSave, childTaskIDs);
}

function statusOfAssignedWork(asked: string | undefined, participantIDs: string[], requesterID: string): string | undefined {
	if (asked !== undefined || participantIDs.includes(requesterID)) return asked;
	return taskStatus.requested;
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

type TaskUpdateInput = TaskWritten & { taskHint?: string; childTaskHints?: string[]; completeChildren?: boolean };

// Everything else the caller can name is a column task_save writes. These three
// are not, so an update naming only these has no column to write.
const fieldsThatAreNotColumns = new Set(['taskHint', 'parentTaskHint', 'childTaskHints']);

function namesAColumn(input: TaskUpdateInput): boolean {
	return Object.keys(input).some((field) => !fieldsThatAreNotColumns.has(field));
}

export async function taskUpdate(
	context: RecordContext,
	input: TaskUpdateInput,
	announceTaskChange?: TaskChangeAnnouncer
): Promise<AnsweredTask> {
	if (!input.taskHint) throw new InputRefused('an update names the task it changes');
	const row = await taskRowOfHint(context, input.taskHint);

	if (input.parentTaskHint !== undefined) await writeTaskParent(context, row, input.parentTaskHint);
	if (input.childTaskHints !== undefined) await linkTaskChildren(context, row, input.childTaskHints);
	if (!namesAColumn(input)) return answeredTask(context, await taskByID(context, row.id));

	const written = writtenFields(context, input, row);
	const save = input.completeChildren ? saveTaskCompletingChildren : saveTask;
	const saved = await save(context.caller, taskWriteArguments(row, written));
	await writeCRMColumns(context, saved, await crmColumnsWritten(context, input, row));
	const patched = await taskByID(context, saved);
	refuseUnlessPatched(written, patched);
	await announceTaskChange?.(taskChangeOf(row, patched, context.requesterID));
	return answeredTask(context, patched);
}

async function writeTaskParent(context: RecordContext, row: TaskRow, hint: string): Promise<void> {
	const named = hint.trim();
	const parent = named ? (await taskFromHint(context.caller, named, false)).id : null;
	if (parent === row.id) throw new InputRefused('a task cannot be its own parent');
	const { error } = await context.caller.rpc('task_parent_set', {
		target_task_id: row.id,
		target_parent_task_id: parent
	});
	if (error) throw refusalOfPostgresError(error);
}

async function linkTaskChildren(context: RecordContext, row: TaskRow, hints: string[]): Promise<void> {
	if (hints.length === 0) return;
	const children = await Promise.all(hints.map((hint) => taskFromHint(context.caller, hint, false)));
	const childIDs = children.map((child) => child.id);
	const { error } = await context.caller.rpc('task_children_link', {
		parent_id: row.id,
		child_ids: childIDs
	});
	if (error) throw refusalOfPostgresError(error);
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
	throw new ServiceUnanswered(`the record saved this task without ${unwritten.join(', ')} as asked`, 'task_result_invalid');
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
	if (!input.taskHint) throw new InputRefused('a deletion names the task it removes');
	const row = await taskRowOfHint(context, input.taskHint);
	await deleteTask(context.caller, row.id);
	return { taskID: row.id, deleted: true };
}

const rollsForwardToTheWeekRead = ['planned', 'paused'];
const staysInTheWeekItEnded = ['completed', 'rejected', 'stopped'];

function weekDatesPredicate(context: RecordContext, weekFrom: number, weekTo: number): string {
	const timeZone = context.labels.timezone;
	const asked = weekWindow(timeZone, context.now, weekFrom, weekTo);
	const thisWeek = weekWindow(timeZone, context.now, 0, 0);
	const nextMonday = instantOfDay(timeZone, dayShifted(thisWeek.to, 1));
	const isThisWeekAsked = asked.from <= thisWeek.from && thisWeek.from <= asked.to;
	const endsInTheWeeksAsked = lastDayBetween(timeZone, asked.from, asked.to);
	const startsInTheWeeksAsked = `starts_at.gte.${instantOfDay(timeZone, asked.from)},starts_at.lt.${instantOfDay(timeZone, dayShifted(asked.to, 1))}`;
	return [
		...(isThisWeekAsked ? ['status.eq.paused', `and(status.eq.planned,or(starts_at.is.null,starts_at.lt.${nextMonday}))`] : []),
		`and(status.eq.planned,starts_at.gte.${nextMonday},${startsInTheWeeksAsked})`,
		`and(status.in.(${staysInTheWeekItEnded.join(',')}),or(and(${endsInTheWeeksAsked}),and(ends_at.is.null,${startsInTheWeeksAsked})))`,
		`and(status.not.in.(${[...rollsForwardToTheWeekRead, ...staysInTheWeekItEnded].join(',')}),${endsInTheWeeksAsked})`
	].join(',');
}

function searchableText(value: string): string {
	return value.trim().toLowerCase().split(/\s+/).join('');
}

function taskSearchOf(context: RecordContext, query: string | undefined): TaskSearch | undefined {
	const text = searchableText(query ?? '');
	if (!text) return undefined;
	const soleParticipantIDs = context.people
		.filter((person) => searchableText(personName(person.name, context.locale)).includes(text))
		.map((person) => person.personID);
	return { text, soleParticipantIDs };
}

export async function taskList(context: RecordContext, input: TaskListInput) {
	return readTaskList(context, input, undefined);
}

export async function taskBoard(context: RecordContext, input: TaskBoardInput) {
	const { boardWeek, ...filters } = input;
	return readTaskList(context, filters, taskBoardWindow(boardWeek, context.labels.timezone, context.now));
}

async function readTaskList(context: RecordContext, input: TaskListInput, board: TaskBoardWindow | undefined) {
	const whose = whoseRecords(context.people, input.personHints, input.scope, context.requesterID);
	const deal = input.opportunityHint ? await opportunityOfCRMHint(context, input.opportunityHint) : null;
	const organization = input.organizationHint
		? await organizationOfCRMHint(context, input.organizationHint)
		: null;
	const everyWeek =
		input.everyWeek === true ||
		(Boolean(deal || organization) && input.weekFrom === undefined && input.weekTo === undefined);
	const weekFrom = input.weekFrom ?? 0;
	const weekTo = input.weekTo ?? input.weekFrom ?? 0;
	const page = await taskPage(context.caller, {
		whose,
		status: input.status,
		organizationID: organization?.id,
		opportunityID: deal?.id,
		dates: board ? taskBoardDatePredicate(board) : everyWeek ? undefined : weekDatesPredicate(context, weekFrom, weekTo),
		search: taskSearchOf(context, input.query),
		carriesLinks: !board
	}, input, board ? 'task_board_get' : 'task_list');
	const people = peopleByID(context);
	return {
		...(board ? {
			boardWeek: board.week,
			childProgress: Object.entries(await taskChildProgressForBoard(context.caller, page.items.map(row => row.id)))
				.map(([parentTaskID, progress]) => ({ parentTaskID, ...progress }))
		} : {}),
		scope: whose.everyone ? 'everyone' : 'person',
		ownerID: ownerNamedBy(whose),
		...(!board ? { weekFrom, weekTo } : {}),
		statusFilter: input.status ?? '',
		count: page.items.length,
		totalCount: page.totalCount,
		unfinishedCount: page.unfinishedCount,
		tasks: page.items.map((row) => answeredTask(context, row, people)),
		registeredLabels: registeredLabelsOf(context.labels),
		...(page.nextCursor ? { nextCursor: page.nextCursor } : {})
	};
}

function registeredLabelsOf(labels: CompanyLabels): TaskLabelVocabulary {
	return {
		businesses: labels.businesses,
		types: labels.types,
		sizes: Object.values(WorkspaceTaskSize),
		statuses: Object.values(WorkspaceTaskStatus),
		...(labels.etcBusinessColor ? { etcBusinessColor: labels.etcBusinessColor } : {}),
		...(labels.etcTypeColor ? { etcTypeColor: labels.etcTypeColor } : {})
	};
}

const dependentObjectsStillExist = '2BP01';

export async function taskVocabularySet(
	context: RecordContext,
	input: TaskVocabularySetInput
): Promise<TaskLabelVocabulary> {
	const { error } = await context.caller.rpc('task_vocabulary_save', {
		target_vocabulary: vocabularyWritten(context.labels, input)
	});
	if (error) throw refusedVocabularyWrite(error);
	return registeredLabelsOf(await labelsAfterTheWrite(context));
}

function vocabularyWritten(current: CompanyLabels, input: TaskVocabularySetInput): TaskVocabularySetInput {
	const etcBusinessColor = input.etcBusinessColor ?? current.etcBusinessColor;
	const etcTypeColor = input.etcTypeColor ?? current.etcTypeColor;
	return {
		businesses: input.businesses ?? current.businesses,
		types: input.types ?? current.types,
		...(etcBusinessColor ? { etcBusinessColor } : {}),
		...(etcTypeColor ? { etcTypeColor } : {})
	};
}

async function labelsAfterTheWrite(context: RecordContext): Promise<CompanyLabels> {
	const company = await context.caller
		.from('company')
		.select('task_vocabulary')
		.limit(1)
		.single<{ task_vocabulary: unknown }>();
	if (company.error) throw refusalOfPostgresError(company.error);
	return labelsOfVocabulary(company.data.task_vocabulary, context.labels.timezone);
}

function refusedVocabularyWrite(error: PostgresError): Refusal {
	if (error.code === dependentObjectsStillExist) return new RecordAlreadyHolds(error.message, 'task_label_in_use');
	return refusalOfPostgresError(error);
}

export { displayNameOf, personOfHint };
