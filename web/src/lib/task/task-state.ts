import { projectURL } from '$lib/supabase';
import { taskAccountScope } from './task-account-scope';
import { taskSnapshotGeneration } from '../../routes/task/task-snapshot-storage';
import { supabaseMember } from '$lib/supabase-session';
import { isRefusalCode } from '$lib/public-api-call';
import { companyDirectory, type RecordPerson } from '$lib/record/person-directory';
import { announceTaskMoved } from '$lib/task/announce-task';
import { centralStatusFromWord, centralStatusWord, centralTaskStatusOptions } from '$lib/task/central-task';
import {
	addTask,
	deleteTask,
	everyTaskOfTheCompany,
	linkTaskChildren,
	setTaskLabels,
	setTaskParent,
	updateTask,
	type RecordTask,
	type WrittenTask
} from '$lib/task/task-record';
import {
	currentScoresOf,
	memberScoreDetails,
	memberTaskTallies,
	startOfISOWeek,
	totalScoreOf,
	type MemberTaskTally
} from '$lib/task/task-scores';
import { taskDefinitionsOf, taskVocabularyOfDefinitions } from '$lib/task/task-vocabulary';
import { taskWeekForCode, taskWeekOfDate } from '$lib/task/task-week-code';
import type {
	TaskDefinitions,
	TaskMember,
	TaskMemberScoreDetail,
	TaskMetrics,
	TaskState,
	Task,
	TaskWeeklySummary
} from '../../routes/task/task-types';

export const taskStatusOptions = centralTaskStatusOptions;

const untitledTask = '(제목 없음)';

type ListedTasks = Awaited<ReturnType<typeof everyTaskOfTheCompany>>;
type TaskViewer = { memberID: string; email: string; name: string; isAdmin: boolean };
const beingRead = new Map<string, { scope: string; generation: number; listed: Promise<ListedTasks>; value: Promise<TaskState> }>();

export function forgetTaskStateRead(scope: string): void {
	for (const [key, reading] of beingRead) if (reading.scope === scope) beingRead.delete(key);
}

export function taskState(scope?: string, boardWeek?: string): Promise<TaskState> {
	if (scope === undefined) {
		return supabaseMember().then((member) => taskState(taskAccountScope(member, projectURL()), boardWeek));
	}
	return taskReadFor(scope, boardWeek).value;
}

export function taskBoardState(scope: string, viewer: TaskViewer, boardWeek?: string): Promise<TaskState> {
	return taskReadFor(scope, boardWeek).listed.then(listed => taskBoardStateOf(listed, viewer, boardWeek));
}

export function taskBoardStateOf(listed: ListedTasks, viewer: TaskViewer, boardWeek?: string): TaskState {
	return stateOfListedTasks(listed, [{
		personID: viewer.memberID, email: viewer.email, name: viewer.name, isAdmin: viewer.isAdmin
	}], viewer.memberID, boardWeek, viewer);
}

function taskReadFor(scope: string, boardWeek?: string) {
	const generation = taskSnapshotGeneration();
	const key = JSON.stringify([scope, boardWeek ?? 'full']);
	const current = beingRead.get(key);
	if (current?.generation === generation) return current;
	const listed = everyTaskOfTheCompany(boardWeek);
	const reading = { scope, generation, listed, value: readTaskState(listed, boardWeek) };
	beingRead.set(key, reading);
	void reading.value.catch(() => undefined).finally(() => {
		if (beingRead.get(key) === reading) beingRead.delete(key);
	});
	return reading;
}

async function readTaskState(listedTasks: Promise<ListedTasks>, boardWeek?: string): Promise<TaskState> {
	const [listed, directory] = await Promise.all([listedTasks, companyDirectory()]);
	return stateOfListedTasks(listed, directory.people, directory.requesterID, boardWeek);
}

function stateOfListedTasks(listed: ListedTasks, people: RecordPerson[], requesterID: string, boardWeek?: string, viewer?: TaskViewer): TaskState {
	const tasks = sortedByEnd(listed.tasks.map(taskOf));
	const memberIDs = people.map((person) => person.personID);
	const tallies = memberTaskTallies(tasks, memberIDs);
	const standingMetrics = boardWeek ? {} : standingMetricsOf(memberScoreDetails(tasks, memberIDs, startOfISOWeek(new Date())), tallies);
	const me = people.find((person) => person.personID === requesterID);

	return {
		completeness: boardWeek ? 'board' : 'full',
		peopleReady: viewer === undefined,
		...(boardWeek ? { boardWeek, childProgressByParent: Object.fromEntries((listed.childProgress ?? []).map(({ parentTaskID, ...progress }) => [parentTaskID, progress])) } : {}),
		currentWeek: taskWeekOfDate(new Date()),
		members: people.map((person) => memberOf(person, tallies[person.personID])),
		tasks,
		metrics: { ...metricsOf(tasks), ...standingMetrics },
		definitions: taskDefinitionsOf(listed.registeredLabels),
		statusOptions: taskStatusOptions,
		currentUserEmail: me?.email ?? viewer?.email ?? '',
		currentUserName: me?.name ?? viewer?.name ?? '',
		isAdmin: me?.isAdmin ?? viewer?.isAdmin ?? false
	};
}

export function taskWeeklySummaryOf(state: TaskState, week: string): TaskWeeklySummary {
	const shown = week ? taskWeekForCode(week, new Date()) : taskWeekOfDate(new Date());
	const weeklyTasks = state.tasks.filter((task) => task.weekCode === shown.code);
	return {
		week: shown,
		currentWeek: state.currentWeek,
		weeklyTasks,
		metrics: metricsOf(weeklyTasks)
	};
}

export function taskOf(record: RecordTask): Task {
	return {
		id: record.taskID,
		parentTaskID: record.parentTaskID || undefined,
		ownerID: record.ownerID ?? '',
		ownerName: record.ownerName ?? '',
		participantIDs: record.participantIDs ?? [],
		participantNames: record.participantNames ?? [],
		requesterID: record.requesterID || undefined,
		requesterName: record.requesterName || undefined,
		business: record.business || null,
		type: record.type || null,
		content: record.content ?? '',
		size: record.size ?? '',
		status: centralStatusWord(record.status ?? ''),
		startDate: record.startDate || undefined,
		endDate: record.endDate || undefined,
		createdAt: record.createdAt,
		weekCode: record.weekCode ?? ''
	};
}

export function writtenTaskOf(task: Task): WrittenTask {
	return {
		title: task.content || untitledTask,
		status: centralStatusFromWord(task.status),
		business: task.business ?? '',
		type: task.type ?? '',
		startsAt: task.startDate ?? '',
		endsAt: task.endDate ?? '',
		participantPersonHints: task.participantIDs,
		...(task.size ? { size: task.size } : {})
	};
}

function withoutUnchosenLabels(written: WrittenTask): WrittenTask {
	const { business, type, ...chosen } = written;
	return { ...chosen, ...(business ? { business } : {}), ...(type ? { type } : {}) };
}

export function changedTaskFields(task: Task, originalTask: Task): Partial<WrittenTask> {
	if (task.id !== originalTask.id) throw new Error('a task edit must match its original task');
	const original: Record<string, unknown> = writtenTaskOf(originalTask);
	return Object.fromEntries(Object.entries(writtenTaskOf(task)).filter(
		([key, value]) => JSON.stringify(value) !== JSON.stringify(original[key])
	));
}

export async function saveTask(task: Task, statusBefore: string | null, originalTask?: Task): Promise<void> {
	if (task.id) {
		const written = originalTask ? changedTaskFields(task, originalTask) : writtenTaskOf(task);
		if (Object.keys(written).length === 0) return;
		await updateTask(task.id, written);
	} else {
		const written = writtenTaskOf(task);
		await addTask({ ...withoutUnchosenLabels(written), ...(task.parentTaskID ? { parentTaskHint: task.parentTaskID } : {}) });
	}
	if (statusBefore !== null && statusBefore !== centralStatusFromWord(task.status)) {
		void announceTaskMoved(task.id);
	}
}

export async function moveTask(taskID: string, status: string): Promise<void> {
	await updateTask(taskID, { status: centralStatusFromWord(status) });
	void announceTaskMoved(taskID);
}

export async function removeTask(taskID: string): Promise<void> {
	await deleteTask(taskID);
}

export async function updateTaskParent(taskID: string, parentTaskID: string | null): Promise<void> {
	await setTaskParent(taskID, parentTaskID);
}

export async function updateTaskParents(taskIDs: string[], parentTaskID: string): Promise<void> {
	if (taskIDs.length === 0) return;
	await linkTaskChildren(parentTaskID, taskIDs);
}

export async function saveTaskVocabulary(
	definitions: Omit<TaskDefinitions, 'sizes'>,
	messages: { failure: string; inUse: string }
): Promise<void> {
	const vocabulary = taskVocabularyOfDefinitions(definitions);
	try {
		await setTaskLabels({
			businesses: vocabulary.businesses ?? [],
			types: vocabulary.types ?? [],
			...(vocabulary.etcBusinessColor ? { etcBusinessColor: vocabulary.etcBusinessColor } : {}),
			...(vocabulary.etcTypeColor ? { etcTypeColor: vocabulary.etcTypeColor } : {})
		});
	} catch (refusal) {
		if (isRefusalCode(refusal, 'task_label_in_use')) throw new Error(messages.inUse);
		throw new Error(messages.failure);
	}
}

function sortedByEnd(tasks: Task[]): Task[] {
	return [...tasks].sort((left, right) => (right.endDate ?? '').localeCompare(left.endDate ?? ''));
}

function memberOf(person: RecordPerson, tally: MemberTaskTally | undefined): TaskMember {
	return {
		id: person.personID,
		name: person.name,
		email: person.email,
		hireDate: person.hireDate || undefined,
		role: person.isAdmin ? 'admin' : 'member',
		distance: tally?.distance ?? 0,
		activeTaskCount: tally?.activeTaskCount ?? 0,
		completeTaskCount: tally?.completeTaskCount ?? 0
	};
}

function standingMetricsOf(
	scoreDetails: Record<string, TaskMemberScoreDetail>,
	tallies: Record<string, MemberTaskTally>
): Partial<TaskMetrics> {
	const memberScores = currentScoresOf(scoreDetails);
	const memberDistances = Object.fromEntries(
		Object.entries(tallies).map(([memberID, tally]) => [memberID, tally.distance])
	);
	return {
		memberScores,
		memberScoreDetails: scoreDetails,
		memberDistances,
		totalScore: totalScoreOf(memberScores),
		totalDistance: Object.values(memberDistances).reduce((total, distance) => total + distance, 0)
	};
}

function metricsOf(tasks: Task[]): TaskMetrics {
	const statusCounts: Record<string, number> = {};
	const businessCounts: Record<string, number> = {};
	const typeCounts: Record<string, number> = {};
	for (const task of tasks) {
		statusCounts[task.status] = (statusCounts[task.status] ?? 0) + 1;
		if (task.business) businessCounts[task.business] = (businessCounts[task.business] ?? 0) + 1;
		if (task.type) typeCounts[task.type] = (typeCounts[task.type] ?? 0) + 1;
	}
	return {
		totalTasks: tasks.length,
		completedTasks: statusCounts['completed'] ?? 0,
		requestedTasks: statusCounts['requested'] ?? 0,
		pausedTasks: statusCounts['paused'] ?? 0,
		stoppedTasks: statusCounts['stopped'] ?? 0,
		statusCounts,
		businessCounts,
		typeCounts
	};
}
