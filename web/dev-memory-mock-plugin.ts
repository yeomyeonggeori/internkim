import type { Plugin } from 'vite';
import {
	createDevAdminMockResponse,
	createDevAdminMockState,
	parseJSONRecord,
	readRequestBody,
	shouldHandleDevAdminMockRequest,
	writeJSON,
	type DevAdminMockState,
	type DevMockRequest,
	type DevMockResponse
} from './dev-admin-mock';
import type { MemoryGraphResponse } from './src/routes/memory/memory-graph-api';
import type { MemorySchedule } from './src/routes/memory/memory-schedule-api';

type DevMemoryMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevMemoryMockState = DevAdminMockState & {
	schedules: MemorySchedule[];
};

export function devMemoryMockPlugin(options: DevMemoryMockPluginOptions): Plugin {
	const state = createDevMemoryMockState(options.userEmail);

	return {
		name: 'internkim-dev-memory-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevMemoryMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevMemoryMockResponse(state, {
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

export function createDevMemoryMockState(userEmail: string): DevMemoryMockState {
	return {
		...createDevAdminMockState(userEmail),
		schedules: createDevMemorySchedules()
	};
}

export function createDevMemoryMockResponse(
	state: DevMemoryMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/memory/api/schedules') {
		return { status: 200, body: paginatedSchedules(state.schedules, request.searchParams) };
	}
	if (request.method === 'GET' && request.pathname === '/memory/api/graph') {
		return { status: 200, body: createDevMemoryGraph() };
	}
	if (request.method === 'POST' && request.pathname === '/memory/api/schedules/delete') {
		const taskScheduleID = stringFromRecord(request.body, 'taskScheduleID');
		state.schedules = state.schedules.filter((schedule) => schedule.taskScheduleID !== taskScheduleID);
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'POST' && request.pathname === '/memory/api/schedules/cancel') {
		const taskScheduleID = stringFromRecord(request.body, 'taskScheduleID');
		state.schedules = state.schedules.map((schedule) =>
			schedule.taskScheduleID === taskScheduleID ? { ...schedule, nextRunAt: undefined } : schedule
		);
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'POST' && request.pathname === '/memory/api/schedules/update') {
		return { status: 200, body: { ok: true } };
	}
	return createDevAdminMockResponse(state, request);
}

function shouldHandleDevMemoryMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/memory/api/schedules') return true;
	if (method === 'GET' && pathname === '/memory/api/graph') return true;
	if (method === 'POST' && pathname === '/memory/api/schedules/delete') return true;
	if (method === 'POST' && pathname === '/memory/api/schedules/cancel') return true;
	if (method === 'POST' && pathname === '/memory/api/schedules/update') return true;
	return shouldHandleDevAdminMockRequest(method, pathname);
}

function createDevMemorySchedules(): MemorySchedule[] {
	return Array.from({ length: 58 }, (_, index) => {
		const scheduleNumber = index + 1;
		const isInterval = scheduleNumber % 3 === 0;
		const isExpired = scheduleNumber % 8 === 0;
		return {
			taskScheduleID: `dev-schedule-${String(scheduleNumber).padStart(3, '0')}`,
			creatorPersonID: 'dev-person-admin',
			name: `개발 확인 예약 ${scheduleNumber}`,
			executionMode: 'task',
			kind: isInterval ? 'interval' : 'cron',
			intervalSecond: isInterval ? 3600 : undefined,
			cronExpression: isInterval ? undefined : '0 9 * * *',
			maxRunCount: scheduleNumber % 5 === 0 ? 10 : undefined,
			completedRunCount: scheduleNumber % 5 === 0 ? scheduleNumber % 10 : scheduleNumber,
			createdAt: `2026-06-${String(Math.min(28, scheduleNumber)).padStart(2, '0')}T08:00:00+09:00`,
			updatedAt: `2026-06-${String(Math.min(28, scheduleNumber)).padStart(2, '0')}T08:30:00+09:00`,
			nextRunAt: isExpired ? undefined : `2026-07-${String((scheduleNumber % 28) + 1).padStart(2, '0')}T09:00:00+09:00`,
			expiresAt: isExpired ? '2026-06-10T18:00:00+09:00' : undefined,
			failureCount: scheduleNumber % 9,
			promptPreview: `개발 확인용 예약 작업 ${scheduleNumber}번을 실행합니다.`,
			timeZone: 'Asia/Seoul'
		};
	});
}

function paginatedSchedules(schedules: MemorySchedule[], searchParams: URLSearchParams) {
	const includeExpired = searchParams.get('includeExpired') === 'true';
	const pageSize = positiveIntegerFromSearch(searchParams, 'pageSize', 25);
	const page = Math.max(1, positiveIntegerFromSearch(searchParams, 'page', 1));
	const filteredSchedules = includeExpired ? schedules : schedules.filter((schedule) => schedule.nextRunAt);
	const startIndex = (page - 1) * pageSize;
	return {
		schedules: filteredSchedules.slice(startIndex, startIndex + pageSize),
		count: filteredSchedules.length,
		totalCount: filteredSchedules.length,
		page,
		pageSize,
		checkedAt: '2026-06-17T12:00:00+09:00'
	};
}

function createDevMemoryGraph(): MemoryGraphResponse {
	return {
		health: { configured: true, reachable: true },
		namespaces: [{ namespaceID: 'dev-memory', scopeType: 'workspace', episodeCount: 3 }],
		episodes: [
			{
				episodeID: 'dev-episode-1',
				platform: 'mattermost',
				prompt: '개발 확인용 기억을 저장해줘.',
				namespaceIDs: ['dev-memory'],
				ingestionStatus: 'succeeded',
				occurredAt: '2026-06-17T12:00:00+09:00'
			}
		],
		facts: [
			{
				factID: 'dev-fact-1',
				scopeType: 'workspace',
				namespaceID: 'dev-memory',
				content: '개발 확인용 기억입니다.',
				score: 0.92,
				sourceEpisodeID: 'dev-episode-1'
			}
		],
		nodes: [
			{ nodeID: 'namespace:dev-memory', label: 'dev-memory', kind: 'namespace', scopeType: 'workspace' },
			{ nodeID: 'episode:dev-episode-1', label: 'mattermost 06-17 12:00', kind: 'episode', status: 'succeeded' },
			{ nodeID: 'fact:dev-memory:dev-fact-1', label: '개발 확인용 기억입니다.', kind: 'fact', scopeType: 'workspace' }
		],
		edges: [
			{ sourceID: 'namespace:dev-memory', targetID: 'episode:dev-episode-1', weight: 1 },
			{ sourceID: 'namespace:dev-memory', targetID: 'fact:dev-memory:dev-fact-1', weight: 2 }
		]
	};
}

function positiveIntegerFromSearch(searchParams: URLSearchParams, key: string, fallback: number): number {
	const value = Number(searchParams.get(key));
	if (!Number.isFinite(value) || value <= 0) return fallback;
	return Math.floor(value);
}

function stringFromRecord(body: string | undefined, key: string): string {
	const value = parseJSONRecord(body)[key];
	return typeof value === 'string' ? value : '';
}
