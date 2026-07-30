import { devFlowSizes, devFlowSizesEnglish } from './src/routes/flow/dev-flow-fixture-data';
import { devLocale, setDevLocale } from './dev-locale-state';
import type { Plugin } from 'vite';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { createFlowTaskBoardMove, type FlowTaskBoardMoveRequest } from './src/routes/flow/flow-task-board-drag';
import { isFlowTaskBoardStatus } from './src/routes/flow/flow-task-board-model';
import { createDevFlowState, createDevFlowWeeklySummary } from './src/routes/flow/dev-flow-fixture';
import type { FlowDefinitions, FlowSizeDefinition, FlowState, FlowTask } from './src/routes/flow/flow-types';

type DevFlowMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

const statusRankStep = 1024;

type DevFlowMockState = {
	userEmail: string;
	flowState: FlowState;
	nextTaskID: number;
};

type DevFlowMockRequest = {
	method: string;
	pathname: string;
	searchParams: URLSearchParams;
	body?: string;
};

type DevFlowMockResponse = {
	status: number;
	body: unknown;
};

export function devFlowMockPlugin(options: DevFlowMockPluginOptions): Plugin {
	const state = createDevFlowMockState(options.userEmail);

	return {
		name: 'internkim-dev-flow-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevFlowMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, async (body) => {
					const mockResponse = await createDevFlowMockResponse(state, {
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

function shouldHandleDevFlowMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/flow/api/summary') return true;
	if (method === 'GET' && pathname === '/flow/api/state') return true;
	if (method === 'POST' && pathname === '/flow/api/test/reset') return true;
	if (method === 'POST' && pathname === '/flow/api/tasks/move') return true;
	if (method === 'POST' && pathname === '/flow/api/tasks') return true;
	if (method === 'PUT' && pathname.startsWith('/flow/api/tasks/')) return true;
	if (method === 'DELETE' && pathname.startsWith('/flow/api/tasks/')) return true;
	if (method === 'PUT' && pathname === '/flow/api/definitions') return true;
	if (method === 'GET' && pathname === '/auth/session') return true;
	if (method === 'GET' && pathname === '/admin/api/session') return true;
	if (method === 'GET' && pathname === '/admin/api/locale') return true;
	return method === 'PUT' && pathname === '/admin/api/locale';
}

export function createDevFlowMockState(userEmail: string): DevFlowMockState {
	return {
		userEmail,
		flowState: createDevFlowState(userEmail, devLocale()),
		nextTaskID: 1
	};
}

export async function createDevFlowMockResponse(
	state: DevFlowMockState,
	request: DevFlowMockRequest
): Promise<DevFlowMockResponse | undefined> {
	if (request.method === 'GET' && request.pathname === '/flow/api/summary') {
		return { status: 200, body: createDevFlowWeeklySummary(request.searchParams.get('week')) };
	}
	if (request.method === 'GET' && request.pathname === '/flow/api/state') {
		return { status: 200, body: { ...state.flowState, definitions: localizedDefinitions(state.flowState.definitions) } };
	}
	if (request.method === 'POST' && request.pathname === '/flow/api/test/reset') {
		resetDevFlowMockState(state);
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'POST' && request.pathname === '/flow/api/tasks') {
		const parsed = parseJSONRecord(request.body);
		const task = flowTaskFromRecord(parsed, createFlowTaskFallback(state));
		const statusRank = typeof parsed.statusRank === 'number'
			? task.statusRank
			: nextBottomStatusRank(state.flowState.tasks, task.status);
		state.flowState.tasks = [...state.flowState.tasks, { ...task, statusRank }];
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'POST' && request.pathname === '/flow/api/tasks/move') {
		return createFlowTaskBoardMoveMockResponse(state, parseJSONRecord(request.body));
	}
	if (request.method === 'PUT' && request.pathname.startsWith('/flow/api/tasks/')) {
		const taskID = decodeURIComponent(request.pathname.slice('/flow/api/tasks/'.length));
		const existingTask = state.flowState.tasks.find((task) => task.id === taskID);
		if (!existingTask) return { status: 404, body: { error: 'task not found' } };
		const updatedTask = flowTaskFromRecord(parseJSONRecord(request.body), existingTask);
		state.flowState.tasks = state.flowState.tasks.map((task) =>
			task.id === taskID ? { ...updatedTask, id: taskID } : task
		);
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'DELETE' && request.pathname.startsWith('/flow/api/tasks/')) {
		const taskID = decodeURIComponent(request.pathname.slice('/flow/api/tasks/'.length));
		const hasTask = state.flowState.tasks.some((task) => task.id === taskID);
		if (!hasTask) return { status: 404, body: { error: 'task not found' } };
		state.flowState.tasks = state.flowState.tasks.filter((task) => task.id !== taskID);
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'PUT' && request.pathname === '/flow/api/definitions') {
		state.flowState.definitions = flowDefinitionsFromRecord(parseJSONRecord(request.body), state.flowState.definitions);
		return { status: 200, body: state.flowState.definitions };
	}
	if (request.method === 'GET' && request.pathname === '/auth/session') {
		return { status: 200, body: { authenticated: true, email: state.userEmail, isAdmin: true } };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/session') {
		return {
			status: 200,
			body: {
				email: state.userEmail,
				claimedAdminEmail: state.userEmail,
				isAdmin: true,
				isClaimed: true,
				bootstrapStatus: 'claimed'
			}
		};
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/locale') {
		return { status: 200, body: { locale: devLocale() } };
	}
	if (request.method === 'PUT' && request.pathname === '/admin/api/locale') {
		setDevLocale(localeFromBody(request.body));
		return { status: 200, body: { locale: devLocale() } };
	}
	return undefined;
}

function localizedDefinitions(definitions: FlowDefinitions): FlowDefinitions {
	return { ...definitions, sizes: devLocale() === 'en' ? devFlowSizesEnglish : devFlowSizes };
}

function resetDevFlowMockState(state: DevFlowMockState): void {
	state.flowState = createDevFlowState(state.userEmail, devLocale());
	state.nextTaskID = 1;
}

function createFlowTaskFallback(state: DevFlowMockState): FlowTask {
	const firstMember = state.flowState.members[0];
	const firstSize = state.flowState.definitions.sizes[0];
	return {
		id: `dev-flow-task-${state.nextTaskID++}`,
		ownerID: firstMember?.id ?? '',
		ownerName: firstMember?.name ?? '',
		participantIDs: firstMember ? [firstMember.id] : [],
		participantNames: firstMember ? [firstMember.name] : [],
		business: state.flowState.definitions.categories[0] ?? '',
		type: state.flowState.definitions.types[0] ?? '',
		content: '',
		goal: '',
		size: firstSize?.name ?? '',
		status: state.flowState.statusOptions[0] ?? '요청',
		statusRank: statusRankStep,
		weekCode: state.flowState.currentWeek?.code ?? '',
		flag: 0,
		createdAt: new Date().toISOString()
	};
}

function flowTaskFromRecord(parsed: Record<string, unknown>, fallback: FlowTask): FlowTask {
	return {
		...fallback,
		id: nonEmptyStringFromValue(parsed.id, fallback.id),
		ownerID: stringFromValue(parsed.ownerID, fallback.ownerID),
		ownerName: stringFromValue(parsed.ownerName, fallback.ownerName),
		participantIDs: stringArrayFromValue(parsed.participantIDs, fallback.participantIDs),
		participantNames: stringArrayFromValue(parsed.participantNames, fallback.participantNames),
		business: stringFromValue(parsed.business, fallback.business),
		type: stringFromValue(parsed.type, fallback.type),
		content: stringFromValue(parsed.content, fallback.content),
		goal: stringFromValue(parsed.goal, fallback.goal),
		size: stringFromValue(parsed.size, fallback.size),
		status: stringFromValue(parsed.status, fallback.status),
		statusRank: numberFromValue(parsed.statusRank, fallback.statusRank),
		startDate: optionalStringFromValue(parsed.startDate, fallback.startDate),
		endDate: optionalStringFromValue(parsed.endDate, fallback.endDate),
		createdAt: optionalStringFromValue(parsed.createdAt, fallback.createdAt),
		weekCode: stringFromValue(parsed.weekCode, fallback.weekCode),
		flag: numberFromValue(parsed.flag, fallback.flag),
		requestReason: optionalStringFromValue(parsed.requestReason, fallback.requestReason),
		decisionReason: optionalStringFromValue(parsed.decisionReason, fallback.decisionReason)
	};
}

function flowDefinitionsFromRecord(parsed: Record<string, unknown>, fallback: FlowDefinitions): FlowDefinitions {
	return {
		categories: stringArrayFromValue(parsed.categories, fallback.categories),
		categoryColors: colorMapFromValue(parsed.categoryColors, fallback.categoryColors),
		types: stringArrayFromValue(parsed.types, fallback.types),
		typeColors: colorMapFromValue(parsed.typeColors, fallback.typeColors),
		sizes: flowSizeDefinitionsFromValue(parsed.sizes, fallback.sizes)
	};
}

function flowSizeDefinitionsFromValue(value: unknown, fallback: FlowSizeDefinition[]): FlowSizeDefinition[] {
	if (!Array.isArray(value)) return fallback;
	const sizes = value.map((item, index) =>
		flowSizeDefinitionFromRecord(isUnknownRecord(item) ? item : {}, fallback[index])
	).filter((size): size is FlowSizeDefinition => size !== null);
	return sizes.length > 0 ? sizes : fallback;
}

function flowSizeDefinitionFromRecord(
	parsed: Record<string, unknown>,
	fallback: FlowSizeDefinition | undefined
): FlowSizeDefinition | null {
	const name = nonEmptyStringFromValue(parsed.name, fallback?.name ?? '');
	if (!name) return null;
	const distanceKm = Math.max(1, numberFromValue(parsed.distanceKm, fallback?.distanceKm ?? 1));
	const maxHours = Math.max(1, numberFromValue(parsed.maxHours, fallback?.maxHours ?? 1));
	return {
		name,
		distanceKm,
		maxHours,
		score: Math.max(1, numberFromValue(parsed.score, distanceKm)),
		label: stringFromValue(parsed.label, `${distanceKm}km · ${maxHours}h`),
		developmentExample: stringFromValue(parsed.developmentExample, fallback?.developmentExample ?? ''),
		otherExample: stringFromValue(parsed.otherExample, fallback?.otherExample ?? ''),
		note: stringFromValue(parsed.note, fallback?.note ?? '')
	};
}

function colorMapFromValue(value: unknown, fallback: Record<string, string> | undefined): Record<string, string> {
	if (!isUnknownRecord(value)) return fallback ?? {};
	return Object.fromEntries(
		Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string')
	);
}

function createFlowTaskBoardMoveMockResponse(
	state: DevFlowMockState,
	parsed: Record<string, unknown>
): DevFlowMockResponse {
	const moveRequest = flowTaskBoardMoveRequestFromRecord(parsed);
	const validationResponse = validateFlowTaskBoardMoveMockRequest(state, moveRequest);
	if (validationResponse) return validationResponse;

	const move = createFlowTaskBoardMove(state.flowState.tasks, moveRequest);
	if (!move) return { status: 200, body: { ok: true } };
	state.flowState.tasks = move.tasks;
	return { status: 200, body: { ok: true } };
}

function flowTaskBoardMoveRequestFromRecord(parsed: Record<string, unknown>): FlowTaskBoardMoveRequest {
	return {
		taskID: trimmedStringFromValue(parsed.taskID),
		targetStatus: trimmedStringFromValue(parsed.targetStatus),
		beforeTaskID: nullableStringFromValue(parsed.beforeTaskID)
	};
}

function validateFlowTaskBoardMoveMockRequest(
	state: DevFlowMockState,
	moveRequest: FlowTaskBoardMoveRequest
): DevFlowMockResponse | null {
	if (moveRequest.taskID === '') {
		return { status: 400, body: { error: 'task id is required' } };
	}
	if (!isFlowTaskBoardStatus(moveRequest.targetStatus)) {
		return { status: 400, body: { error: 'target status is not movable on the board' } };
	}
	if (moveRequest.beforeTaskID === moveRequest.taskID) {
		return { status: 400, body: { error: 'before task cannot be the moved task' } };
	}
	if (!state.flowState.tasks.some((task) => task.id === moveRequest.taskID)) {
		return { status: 404, body: { error: 'task not found' } };
	}
	if (
		moveRequest.beforeTaskID
		&& !state.flowState.tasks.some((task) =>
			task.id === moveRequest.beforeTaskID && task.status === moveRequest.targetStatus
		)
	) {
		return { status: 400, body: { error: 'before task is not in target status' } };
	}
	return null;
}

function nextBottomStatusRank(tasks: FlowTask[], status: string): number {
	const ranks = tasks.filter((task) => task.status === status).map((task) => task.statusRank);
	return ranks.length === 0 ? statusRankStep : Math.max(...ranks) + statusRankStep;
}

function localeFromBody(body: string | undefined): 'ko' | 'en' {
	const parsed = parseJSONRecord(body);
	return parsed.locale === 'en' ? 'en' : 'ko';
}

function parseJSONRecord(body: string | undefined): Record<string, unknown> {
	if (!body) return {};
	try {
		const parsed: unknown = JSON.parse(body);
		if (isUnknownRecord(parsed)) return parsed;
	} catch {
		return {};
	}
	return {};
}

function isUnknownRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringFromValue(value: unknown, fallback: string): string {
	return typeof value === 'string' ? value : fallback;
}

function trimmedStringFromValue(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

function nonEmptyStringFromValue(value: unknown, fallback: string): string {
	if (typeof value !== 'string') return fallback;
	const trimmed = value.trim();
	return trimmed ? trimmed : fallback;
}

function optionalStringFromValue(value: unknown, fallback: string | undefined): string | undefined {
	return typeof value === 'string' ? value : fallback;
}

function nullableStringFromValue(value: unknown): string | null {
	if (typeof value !== 'string') return null;
	const trimmedValue = value.trim();
	return trimmedValue ? trimmedValue : null;
}

function stringArrayFromValue(value: unknown, fallback: string[]): string[] {
	if (!Array.isArray(value)) return fallback;
	return value.filter((item): item is string => typeof item === 'string');
}

function numberFromValue(value: unknown, fallback: number): number {
	return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function readRequestBody(request: IncomingMessage, callback: (body: string) => void) {
	if (request.method === 'GET') {
		callback('');
		return;
	}
	const contentLength = request.headers['content-length'];
	if (contentLength === '0' || (!contentLength && !request.headers['transfer-encoding'])) {
		callback('');
		return;
	}
	let body = '';
	request.on('data', (chunk: Buffer) => {
		body += chunk.toString('utf8');
	});
	request.on('end', () => callback(body));
	request.on('error', () => callback(''));
}

function writeJSON(response: ServerResponse, status: number, body: unknown) {
	response.statusCode = status;
	response.setHeader('Content-Type', 'application/json');
	response.end(JSON.stringify(body));
}
