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
import { isSupabaseConfigured } from '$lib/supabase';
import { normalizedTaskDefinitionValue } from './task-workspace-model';
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

export async function fetchTaskWeeklySummary(week: string, fallbackMessage: string): Promise<TaskWeeklySummary> {
	if (isSupabaseConfigured()) return supabaseTaskWeeklySummary(week);
	const query = week ? `?week=${encodeURIComponent(week)}` : '';
	const response = await fetch(`/task/api/summary${query}`, { credentials: 'include' });
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
	const weeklySummary = (await response.json()) as TaskWeeklySummary;
	return { ...weeklySummary, weeklyTasks: (weeklySummary.weeklyTasks ?? []).map(normalizedTask) };
}

export async function fetchTaskState(fallbackMessage: string): Promise<TaskState> {
	if (isSupabaseConfigured()) return supabaseTaskState();
	const response = await fetch('/task/api/state', { credentials: 'include' });
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
	const state = (await response.json()) as TaskState;
	return { ...state, tasks: (state.tasks ?? []).map(normalizedTask) };
}

function normalizedTask(task: Task): Task {
	return {
		...task,
		business: normalizedTaskDefinitionValue(task.business),
		type: normalizedTaskDefinitionValue(task.type)
	};
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
		isAdmin: state.isAdmin,
		source: state.source
	};
}

export async function createQuickTask(request: TaskQuickTaskRequest, fallbackMessage: string): Promise<TaskQuickTaskResult> {
	if (isSupabaseConfigured()) return createQuickTaskThroughCompanyApp(request, fallbackMessage);
	const response = await fetch('/task/api/tasks/quick', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
	return quickTaskResultFromResponse(await responseJSON(response));
}

async function createQuickTaskThroughCompanyApp(
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

export async function saveTask(task: Task, fallbackMessage: string, statusBefore: string | null): Promise<void> {
	if (isSupabaseConfigured()) return saveSupabaseTask(task, statusBefore);
	const method = task.id ? 'PUT' : 'POST';
	const path = task.id ? `/task/api/tasks/${encodeURIComponent(task.id)}` : '/task/api/tasks';
	const response = await fetch(path, {
		method,
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(taskSavePayload(task))
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function moveTaskOnBoard(request: TaskBoardMoveRequest, fallbackMessage: string): Promise<void> {
	if (isSupabaseConfigured()) return moveSupabaseTask(request.taskID, request.targetStatus);
	const response = await fetch('/task/api/tasks/move', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function deleteTask(taskID: string, fallbackMessage: string): Promise<void> {
	if (isSupabaseConfigured()) return deleteSupabaseTask(taskID);
	const response = await fetch(`/task/api/tasks/${encodeURIComponent(taskID)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function updateTaskParent(
	taskID: string,
	parentTaskID: string | undefined,
	fallbackMessage: string
): Promise<void> {
	if (isSupabaseConfigured()) return updateSupabaseTaskParent(taskID, parentTaskID ?? null);
	const response = await fetch(`/task/api/tasks/${encodeURIComponent(taskID)}/parent`, {
		method: 'PATCH',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ parentTaskID: parentTaskID ?? null })
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function updateTaskParents(
	taskIDs: string[],
	parentTaskID: string,
	fallbackMessage: string
): Promise<void> {
	if (taskIDs.length === 0) return;
	if (isSupabaseConfigured()) return updateSupabaseTaskParents(taskIDs, parentTaskID);
	const response = await fetch('/task/api/tasks/parents', {
		method: 'PATCH',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ taskIDs, parentTaskID })
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function saveTaskDefinitions(
	definitions: TaskDefinitions,
	fallbackMessage: string,
	inUseMessage: string
): Promise<void> {
	if (isSupabaseConfigured()) {
		return saveSupabaseTaskVocabulary(definitions, { failure: fallbackMessage, inUse: inUseMessage });
	}
	const response = await fetch('/task/api/definitions', {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(definitions)
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

function responseErrorMessage(_response: Response, fallback: string): string {
	return fallback;
}

async function responseJSON(response: Response): Promise<unknown> {
	const contentType = response.headers.get('content-type') ?? '';
	if (!contentType.includes('application/json')) return null;
	return response.json() as Promise<unknown>;
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

function taskSavePayload(task: Task): Partial<Task> {
	const { createdAt: _createdAt, ...taskPayload } = task;
	if (taskPayload.id) {
		const { parentTaskID: _parentTaskID, ...existingTaskPayload } = taskPayload;
		return existingTaskPayload;
	}
	const { id: _id, statusRank, ...payload } = taskPayload;
	if (statusRank !== 0) return { ...payload, statusRank };
	return payload;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
