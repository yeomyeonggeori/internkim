import type { FlowDefinitions, FlowSummary, FlowTask } from './flow-types';

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

export async function fetchFlowSummary(week: string, fallbackMessage: string): Promise<FlowSummary> {
	const query = week ? `?week=${encodeURIComponent(week)}` : '';
	const response = await fetch(`/flow/api/summary${query}`, { credentials: 'include' });
	if (!response.ok) throw new Error(responseErrorMessage(response, fallbackMessage));
	return (await response.json()) as FlowSummary;
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
	const method = task.id ? 'PUT' : 'POST';
	const path = task.id ? `/flow/api/tasks/${encodeURIComponent(task.id)}` : '/flow/api/tasks';
	const response = await fetch(path, {
		method,
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(task)
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

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
