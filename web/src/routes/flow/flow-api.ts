import type { FlowDefinitions, FlowState, FlowSummary, FlowTask, FlowWeeklySummary } from './flow-types';
import type { FlowTaskBoardMoveRequest } from './flow-task-board-drag';
import {
	deleteSupabaseFlowTask,
	moveSupabaseFlowTask,
	saveSupabaseFlowTask,
	supabaseFlowState,
	supabaseFlowWeeklySummary
} from '$lib/flow/supabase-flow';
import { isSupabaseConfigured } from '$lib/supabase';

export type FlowQuickTaskRequest = {
	prompt: string;
	ownerID: string;
	participantIDs: string[];
	weekCode: string;
	allowDuplicate?: boolean;
};

export type FlowQuickTaskResult = {
	status: 'created' | 'skipped_duplicate';
	reason: string;
};

export async function fetchFlowWeeklySummary(week: string, fallbackMessage: string): Promise<FlowWeeklySummary> {
	if (isSupabaseConfigured) return supabaseFlowWeeklySummary(week);
	const query = week ? `?week=${encodeURIComponent(week)}` : '';
	const response = await fetch(`/flow/api/summary${query}`, { credentials: 'include' });
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
	return (await response.json()) as FlowWeeklySummary;
}

export async function fetchFlowState(fallbackMessage: string): Promise<FlowState> {
	if (isSupabaseConfigured) return supabaseFlowState();
	const response = await fetch('/flow/api/state', { credentials: 'include' });
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
	return (await response.json()) as FlowState;
}

export function mergeFlowSummary(state: FlowState, weeklySummary: FlowWeeklySummary): FlowSummary {
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

export async function createQuickFlowTask(request: FlowQuickTaskRequest, fallbackMessage: string): Promise<FlowQuickTaskResult> {
	const response = await fetch('/flow/api/tasks/quick', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
	return quickTaskResultFromResponse(await responseJSON(response));
}

export async function saveFlowTask(task: FlowTask, fallbackMessage: string): Promise<void> {
	if (isSupabaseConfigured) return saveSupabaseFlowTask(task);
	const method = task.id ? 'PUT' : 'POST';
	const path = task.id ? `/flow/api/tasks/${encodeURIComponent(task.id)}` : '/flow/api/tasks';
	const response = await fetch(path, {
		method,
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(flowTaskSavePayload(task))
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function moveFlowTaskOnBoard(request: FlowTaskBoardMoveRequest, fallbackMessage: string): Promise<void> {
	if (isSupabaseConfigured) return moveSupabaseFlowTask(request.taskID, request.targetStatus);
	const response = await fetch('/flow/api/tasks/move', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function deleteFlowTask(taskID: string, fallbackMessage: string): Promise<void> {
	if (isSupabaseConfigured) return deleteSupabaseFlowTask(taskID);
	const response = await fetch(`/flow/api/tasks/${encodeURIComponent(taskID)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
}

export async function saveFlowDefinitions(definitions: FlowDefinitions, fallbackMessage: string): Promise<void> {
	const response = await fetch('/flow/api/definitions', {
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

function quickTaskResultFromResponse(value: unknown): FlowQuickTaskResult {
	if (!isRecord(value)) return { status: 'created', reason: '' };
	if (value.status === 'skipped_duplicate') {
		return {
			status: 'skipped_duplicate',
			reason: typeof value.reason === 'string' ? value.reason : ''
		};
	}
	return { status: 'created', reason: '' };
}

function flowTaskSavePayload(task: FlowTask): Partial<FlowTask> {
	const { createdAt: _createdAt, ...taskPayload } = task;
	if (taskPayload.id) return taskPayload;
	const { id: _id, statusRank, ...payload } = taskPayload;
	if (statusRank !== 0) return { ...payload, statusRank };
	return payload;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
