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

let beingRead: Promise<TaskState> | null = null;

// The board asks for the state and the week at the same moment, and the week is
// a slice of the state. Callers within one turn share the read; nothing is held
// once it settles, so the next read is current.
export function taskState(): Promise<TaskState> {
	if (beingRead) return beingRead;
	const reading = readTaskState();
	beingRead = reading;
	void reading.catch(() => undefined).finally(() => {
		if (beingRead === reading) beingRead = null;
	});
	return reading;
}

async function readTaskState(): Promise<TaskState> {
	const [listed, directory] = await Promise.all([everyTaskOfTheCompany(), companyDirectory()]);
	const tasks = sortedByEnd(listed.tasks.map(taskOf));
	const people = directory.people;
	const memberIDs = people.map((person) => person.personID);
	const tallies = memberTaskTallies(tasks, memberIDs);
	const scoreDetails = memberScoreDetails(tasks, memberIDs, startOfISOWeek(new Date()));
	const me = people.find((person) => person.personID === directory.requesterID);

	return {
		currentWeek: taskWeekOfDate(new Date()),
		members: people.map((person) => memberOf(person, tallies[person.personID])),
		tasks,
		metrics: { ...metricsOf(tasks), ...standingMetricsOf(scoreDetails, tallies) },
		definitions: taskDefinitionsOf(listed.registeredLabels),
		statusOptions: taskStatusOptions,
		currentUserEmail: me?.email ?? '',
		currentUserName: me?.name ?? '',
		isAdmin: me?.isAdmin ?? false
	};
}

export async function taskWeeklySummary(week: string): Promise<TaskWeeklySummary> {
	const state = await taskState();
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

export async function saveTask(task: Task, statusBefore: string | null): Promise<void> {
	const written = writtenTaskOf(task);
	if (task.id) {
		await updateTask(task.id, written);
	} else {
		await addTask({ ...written, ...(task.parentTaskID ? { parentTaskHint: task.parentTaskID } : {}) });
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
	definitions: TaskDefinitions,
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
