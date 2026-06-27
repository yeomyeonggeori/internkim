import type { Plugin } from 'vite';
import {
	createDevAdminMockResponse,
	createDevAdminMockState,
	readRequestBody,
	shouldHandleDevAdminMockRequest,
	writeJSON,
	type DevAdminMockState,
	type DevMockRequest,
	type DevMockResponse
} from './dev-admin-mock';
import type { DailyCostScope, DailyCostSummary, TaskDetail, TaskEvent, TaskRunSummary } from './src/routes/tasks/tasks-api';

type DevTasksMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevTasksMockState = DevAdminMockState & {
	taskRuns: TaskRunSummary[];
};

const taskRunStatuses = ['completed', 'completed', 'completed', 'failed', 'running', 'waiting_approval', 'blocked'] as const;
const defaultDailyCostTaskRunLimit = 500;
const maxDailyCostTaskRunLimit = 1000;
const deletableTaskRunStatuses = new Set(['completed', 'failed', 'cancelled', 'blocked']);

export function devTasksMockPlugin(options: DevTasksMockPluginOptions): Plugin {
	const state = createDevTasksMockState(options.userEmail);

	return {
		name: 'internkim-dev-tasks-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevTasksMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevTasksMockResponse(state, {
						method,
						pathname: requestURL.pathname,
						searchParams: requestURL.searchParams,
						body
					});
					if (!mockResponse) {
						next();
						return;
					}
					writeJSON(response, mockResponse.status, mockResponse.body);
				});
			});
		}
	};
}

export function createDevTasksMockState(userEmail: string): DevTasksMockState {
	return {
		...createDevAdminMockState(userEmail),
		taskRuns: createDevTaskRuns()
	};
}

export function createDevTasksMockResponse(
	state: DevTasksMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/tasks/api/runs') {
		return { status: 200, body: paginatedTaskRuns(state.taskRuns, request.searchParams) };
	}
	if (request.method === 'GET' && request.pathname === '/tasks/api/run-detail') {
		return taskDetailResponse(state, request.searchParams);
	}
	if (request.method === 'DELETE' && request.pathname.startsWith('/tasks/api/runs/')) {
		return deleteTaskRunResponse(state, request.pathname);
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/diagnostics/tasks') {
		return { status: 200, body: paginatedTaskRuns(state.taskRuns, request.searchParams) };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/diagnostics/service-logs') {
		return serviceLogsResponse(request.searchParams);
	}
	return createDevAdminMockResponse(state, request);
}

function shouldHandleDevTasksMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/tasks/api/runs') return true;
	if (method === 'GET' && pathname === '/tasks/api/run-detail') return true;
	if (method === 'DELETE' && pathname.startsWith('/tasks/api/runs/')) return true;
	if (method === 'GET' && pathname === '/admin/api/diagnostics/tasks') return true;
	if (method === 'GET' && pathname === '/admin/api/diagnostics/service-logs') return true;
	return shouldHandleDevAdminMockRequest(method, pathname);
}

function createDevTaskRuns(): TaskRunSummary[] {
	return Array.from({ length: 60 }, (_, index) => {
		const taskNumber = index + 1;
		const status = taskRunStatuses[index % taskRunStatuses.length];
		const updatedAt = new Date(Date.UTC(2026, 5, 17, 1, 0, 0) - index * 12 * 60 * 1000).toISOString();
		return {
			taskRunID: `dev-task-run-${String(taskNumber).padStart(3, '0')}`,
			requesterPersonID: 'dev-person-admin',
			status,
			prompt: `개발 확인용 작업 기록 ${taskNumber}`,
			failureReason: status === 'failed' ? 'mock failure reason for pagination check' : undefined,
			llmCostUSD: devTaskRunCostUSD(index),
			llmCallCount: devTaskRunCallCount(index),
			createdAt: updatedAt,
			updatedAt
		};
	});
}

function taskDetailResponse(state: DevTasksMockState, searchParams: URLSearchParams): DevMockResponse {
	const taskRunID = searchParams.get('taskRunID') ?? '';
	const taskRun = state.taskRuns.find((candidate) => candidate.taskRunID === taskRunID);
	if (!taskRun) return { status: 404, body: { error: 'task_run_not_found' } };
	return { status: 200, body: createDevTaskDetail(taskRun) };
}

function deleteTaskRunResponse(state: DevTasksMockState, pathname: string): DevMockResponse {
	const taskRunID = decodeURIComponent(pathname.slice('/tasks/api/runs/'.length));
	const taskRun = state.taskRuns.find((candidate) => candidate.taskRunID === taskRunID);
	if (!taskRun) return { status: 404, body: { error: 'task_run_not_found' } };
	if (!deletableTaskRunStatuses.has(taskRun.status)) {
		return { status: 409, body: { error: 'task_run_not_deletable' } };
	}
	state.taskRuns = state.taskRuns.filter((candidate) => candidate.taskRunID !== taskRunID);
	return { status: 200, body: { status: 'deleted', taskRunID } };
}

function createDevTaskDetail(taskRun: TaskRunSummary): TaskDetail {
	return {
		taskRun: {
			...taskRun,
			result:
				taskRun.status === 'completed'
					? '요청한 작업을 완료했고 게시 URL과 주요 변경 사항을 사용자에게 전달했습니다.'
					: taskRun.result
		},
		taskEvents: createDevTaskEvents(taskRun)
	};
}

function createDevTaskEvents(taskRun: TaskRunSummary): TaskEvent[] {
	return [
		taskEvent('task.created', taskRun.prompt ?? '', taskRun.createdAt),
		taskEvent('llm.call', {
			kind: 'structured',
			schemaName: 'blueclaw_agent_turn_action',
			provider: 'openrouter',
			model: 'google/gemini-3.1-flash-lite',
			latencyMs: 3465,
			promptBytes: 137154,
			contentBytes: 1125,
			promptTokens: 34170,
			completionTokens: 268,
			totalTokens: 34438,
			costUSD: taskRun.llmCostUSD ?? 0.0089445
		}, taskRun.createdAt),
		taskEvent('agent.action', {
			action: 'continue',
			toolName: 'site.app.create',
			toolInput: {
				title: '맛있는 귤 세상',
				slug: 'tasty-tangerine',
				description: '귤 소개 웹사이트'
			},
			reason: '사용자가 사이트 생성을 요청했으므로 사이트 생성 도구를 호출합니다.'
		}, taskRun.createdAt),
		taskEvent('tool.site.app.create.requested', {
			observationID: 'obs-006',
			toolName: 'site.app.create',
			input: {
				title: '맛있는 귤 세상',
				slug: 'tasty-tangerine',
				audience: '귤을 좋아하는 사람들'
			}
		}, taskRun.updatedAt),
		taskEvent('tool.site.app.publish.result', {
			observationID: 'obs-012',
			tool: 'site.app.publish',
			output: {
				data: {
					siteID: '0da25b8c036e2cb7a05e3200',
					slug: 'tangerine-hub',
					publishedURL: 'https://tangerine-hub.zd2df6qt6jmc.intern.kim',
					status: 'published',
					tlsStatus: 'active'
				}
			}
		}, taskRun.updatedAt),
		taskEvent('agent.action', {
			action: taskRun.status === 'completed' ? 'finish' : 'continue',
			message: taskRun.status === 'completed'
				? '요청하신 작업이 완료되어 게시되었습니다.'
				: '작업을 계속 진행합니다.',
			goalStatus: taskRun.status === 'completed' ? 'satisfied' : 'working',
			goalSatisfied: taskRun.status === 'completed'
		}, taskRun.updatedAt)
	];
}

function taskEvent(name: string, body: unknown, createdAt?: string): TaskEvent {
	return {
		name,
		body: typeof body === 'string' ? body : JSON.stringify(body),
		createdAt
	};
}

function serviceLogsResponse(searchParams: URLSearchParams): DevMockResponse {
	const taskRunID = searchParams.get('taskRunID') ?? undefined;
	return {
		status: 200,
		body: {
			service: searchParams.get('service') ?? 'blueclaw',
			taskRunID,
			count: 3,
			lines: [
				`2026-06-25T07:29:04Z task=${taskRunID ?? 'unknown'} tool.site.app.publish started`,
				`2026-06-25T07:29:05Z task=${taskRunID ?? 'unknown'} publish URL https://tangerine-hub.zd2df6qt6jmc.intern.kim`,
				`2026-06-25T07:29:06Z task=${taskRunID ?? 'unknown'} task completed`
			]
		}
	};
}

function paginatedTaskRuns(
	taskRuns: TaskRunSummary[],
	searchParams: URLSearchParams
): TaskRunSummary[] | { taskRuns: TaskRunSummary[]; totalCount: number; dailyCostSummaries: DailyCostSummary[]; dailyCostScope?: DailyCostScope } {
	const status = searchParams.get('status')?.trim();
	const filteredTaskRuns = status ? taskRuns.filter((taskRun) => taskRun.status === status) : taskRuns;
	const offset = positiveIntegerFromSearch(searchParams, 'offset', 0);
	const limit = positiveIntegerFromSearch(searchParams, 'limit', filteredTaskRuns.length);
	const shouldIncludeCost = searchParams.get('includeCost') === 'true';
	const dailyCostTaskRunLimit = dailyCostTaskRunLimitFromSearch(searchParams);
	const selectedTaskRuns = filteredTaskRuns.slice(offset, offset + limit);
	const responseTaskRuns = shouldIncludeCost ? selectedTaskRuns : selectedTaskRuns.map(taskRunWithoutCost);
	if (searchParams.get('includeTotal') === 'true') {
		const dailyCostTaskRuns = filteredTaskRuns.slice(0, dailyCostTaskRunLimit);
		return {
			taskRuns: responseTaskRuns,
			totalCount: filteredTaskRuns.length,
			dailyCostSummaries: shouldIncludeCost ? dailyCostSummaries(dailyCostTaskRuns) : [],
			dailyCostScope: shouldIncludeCost ? dailyCostScope(filteredTaskRuns.length, dailyCostTaskRuns.length, dailyCostTaskRunLimit) : undefined
		};
	}
	return responseTaskRuns;
}

function taskRunWithoutCost(taskRun: TaskRunSummary): TaskRunSummary {
	return {
		taskRunID: taskRun.taskRunID,
		requesterPersonID: taskRun.requesterPersonID,
		requesterDisplayName: taskRun.requesterDisplayName,
		status: taskRun.status,
		prompt: taskRun.prompt,
		result: taskRun.result,
		failureReason: taskRun.failureReason,
		createdAt: taskRun.createdAt,
		updatedAt: taskRun.updatedAt
	};
}

function dailyCostSummaries(taskRuns: TaskRunSummary[]): DailyCostSummary[] {
	const summaryByDate = new Map<string, DailyCostSummary>();
	for (const taskRun of taskRuns) {
		const date = taskRun.createdAt?.slice(0, 10);
		if (!date) continue;
		const costUSD = taskRun.llmCostUSD ?? 0;
		const llmCallCount = taskRun.llmCallCount ?? 0;
		if (costUSD <= 0 && llmCallCount <= 0) continue;
		const summary = summaryByDate.get(date) ?? { date, costUSD: 0, taskRunCount: 0, llmCallCount: 0 };
		summary.costUSD += costUSD;
		summary.taskRunCount += 1;
		summary.llmCallCount += llmCallCount;
		summaryByDate.set(date, summary);
	}
	return [...summaryByDate.values()].sort((left, right) => right.date.localeCompare(left.date));
}

function dailyCostScope(totalTaskRunCount: number, taskRunCount: number, taskRunLimit: number): DailyCostScope {
	return {
		taskRunLimit,
		taskRunCount,
		totalTaskRunCount,
		isTruncated: taskRunCount < totalTaskRunCount
	};
}

function dailyCostTaskRunLimitFromSearch(searchParams: URLSearchParams): number {
	const limit = positiveIntegerFromSearch(searchParams, 'dailyCostTaskRunLimit', defaultDailyCostTaskRunLimit);
	if (limit <= 0) return defaultDailyCostTaskRunLimit;
	if (limit > maxDailyCostTaskRunLimit) return maxDailyCostTaskRunLimit;
	return limit;
}

function devTaskRunCostUSD(index: number): number {
	return Number((0.0012 + (index % 6) * 0.00085).toFixed(6));
}

function devTaskRunCallCount(index: number): number {
	return 1 + (index % 3);
}

function positiveIntegerFromSearch(searchParams: URLSearchParams, key: string, fallback: number): number {
	const value = Number(searchParams.get(key));
	if (!Number.isFinite(value) || value < 0) return fallback;
	return Math.floor(value);
}
