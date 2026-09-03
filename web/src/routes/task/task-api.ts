import type { TaskDefinitions, TaskState, TaskSummary, Task, TaskWeeklySummary } from './task-types';
import type { TaskBoardMoveRequest } from './task-board-drag';
import {
	deleteSupabaseTask,
	moveSupabaseTask,
	saveSupabaseTask,
	saveSupabaseTaskVocabulary,
	supabaseTaskState,
	supabaseTaskWeeklySummary
} from '$lib/task/supabase-task';
import {
	updateSupabaseTaskParent,
	updateSupabaseTaskParents
} from '$lib/task/supabase-task-relationships';
import { callCompanyApp } from '$lib/host-bridge';

export type TaskQuickTaskRequest = {
	prompt: string;
	ownerID: string;
	participantIDs: string[];
	weekCode: string;
	allowDuplicate?: boolean;
};

export type TaskQuickTaskResult = {
	status: 'created' | 'skipped_duplicate';
	reason: string;
};

export function fetchTaskWeeklySummary(week: string): Promise<TaskWeeklySummary> {
	return supabaseTaskWeeklySummary(week);
}

export function fetchTaskState(): Promise<TaskState> {
	return supabaseTaskState();
}

export function mergeTaskSummary(state: TaskState, weeklySummary: TaskWeeklySummary): TaskSummary {
	return {
		week: weeklySummary.week,
		currentWeek: state.currentWeek ?? weeklySummary.currentWeek,
		members: state.members,
		tasks: state.tasks,
		weeklyTasks: weeklySummary.weeklyTasks,
		metrics: {
			...weeklySummary.metrics,
			memberScores: state.metrics.memberScores ?? {},
			memberScoreDetails: state.metrics.memberScoreDetails ?? {},
			totalScore: state.metrics.totalScore ?? 0
		},
		definitions: state.definitions,
		report: weeklySummary.report,
		statusOptions: state.statusOptions,
		currentUserEmail: state.currentUserEmail,
		currentUserName: state.currentUserName,
		isAdmin: state.isAdmin
	};
}

export async function createQuickTask(
	request: TaskQuickTaskRequest,
	fallbackMessage: string
): Promise<TaskQuickTaskResult> {
	const answer = await callCompanyApp({
		capability: 'person.task.quick_task',
		body: {
			prompt: request.prompt,
			allowDuplicate: request.allowDuplicate ?? false
		}
	});
	if (answer.status >= 400) throw new Error(companyAppErrorMessage(answer.body, fallbackMessage));
	return quickTaskResultFromResponse(answer.body);
}

function companyAppErrorMessage(body: unknown, fallback: string): string {
	if (isRecord(body) && typeof body.error === 'string' && body.error.trim() !== '') return body.error;
	return fallback;
}

export function saveTask(task: Task, statusBefore: string | null): Promise<void> {
	return saveSupabaseTask(task, statusBefore);
}

export function moveTaskOnBoard(request: TaskBoardMoveRequest): Promise<void> {
	return moveSupabaseTask(request.taskID, request.targetStatus);
}

export function deleteTask(taskID: string): Promise<void> {
	return deleteSupabaseTask(taskID);
}

export function updateTaskParent(taskID: string, parentTaskID: string | undefined): Promise<void> {
	return updateSupabaseTaskParent(taskID, parentTaskID ?? null);
}

export async function updateTaskParents(taskIDs: string[], parentTaskID: string): Promise<void> {
	if (taskIDs.length === 0) return;
	return updateSupabaseTaskParents(taskIDs, parentTaskID);
}

export function saveTaskDefinitions(
	definitions: TaskDefinitions,
	fallbackMessage: string,
	inUseMessage: string
): Promise<void> {
	return saveSupabaseTaskVocabulary(definitions, { failure: fallbackMessage, inUse: inUseMessage });
}

function quickTaskResultFromResponse(value: unknown): TaskQuickTaskResult {
	if (!isRecord(value)) return { status: 'created', reason: '' };
	if (value.status === 'skipped_duplicate') {
		return {
			status: 'skipped_duplicate',
			reason: typeof value.reason === 'string' ? value.reason : ''
		};
	}
	return { status: 'created', reason: '' };
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
