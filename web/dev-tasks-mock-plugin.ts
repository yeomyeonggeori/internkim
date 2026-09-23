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
import type { DailyCostScope, DailyCostSummary, TaskDetail, TaskEvent, TaskRunSummary } from './src/routes/runs/runs-api';

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
	if (request.method === 'GET' && request.pathname === '/runs/api') {
		return { status: 200, body: paginatedTaskRuns(state.taskRuns, request.searchParams) };
	}
	if (request.method === 'GET' && request.pathname === '/runs/api/detail') {
		return taskDetailResponse(state, request.searchParams);
	}
	if (request.method === 'GET' && request.pathname === '/runs/api/llm-call') {
		return { status: 200, body: devLLMCallExchange(request.searchParams.get('id') ?? '') };
	}
	if (request.method === 'GET' && request.pathname === '/runs/api/inbound') {
		return { status: 200, body: devInboundMessages(state.taskRuns) };
	}
	if (request.method === 'GET' && request.pathname === '/runs/api/turn-input') {
		return { status: 200, body: devTurnInput() };
	}
	if (request.method === 'DELETE' && request.pathname.startsWith('/runs/api/')) {
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
	if (method === 'GET' && pathname === '/runs/api') return true;
	if (method === 'GET' && pathname === '/runs/api/detail') return true;
	if (method === 'GET' && pathname === '/runs/api/llm-call') return true;
	if (method === 'GET' && pathname === '/runs/api/inbound') return true;
	if (method === 'GET' && pathname === '/runs/api/turn-input') return true;
	if (method === 'DELETE' && pathname.startsWith('/runs/api/')) return true;
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
	const taskRunID = decodeURIComponent(pathname.slice('/runs/api/'.length));
	const taskRun = state.taskRuns.find((candidate) => candidate.taskRunID === taskRunID);
	if (!taskRun) return { status: 404, body: { error: 'task_run_not_found' } };
	if (!deletableTaskRunStatuses.has(taskRun.status)) {
		return { status: 409, body: { error: 'task_run_not_deletable' } };
	}
	state.taskRuns = state.taskRuns.filter((candidate) => candidate.taskRunID !== taskRunID);
	return { status: 200, body: { status: 'deleted', taskRunID } };
}

function createDevTaskDetail(taskRun: TaskRunSummary): Omit<TaskDetail, 'taskEvents'> & { taskEvents: WireTaskEvent[] } {
	return {
		taskRun: {
			...taskRun,
			updatedAt: secondsAfter(taskRun.createdAt, 11.2),
			result:
				taskRun.status === 'completed'
					? '요청한 작업을 완료했고 게시 URL과 주요 변경 사항을 사용자에게 전달했습니다.'
					: taskRun.result
		},
		taskEvents: createDevTaskEvents(taskRun)
	};
}

function secondsAfter(timestamp: string | undefined, seconds: number): string | undefined {
	if (!timestamp) return undefined;
	return new Date(Date.parse(timestamp) + seconds * 1000).toISOString();
}

function createDevTaskEvents(taskRun: TaskRunSummary): WireTaskEvent[] {
	const at = (seconds: number) => secondsAfter(taskRun.createdAt, seconds);
	const isFailed = taskRun.status === 'failed';
	return [
		taskEvent('task.created', taskRun.prompt ?? '', at(0)),
		taskEvent('llm.call', devDecisionRecord(['dev-message-001']), at(0.8), `${taskRun.taskRunID}-decision`),
		taskEvent('task.turn_input', { $part: '0'.repeat(64) }, at(1.1), `${taskRun.taskRunID}-turn-input`),
		taskEvent('llm.call', {
			kind: 'structured',
			schemaName: 'bluecollar_agent_turn_action',
			provider: 'openrouter',
			model: 'google/gemini-3.1-flash-lite',
			latencyMs: 3465,
			promptBytes: 137154,
			contentBytes: 1125,
			promptTokens: 34170,
			completionTokens: 268,
			totalTokens: 34438,
			costUSD: taskRun.llmCostUSD ?? 0.0089445,
			seed: 1234567
		}, at(4.6), `${taskRun.taskRunID}-turn`),
		taskEvent('agent.action', {
			action: 'continue',
			toolName: 'site_serve',
			toolInput: {
				title: '맛있는 귤 세상',
				slug: 'tasty-tangerine',
				description: '귤 소개 웹사이트'
			},
			reason: '사용자가 사이트 생성을 요청했으므로 사이트 생성 도구를 호출합니다.'
		}, at(4.7)),
		taskEvent('tool.site_serve.requested', {
			observationID: 'obs-006',
			toolName: 'site_serve',
			input: {
				title: '맛있는 귤 세상',
				slug: 'tasty-tangerine',
				audience: '귤을 좋아하는 사람들'
			}
		}, at(4.8)),
		isFailed
			? taskEvent('tool.site_serve.result', {
				observationID: 'obs-012',
				tool: 'site_serve',
				output: { content: '게시 서버가 30초 안에 응답하지 않았습니다.' },
				failure: { kind: 'timeout' }
			}, at(34.8))
			: taskEvent('tool.site_serve.result', {
				observationID: 'obs-012',
				tool: 'site_serve',
				output: {
					data: {
						siteID: '0da25b8c036e2cb7a05e3200',
						slug: 'tangerine-hub',
						publishedURL: 'https://tangerine-hub.example-device.example.test',
						status: 'published',
						tlsStatus: 'active'
					}
				}
			}, at(9.3)),
		taskEvent('agent.action', {
			action: taskRun.status === 'completed' ? 'finish' : 'continue',
			message: taskRun.status === 'completed'
				? '요청하신 작업이 완료되어 게시되었습니다.'
				: '작업을 계속 진행합니다.',
			goalStatus: taskRun.status === 'completed' ? 'satisfied' : 'working',
			goalSatisfied: taskRun.status === 'completed'
		}, at(11.2))
	];
}

type WireTaskEvent = Omit<TaskEvent, 'id'> & { taskEventID?: string };

function taskEvent(name: string, body: unknown, createdAt?: string, taskEventID?: string): WireTaskEvent {
	return {
		taskEventID,
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
				`2026-06-25T07:29:04Z task=${taskRunID ?? 'unknown'} tool.site_serve started`,
				`2026-06-25T07:29:05Z task=${taskRunID ?? 'unknown'} publish URL https://tangerine-hub.example-device.example.test`,
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

function devDecisionRecord(decidedMessageIDs: string[]) {
	return {
		kind: 'decision',
		transport: 'decisions',
		model: 'z-ai/glm-5.3-flash',
		latencyMs: 812,
		costUSD: 0.00021,
		decidedMessageIDs,
		decisionAnswers: {
			'm1.target': { type: 'choice', choice: 'room', probabilities: { bot: 0.08, human: 0.21, room: 0.71 } },
			'm1.shouldRespond': { type: 'noul', noul: 0.06 },
			'm1.reaction': { type: 'choice', choice: 'react', probabilities: { react: 0.62, none: 0.38 } },
			'm1.reactionEmoji': { type: 'choice', choice: 'tada', probabilities: { tada: 0.71, thumbsup: 0.22, eyes: 0.07 } },
			'm1.duty': { type: 'choice', choice: 'none', probabilities: { none: 0.97 } }
		},
		decisionDraws: { 'm1.reaction': 0.81 }
	};
}

function devLLMCallExchange(llmCallID: string) {
	if (llmCallID.endsWith('-decision')) {
		return {
			request: JSON.stringify({
				model: 'z-ai/glm-5.3-flash',
				state: { messages: [{ messageID: 'dev-message-001', senderName: '박예시', prompt: '다음 주 출시 확정됐어요!' }], conversationType: 'channel' },
				questions: { 'm1.target': {}, 'm1.shouldRespond': {}, 'm1.reaction': {}, 'm1.reactionEmoji': {}, 'm1.duty': {} }
			}),
			response: JSON.stringify({ model: 'z-ai/glm-5.3-flash', provider: 'Example', answers: {} }),
			input: JSON.stringify({ conversationType: 'channel', messages: [{ id: 'dev-message-001', senderName: '박예시', text: '다음 주 출시 확정됐어요!' }] })
		};
	}
	return {
		request: JSON.stringify({
			model: 'google/gemini-3.1-flash-lite',
			messages: [
				{ role: 'system', content: '당신은 회사의 에이전트 김인턴입니다. 요청을 끝까지 처리하고, 한 일을 증거로 남깁니다.' },
				{ role: 'user', content: [{ type: 'text', text: '귤 소개 사이트 만들어줘' }, { type: 'image_url', image_url: { url: 'data:image/png;base64,iVBORw0KGgo' } }] }
			],
			tools: ['site_serve', 'file_read', 'file_write', 'message_send'].map((name) => ({ type: 'function', function: { name, parameters: {} } })),
			response_format: { type: 'json_schema', json_schema: { name: 'bluecollar_agent_turn_action', schema: { type: 'object' } } },
			seed: 1234567
		}),
		response: JSON.stringify({
			provider: 'Google',
			choices: [{
				finish_reason: 'tool_calls',
				message: {
					role: 'assistant',
					reasoning: '사이트 생성 요청이므로 site_serve를 호출한다.',
					tool_calls: [{ id: 'call-1', type: 'function', function: { name: 'site_serve', arguments: '{"title":"맛있는 귤 세상","slug":"tasty-tangerine"}' } }]
				}
			}]
		})
	};
}

function devTurnInput() {
	return {
		RequesterName: '이샘플',
		ConversationType: 'dm',
		Prompt: '귤 소개 사이트 만들어줘',
		ToolSet: { toolNames: ['file_read', 'file_write', 'message_send', 'site_serve'] },
		TurnStartedAt: '2026-06-17T01:00:00Z'
	};
}

function devInboundMessages(taskRuns: TaskRunSummary[]) {
	const receivedAt = (minutesAgo: number) => new Date(Date.UTC(2026, 5, 17, 1, 0, 0) - minutesAgo * 60 * 1000).toISOString();
	return [
		{ externalMessageID: 'dev-message-001', conversationID: 'channel-general', senderName: '박예시', promptPreview: '다음 주 출시 확정됐어요!', connectorStatus: 'succeeded', ingestedAt: receivedAt(2), result: { handled: true, ignored: true, reason: 'addressing_room dutyMatch=false' }, decision: devDecisionRecord(['dev-message-001']) },
		{ externalMessageID: 'dev-message-002', conversationID: 'channel-general', senderName: '이샘플', promptPreview: '@김인턴 회의록 정리해줘', connectorStatus: 'succeeded', ingestedAt: receivedAt(5), result: { handled: true, taskRunID: taskRuns[0]?.taskRunID } },
		{ externalMessageID: 'dev-message-003', conversationID: 'channel-general', senderName: '최견본', promptPreview: '다들 고생 많았어요, 오늘 회식 있습니다', connectorStatus: 'succeeded', ingestedAt: receivedAt(9), result: { handled: true, ignored: true, reason: 'addressing_react_only' }, decision: { ...devDecisionRecord(['dev-message-003']), decisionDraws: { 'm1.reaction': 0.12 } } },
		{ externalMessageID: 'dev-message-004', conversationID: 'channel-general', senderName: '박예시', promptPreview: '점심 뭐 먹지', connectorStatus: 'pending', ingestedAt: receivedAt(12), result: {} }
	];
}
