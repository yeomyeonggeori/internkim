import { adminApiFetch } from '$lib/admin-api';

export type TaskRunSummary = {
	taskRunID: string;
	requesterPersonID?: string;
	status: string;
	prompt?: string;
	result?: string;
	failureReason?: string;
	createdAt?: string;
	updatedAt?: string;
};

export type TaskEvent = {
	name: string;
	body: string;
	createdAt?: string;
};

export type TaskDetail = {
	taskRun: TaskRunSummary;
	taskEvents: TaskEvent[];
};

export async function fetchTaskRuns(status?: string): Promise<TaskRunSummary[]> {
	const query = new URLSearchParams({ limit: '100' });
	if (status) query.set('status', status);
	const response = await adminApiFetch(`/admin/api/diagnostics/tasks?${query.toString()}`);
	if (!response.ok) {
		throw new Error(`Task list request returned ${response.status}`);
	}
	const document: unknown = await response.json();
	if (!Array.isArray(document)) return [];
	return document.flatMap((entry) => {
		const taskRun = readTaskRunSummary(entry);
		return taskRun ? [taskRun] : [];
	});
}

export async function fetchTaskDetail(taskRunID: string): Promise<TaskDetail> {
	const query = new URLSearchParams({ taskRunID });
	const response = await adminApiFetch(`/admin/api/diagnostics/task-detail?${query.toString()}`);
	if (!response.ok) {
		throw new Error(`Task detail request returned ${response.status}`);
	}
	const document: unknown = await response.json();
	const record = readRecord(document);
	const taskRun = record ? readTaskRunSummary(record.taskRun) : undefined;
	if (!record || !taskRun) {
		throw new Error('Task detail response was malformed');
	}
	const taskEvents = Array.isArray(record.taskEvents)
		? record.taskEvents.flatMap((entry) => {
				const eventRecord = readRecord(entry);
				if (!eventRecord || typeof eventRecord.name !== 'string') return [];
				return [
					{
						name: eventRecord.name,
						body: typeof eventRecord.body === 'string' ? eventRecord.body : '',
						createdAt: typeof eventRecord.createdAt === 'string' ? eventRecord.createdAt : undefined
					}
				];
			})
		: [];
	return { taskRun, taskEvents };
}

export type EventLane = 'llm' | 'tool' | 'failure' | 'control';

export function eventLane(eventName: string): EventLane {
	if (eventName === 'llm.call') return 'llm';
	if (eventName.startsWith('tool.')) return 'tool';
	if (eventName.startsWith('agent.failure') || eventName === 'agent.recovery_attempt') return 'failure';
	return 'control';
}

export type TimelineSummary = {
	llmCallCount: number;
	llmLatencyMS: number;
	llmTotalTokens: number;
	llmCachedPromptTokens: number;
	llmCostUSD: number;
	toolCallCount: number;
};

export function summarizeTimeline(taskEvents: TaskEvent[]): TimelineSummary {
	const summary: TimelineSummary = {
		llmCallCount: 0,
		llmLatencyMS: 0,
		llmTotalTokens: 0,
		llmCachedPromptTokens: 0,
		llmCostUSD: 0,
		toolCallCount: 0
	};
	for (const taskEvent of taskEvents) {
		if (taskEvent.name === 'llm.call') {
			summary.llmCallCount += 1;
			const body = readRecord(parseEventBody(taskEvent.body));
			if (body && typeof body.latencyMs === 'number') summary.llmLatencyMS += body.latencyMs;
			if (body && typeof body.totalTokens === 'number') summary.llmTotalTokens += body.totalTokens;
			if (body && typeof body.cachedPromptTokens === 'number') summary.llmCachedPromptTokens += body.cachedPromptTokens;
			summary.llmCostUSD += effectiveCallCostUSD(body);
		}
		if (taskEvent.name.startsWith('tool.') && taskEvent.name.endsWith('.result')) {
			summary.toolCallCount += 1;
		}
	}
	return summary;
}

function effectiveCallCostUSD(body: Record<string, unknown> | undefined): number {
	if (!body) return 0;
	const cost = typeof body.costUSD === 'number' ? body.costUSD : 0;
	if (cost > 0) return cost;
	const upstreamCost = typeof body.upstreamInferenceCostUSD === 'number' ? body.upstreamInferenceCostUSD : 0;
	return upstreamCost > 0 ? upstreamCost : 0;
}

export function formatCostUSD(costUSD: number): string {
	if (costUSD <= 0) return '$0';
	if (costUSD < 1) return `$${costUSD.toFixed(4)}`;
	return `$${costUSD.toFixed(2)}`;
}

export function parseEventBody(body: string): unknown {
	try {
		return JSON.parse(body);
	} catch {
		return undefined;
	}
}

export function formatEventBody(body: string): string {
	const parsed = parseEventBody(body);
	if (parsed === undefined) return body;
	return JSON.stringify(parsed, undefined, 2);
}

export type ServiceLogsResponse = { service: string; taskRunID?: string; count: number; lines: string[] };

export async function fetchServiceLogs(service: string, taskRunID: string, limit = 200): Promise<ServiceLogsResponse> {
	const query = new URLSearchParams({ service, taskRunID, limit: String(limit) });
	const response = await adminApiFetch(`/admin/api/diagnostics/service-logs?${query.toString()}`);
	if (!response.ok) {
		throw new Error(`Service logs request returned ${response.status}`);
	}
	const document: unknown = await response.json();
	const record = readRecord(document);
	return {
		service: record && typeof record.service === 'string' ? record.service : service,
		taskRunID: record && typeof record.taskRunID === 'string' ? record.taskRunID : undefined,
		count: record && typeof record.count === 'number' ? record.count : 0,
		lines:
			record && Array.isArray(record.lines)
				? record.lines.filter((line): line is string => typeof line === 'string')
				: []
	};
}

function readTaskRunSummary(entry: unknown): TaskRunSummary | undefined {
	const record = readRecord(entry);
	if (!record || typeof record.taskRunID !== 'string' || typeof record.status !== 'string') {
		return undefined;
	}
	return {
		taskRunID: record.taskRunID,
		requesterPersonID: typeof record.requesterPersonID === 'string' ? record.requesterPersonID : undefined,
		status: record.status,
		prompt: typeof record.prompt === 'string' ? record.prompt : undefined,
		result: typeof record.result === 'string' ? record.result : undefined,
		failureReason: typeof record.failureReason === 'string' ? record.failureReason : undefined,
		createdAt: typeof record.createdAt === 'string' ? record.createdAt : undefined,
		updatedAt: typeof record.updatedAt === 'string' ? record.updatedAt : undefined
	};
}

function readRecord(value: unknown): Record<string, unknown> | undefined {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
	return value as Record<string, unknown>;
}
