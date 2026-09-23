import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';

export type TaskRunSummary = {
	taskRunID: string;
	requesterPersonID?: string;
	requesterDisplayName?: string;
	status: string;
	prompt?: string;
	result?: string;
	failureReason?: string;
	llmCostUSD?: number;
	llmCallCount?: number;
	createdAt?: string;
	updatedAt?: string;
};

export type TaskEvent = {
	id?: string;
	name: string;
	body: string;
	createdAt?: string;
};

export type TaskDetail = {
	taskRun: TaskRunSummary;
	taskEvents: TaskEvent[];
};

export type TaskDetailShareOptions = {
	events?: TaskEvent[];
	includeEvents?: boolean;
	title?: string;
};

export type TaskRunsRequest = {
	status?: string;
	limit?: number;
	offset?: number;
	includeTotal?: boolean;
	includeCost?: boolean;
	dailyCostTaskRunLimit?: number;
};

export type TaskRunsResponse = {
	taskRuns: TaskRunSummary[];
	totalCount?: number;
	dailyCostSummaries?: DailyCostSummary[];
	dailyCostScope?: DailyCostScope;
};

export type DailyCostSummary = {
	date: string;
	costUSD: number;
	taskRunCount: number;
	llmCallCount: number;
};

export type DailyCostScope = {
	taskRunLimit: number;
	taskRunCount: number;
	totalTaskRunCount: number;
	isTruncated: boolean;
};

export function taskRunsAPIPath(request: TaskRunsRequest = {}): string {
	const query = new URLSearchParams();
	setPositiveIntegerQuery(query, 'limit', request.limit);
	setPositiveIntegerQuery(query, 'offset', request.offset);
	setPositiveIntegerQuery(query, 'dailyCostTaskRunLimit', request.dailyCostTaskRunLimit);
	if (request.includeTotal) query.set('includeTotal', 'true');
	if (request.includeCost) query.set('includeCost', 'true');
	if (request.status) query.set('status', request.status);
	const queryString = query.toString();
	return queryString ? `/runs/api?${queryString}` : '/runs/api';
}

export async function fetchTaskRuns(request: TaskRunsRequest = {}): Promise<TaskRunsResponse> {
	if (isSupabaseConfigured()) return readTaskRunsResponse(await askTheCompanyApp(request));

	const response = await adminApiFetch(taskRunsAPIPath(request));
	if (!response.ok) {
		throw new Error(`Task list request returned ${response.status}`);
	}
	const document: unknown = await response.json();
	return readTaskRunsResponse(document);
}

export async function deleteTaskRun(taskRunID: string): Promise<void> {
	const response = await adminApiFetch(`/runs/api/${encodeURIComponent(taskRunID)}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error(`Task delete request returned ${response.status}`);
	}
}

export type RetryTaskRunResponse = {
	taskRunID: string;
	status: string;
};

export async function retryTaskRun(taskRunID: string): Promise<RetryTaskRunResponse> {
	const request = { taskRunID };
	const document = isSupabaseConfigured()
		? await askTheCompanyAppToRetry(request)
		: await askTheDeviceToRetry(request);
	return readRetryTaskRunResponse(document);
}

export function readRetryTaskRunResponse(document: unknown): RetryTaskRunResponse {
	const record = readRecord(document);
	if (!record || typeof record.taskRunID !== 'string' || !record.taskRunID.trim() || typeof record.status !== 'string' || !record.status.trim()) {
		throw new Error('Retry task run response was malformed');
	}
	return { taskRunID: record.taskRunID, status: record.status };
}

export const waitingApprovalStatus = 'waiting_approval';
export const confirmationRequestedEventName = 'confirmation.requested';
export const askRequestedEventName = 'ask.requested';
export const approvalDecisions = ['confirm', 'confirm_task', 'cancel'] as const;

export type ApprovalDecision = (typeof approvalDecisions)[number];

export type ApprovalDecisionRequest = {
	taskRunID: string;
	decision: ApprovalDecision;
};

export type ApprovalOutcome = {
	taskRunID: string;
	status: string;
};

export type PendingApproval = {
	taskRun: TaskRunSummary;
	question: string;
	scope: string;
};

export function approvalDecisionRequestOf(taskRunID: string, decision: ApprovalDecision): ApprovalDecisionRequest {
	return { taskRunID, decision };
}

export function pendingApprovalOf(detail: TaskDetail): PendingApproval | undefined {
	if (detail.taskRun.status !== waitingApprovalStatus) return undefined;
	return {
		taskRun: detail.taskRun,
		question: lastEventField(detail.taskEvents, confirmationRequestedEventName, 'userFacingMessage'),
		scope: lastEventField(detail.taskEvents, askRequestedEventName, 'approvalScope')
	};
}

function lastEventField(taskEvents: TaskEvent[], eventName: string, fieldName: string): string {
	for (let index = taskEvents.length - 1; index >= 0; index -= 1) {
		if (taskEvents[index].name !== eventName) continue;
		const body = readRecord(parseEventBody(taskEvents[index].body));
		const value = body?.[fieldName];
		if (typeof value === 'string' && value.trim()) return value.trim();
	}
	return '';
}

export async function fetchPendingApprovals(limit = 20): Promise<PendingApproval[]> {
	const response = await fetchTaskRuns({ status: waitingApprovalStatus, limit });
	const details = await Promise.all(response.taskRuns.map((taskRun) => fetchTaskDetail(taskRun.taskRunID)));
	return details.flatMap((detail) => {
		const approval = pendingApprovalOf(detail);
		return approval ? [approval] : [];
	});
}

export async function decideApproval(taskRunID: string, decision: ApprovalDecision): Promise<ApprovalOutcome> {
	const request = approvalDecisionRequestOf(taskRunID, decision);
	const document = isSupabaseConfigured()
		? await askTheCompanyAppToDecide(request)
		: await askTheDeviceToDecide(request);
	return readApprovalOutcome(document, taskRunID);
}

export function readApprovalOutcome(document: unknown, taskRunID: string): ApprovalOutcome {
	const record = readRecord(document);
	return {
		taskRunID: record && typeof record.taskRunID === 'string' ? record.taskRunID : taskRunID,
		status: record && typeof record.status === 'string' ? record.status : ''
	};
}

async function askTheDeviceToDecide(request: ApprovalDecisionRequest): Promise<unknown> {
	const response = await adminApiFetch('/runs/api/approve', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(`Task approval request returned ${response.status}`);
	return response.json();
}

async function askTheCompanyAppToDecide(request: ApprovalDecisionRequest): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.runs.approve', body: { ...request } });
	if (answer.status >= 400) throw new Error(`Task approval request returned ${answer.status}`);
	return answer.body;
}

async function askTheDeviceToRetry(request: { taskRunID: string }): Promise<unknown> {
	const response = await adminApiFetch('/runs/api/retry', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) throw new Error(`Task retry request returned ${response.status}`);
	return response.json();
}

async function askTheCompanyAppToRetry(request: { taskRunID: string }): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.runs.retry', body: request });
	if (answer.status >= 400) throw new Error(`Task retry request returned ${answer.status}`);
	return answer.body;
}

function readTaskRunsResponse(document: unknown): TaskRunsResponse {
	if (Array.isArray(document)) {
		return { taskRuns: readTaskRunSummaries(document) };
	}
	const record = readRecord(document);
	if (!record || !Array.isArray(record.taskRuns)) return { taskRuns: [] };
	return {
		taskRuns: readTaskRunSummaries(record.taskRuns),
		totalCount: typeof record.totalCount === 'number' && record.totalCount >= 0 ? Math.floor(record.totalCount) : undefined,
		dailyCostSummaries: Array.isArray(record.dailyCostSummaries) ? readDailyCostSummaries(record.dailyCostSummaries) : undefined,
		dailyCostScope: readDailyCostScope(record.dailyCostScope)
	};
}

function readTaskRunSummaries(entries: unknown[]): TaskRunSummary[] {
	return entries.flatMap((entry) => {
		const taskRun = readTaskRunSummary(entry);
		return taskRun ? [taskRun] : [];
	});
}

function setPositiveIntegerQuery(query: URLSearchParams, key: string, value: number | undefined): void {
	if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return;
	query.set(key, String(Math.floor(value)));
}

function readDailyCostSummaries(entries: unknown[]): DailyCostSummary[] {
	return entries.flatMap((entry) => {
		const record = readRecord(entry);
		if (!record || typeof record.date !== 'string') return [];
		return [
			{
				date: record.date,
				costUSD: readNonNegativeNumber(record.costUSD),
				taskRunCount: Math.floor(readNonNegativeNumber(record.taskRunCount)),
				llmCallCount: Math.floor(readNonNegativeNumber(record.llmCallCount))
			}
		];
	});
}

function readDailyCostScope(entry: unknown): DailyCostScope | undefined {
	const record = readRecord(entry);
	if (!record) return undefined;
	return {
		taskRunLimit: Math.floor(readNonNegativeNumber(record.taskRunLimit)),
		taskRunCount: Math.floor(readNonNegativeNumber(record.taskRunCount)),
		totalTaskRunCount: Math.floor(readNonNegativeNumber(record.totalTaskRunCount)),
		isTruncated: record.isTruncated === true
	};
}

export async function fetchTaskDetail(taskRunID: string): Promise<TaskDetail> {
	return readTaskDetail(
		isSupabaseConfigured() ? await askTheCompanyAppForDetail(taskRunID) : await askTheDeviceForDetail(taskRunID)
	);
}

export function readTaskDetail(document: unknown): TaskDetail {
	const record = readRecord(document);
	const taskRun = record ? readTaskRunSummary(record.taskRun) : undefined;
	if (!record || !taskRun) {
		throw new Error('Task detail response was malformed');
	}
	return { taskRun, taskEvents: readTaskEvents(record.taskEvents) };
}

function readTaskEvents(entries: unknown): TaskEvent[] {
	if (!Array.isArray(entries)) return [];
	return entries.flatMap((entry) => {
		const record = readRecord(entry);
		if (!record || typeof record.name !== 'string') return [];
		return [
			{
				id: typeof record.taskEventID === 'string' ? record.taskEventID : undefined,
				name: record.name,
				body: typeof record.body === 'string' ? record.body : '',
				createdAt: typeof record.createdAt === 'string' ? record.createdAt : undefined
			}
		];
	});
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

export function taskDetailShareText(detail: TaskDetail, options: TaskDetailShareOptions = {}): string {
	const taskRun = detail.taskRun;
	const selectedEvents = options.events ?? detail.taskEvents;
	const includeEvents = options.includeEvents ?? true;
	const sections = [
		`# ${options.title ?? 'Task Run'} ${taskRun.taskRunID}`,
		[
			shareLine('status', taskRun.status),
			shareLine('createdAt', taskRun.createdAt),
			shareLine('updatedAt', taskRun.updatedAt),
			shareLine('requester', taskRun.requesterDisplayName || taskRun.requesterPersonID),
			shareLine('prompt', taskRun.prompt),
			shareLine('result', taskRun.result),
			shareLine('failureReason', taskRun.failureReason),
			shareLine('eventCount', String(detail.taskEvents.length))
		]
			.filter(Boolean)
			.join('\n')
	];
	if (includeEvents && selectedEvents.length > 0) {
		sections.push(['## Events', selectedEvents.map((taskEvent, index) => taskEventShareText(taskEvent, index + 1)).join('\n\n')].join('\n\n'));
	}
	return sections.filter(Boolean).join('\n\n');
}

export function taskEventShareText(taskEvent: TaskEvent, eventNumber?: number): string {
	const heading = eventNumber ? `### Event ${eventNumber}: ${taskEvent.name}` : `### ${taskEvent.name}`;
	const formattedBody = formatEventBody(taskEvent.body);
	const bodyLanguage = parseEventBody(taskEvent.body) === undefined ? 'text' : 'json';
	return [
		heading,
		taskEvent.createdAt ? shareLine('createdAt', taskEvent.createdAt) : '',
		'',
		`${markdownFence(formattedBody)}${bodyLanguage}`,
		formattedBody,
		markdownFence(formattedBody)
	]
		.filter((line) => line !== '')
		.join('\n');
}

function shareLine(label: string, value: string | undefined): string {
	const trimmedValue = value?.trim();
	if (!trimmedValue) return '';
	return `- ${label}: ${trimmedValue}`;
}

function markdownFence(content: string): string {
	let fenceLength = 3;
	for (const match of content.matchAll(/`+/g)) {
		fenceLength = Math.max(fenceLength, match[0].length + 1);
	}
	return '`'.repeat(fenceLength);
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
		requesterDisplayName: typeof record.requesterDisplayName === 'string' ? record.requesterDisplayName : undefined,
		status: record.status,
		prompt: typeof record.prompt === 'string' ? record.prompt : undefined,
		result: typeof record.result === 'string' ? record.result : undefined,
		failureReason: typeof record.failureReason === 'string' ? record.failureReason : undefined,
		llmCostUSD: typeof record.llmCostUSD === 'number' && record.llmCostUSD >= 0 ? record.llmCostUSD : undefined,
		llmCallCount: typeof record.llmCallCount === 'number' && record.llmCallCount >= 0 ? Math.floor(record.llmCallCount) : undefined,
		createdAt: typeof record.createdAt === 'string' ? record.createdAt : undefined,
		updatedAt: typeof record.updatedAt === 'string' ? record.updatedAt : undefined
	};
}

function readNonNegativeNumber(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return 0;
	return value;
}

function readRecord(value: unknown): Record<string, unknown> | undefined {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
	return value as Record<string, unknown>;
}

async function askTheDeviceForDetail(taskRunID: string): Promise<unknown> {
	const query = new URLSearchParams({ taskRunID });
	const response = await adminApiFetch(`/runs/api/detail?${query.toString()}`);
	if (!response.ok) throw new Error(`Task detail request returned ${response.status}`);
	return response.json();
}

async function askTheCompanyAppForDetail(taskRunID: string): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.runs.detail', body: { taskRunID } });
	if (answer.status >= 400) throw new Error(`Task detail request returned ${answer.status}`);
	return answer.body;
}

async function askTheCompanyApp(request: TaskRunsRequest): Promise<unknown> {
	const body: Record<string, unknown> = {};
	if (request.limit) body.limit = request.limit;
	if (request.offset) body.offset = request.offset;
	if (request.dailyCostTaskRunLimit) body.dailyCostTaskRunLimit = request.dailyCostTaskRunLimit;
	if (request.includeTotal) body.includeTotal = true;
	if (request.includeCost) body.includeCost = true;
	if (request.status) body.status = request.status;

	const answer = await callCompanyApp({ capability: 'person.runs.list', body });
	if (answer.status >= 400) throw new Error(`Task list request returned ${answer.status}`);
	return answer.body;
}
