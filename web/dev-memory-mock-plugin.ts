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
import type { MemoryFactsResponse } from './src/routes/memory/memory-facts-api';
import type { MemorySchedule } from './src/routes/memory/memory-schedule-api';

type DevMemoryMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevMemoryMockState = DevAdminMockState & {
	schedules: MemorySchedule[];
	forgottenFactIDs: string[];
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
		schedules: createDevMemorySchedules(),
		forgottenFactIDs: []
	};
}

export function createDevMemoryMockResponse(
	state: DevMemoryMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/memory/api/schedules') {
		return { status: 200, body: paginatedSchedules(state.schedules, request.searchParams) };
	}
	if (request.method === 'GET' && request.pathname === '/memory/api/facts') {
		return { status: 200, body: createDevMemoryFacts(state) };
	}
	if (request.method === 'POST' && request.pathname === '/memory/api/facts/forget') {
		const factIDs = readForgottenFactIDs(request.body);
		state.forgottenFactIDs.push(...factIDs);
		return { status: 200, body: { forgottenFactIDs: factIDs } };
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
	if (method === 'GET' && pathname === '/memory/api/facts') return true;
	if (method === 'POST' && pathname === '/memory/api/facts/forget') return true;
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

function createDevMemoryFacts(state: DevMemoryMockState): MemoryFactsResponse {
	const facts = [
		{
			factID: 'dev-fact-1',
			episodeID: 'dev-episode-1',
			ownerPersonID: 'dev-person',
			circleIDs: [],
			kind: 'identity',
			content: '이샘플은 플랫폼 팀 소속이다.',
			validFrom: '2026-06-17T12:00:00+09:00',
			reinforcementCount: 1
		},
		{
			factID: 'dev-fact-2',
			episodeID: 'dev-episode-2',
			ownerPersonID: 'dev-person',
			circleIDs: [],
			kind: 'preference',
			content: '이샘플은 릴리스 노트를 짧은 한국어 문장으로 받는 것을 선호한다.',
			validFrom: '2026-07-01T09:30:00+09:00',
			reinforcementCount: 3
		},
		{
			factID: 'dev-fact-3',
			episodeID: 'dev-episode-3',
			ownerPersonID: 'dev-person',
			circleIDs: ['member'],
			kind: 'fact',
			content: '급여 자료는 HR 서클에서만 다룬다.',
			validFrom: '2026-07-05T14:10:00+09:00',
			reinforcementCount: 1
		},
		{
			factID: 'dev-fact-4',
			episodeID: 'dev-episode-4',
			ownerPersonID: 'dev-colleague',
			circleIDs: ['member'],
			kind: 'fact',
			content: '분기 런치 리뷰는 매주 금요일에 진행된다.',
			validFrom: '2026-06-20T10:00:00+09:00',
			reinforcementCount: 1
		},
		{
			factID: 'dev-fact-5',
			episodeID: 'dev-episode-5',
			ownerPersonID: 'dev-person',
			circleIDs: [],
			kind: 'temporary',
			content: '이샘플은 2026-09-11까지 휴가 중이다.',
			validFrom: '2026-09-01T09:00:00+09:00',
			validUntil: '2026-09-12T00:00:00+09:00',
			reinforcementCount: 1
		}
	] as const;
	return {
		personID: 'dev-person',
		embeddingModel: 'qwen/qwen3-embedding-4b',
		profile: {
			identityLines: ['이샘플은 플랫폼 팀 소속이며 짧은 한국어 릴리스 노트를 선호한다.'],
			currentLines: ['이샘플은 2026-09-11까지 휴가 중이다.'],
			builtAt: '2026-09-01T09:05:00+09:00'
		},
		facts: facts.filter((fact) => !state.forgottenFactIDs.includes(fact.factID)).map((fact) => ({ ...fact, circleIDs: [...fact.circleIDs] }))
	};
}

function readForgottenFactIDs(body: unknown): string[] {
	const record = parseJSONRecord(typeof body === 'string' ? body : undefined);
	const factIDs = record?.factIDs;
	if (!Array.isArray(factIDs)) return [];
	return factIDs.filter((factID): factID is string => typeof factID === 'string');
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
