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
import type { TaskRunSummary } from './src/routes/tasks/tasks-api';

type DevTasksMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevTasksMockState = DevAdminMockState & {
	taskRuns: TaskRunSummary[];
};

const taskRunStatuses = ['completed', 'completed', 'completed', 'failed', 'running', 'waiting_approval', 'blocked'] as const;

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
	if (request.method === 'GET' && request.pathname === '/admin/api/diagnostics/tasks') {
		return { status: 200, body: paginatedTaskRuns(state.taskRuns, request.searchParams) };
	}
	return createDevAdminMockResponse(state, request);
}

function shouldHandleDevTasksMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/admin/api/diagnostics/tasks') return true;
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
			createdAt: updatedAt,
			updatedAt
		};
	});
}

function paginatedTaskRuns(taskRuns: TaskRunSummary[], searchParams: URLSearchParams): TaskRunSummary[] | { taskRuns: TaskRunSummary[]; totalCount: number } {
	const status = searchParams.get('status')?.trim();
	const filteredTaskRuns = status ? taskRuns.filter((taskRun) => taskRun.status === status) : taskRuns;
	const offset = positiveIntegerFromSearch(searchParams, 'offset', 0);
	const limit = positiveIntegerFromSearch(searchParams, 'limit', filteredTaskRuns.length);
	const selectedTaskRuns = filteredTaskRuns.slice(offset, offset + limit);
	if (searchParams.get('includeTotal') === 'true') {
		return { taskRuns: selectedTaskRuns, totalCount: filteredTaskRuns.length };
	}
	return selectedTaskRuns;
}

function positiveIntegerFromSearch(searchParams: URLSearchParams, key: string, fallback: number): number {
	const value = Number(searchParams.get(key));
	if (!Number.isFinite(value) || value < 0) return fallback;
	return Math.floor(value);
}
