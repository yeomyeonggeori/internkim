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
import type { CircleRecord, OrgGroup, UserRecord, UserRole, UsersResponse } from './src/routes/admin/admin-types';

type DevAdminUsersMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevAdminUsersMockState = DevAdminMockState & {
	records: UserRecord[];
	availableCircles: CircleRecord[];
	availableGroups: OrgGroup[];
};

const defaultRole: UserRole = 'member';

export function devAdminUsersMockPlugin(options: DevAdminUsersMockPluginOptions): Plugin {
	const state = createDevAdminUsersMockState(options.userEmail);

	return {
		name: 'internkim-dev-admin-users-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevAdminUsersMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevAdminUsersMockResponse(state, {
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

function createDevAdminUsersMockState(userEmail: string): DevAdminUsersMockState {
	return {
		...createDevAdminMockState(userEmail),
		records: createDevUserRecords(),
		availableCircles: createDevCircles(),
		availableGroups: createDevGroups()
	};
}

function createDevAdminUsersMockResponse(
	state: DevAdminUsersMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/admin/api/health') {
		return { status: 200, body: { ok: true } };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/users') {
		return { status: 200, body: usersResponse(state) };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/updates/status') {
		return { status: 200, body: { state: 'idle', updateAllowed: false } };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/updates/releases') {
		return { status: 200, body: { entries: [] } };
	}
	if (request.method === 'POST' && request.pathname === '/admin/api/users') {
		return saveUserResponse(state, request.body);
	}
	if (request.method === 'POST' && request.pathname === '/admin/api/users/batch') {
		return { status: 200, body: usersResponse(state) };
	}
	if (request.method === 'POST' && request.pathname === '/admin/api/circles') {
		return createCircleResponse(state, request.body);
	}
	if (request.method === 'DELETE' && request.pathname.startsWith('/admin/api/circles/')) {
		return deleteCircleResponse(state, request.pathname);
	}
	if (request.method === 'POST' && request.pathname.endsWith('/password-reset')) {
		const email = decodeURIComponent(request.pathname.replace('/admin/api/users/', '').replace('/password-reset', ''));
		return { status: 200, body: { ...usersResponse(state), temporaryPasswordEmail: email, temporaryPassword: 'temp-pass-1234' } };
	}
	return createDevAdminMockResponse(state, request);
}

function shouldHandleDevAdminUsersMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/admin/api/health') return true;
	if (method === 'GET' && pathname === '/admin/api/users') return true;
	if (method === 'GET' && pathname === '/admin/api/updates/status') return true;
	if (method === 'GET' && pathname === '/admin/api/updates/releases') return true;
	if (method === 'POST' && pathname === '/admin/api/users') return true;
	if (method === 'POST' && pathname === '/admin/api/users/batch') return true;
	if (method === 'POST' && pathname === '/admin/api/circles') return true;
	if (method === 'DELETE' && pathname.startsWith('/admin/api/circles/')) return true;
	if (method === 'POST' && pathname.startsWith('/admin/api/users/') && pathname.endsWith('/password-reset')) return true;
	return shouldHandleDevAdminMockRequest(method, pathname);
}

function usersResponse(state: DevAdminUsersMockState): UsersResponse {
	return {
		records: state.records,
		users: state.records.map((record) => record.email),
		availableCircles: state.availableCircles,
		availableGroups: state.availableGroups
	};
}

function saveUserResponse(state: DevAdminUsersMockState, body: string | undefined): DevMockResponse {
	const parsed = parseJSONRecord(body);
	const email = stringField(parsed, 'email').toLowerCase();
	const handle = stringField(parsed, 'handle').toLowerCase();
	const name = stringField(parsed, 'name');
	if (!email || !handle || !name) {
		return { status: 400, body: { error: 'invalid_user' } };
	}
	const existingRecord = state.records.find((record) => record.email.toLowerCase() === email);
	const nextRecord = normalizeUserRecord(parsed, existingRecord);
	state.records = existingRecord
		? state.records.map((record) => record.email.toLowerCase() === email ? nextRecord : record)
		: [...state.records, nextRecord];
	return { status: 200, body: usersResponse(state) };
}

function createCircleResponse(state: DevAdminUsersMockState, body: string | undefined): DevMockResponse {
	const parsed = parseJSONRecord(body);
	const circleID = stringField(parsed, 'circleID').toLowerCase();
	if (!circleID) return { status: 400, body: { error: 'invalid_circle' } };
	const displayName = stringField(parsed, 'displayName') || circleID;
	const nextCircle = { circleID, displayName, isMattermostManaged: booleanField(parsed, 'isMattermostManaged', true) };
	state.availableCircles = [
		...state.availableCircles.filter((circle) => circle.circleID !== circleID),
		nextCircle
	];
	return { status: 200, body: usersResponse(state) };
}

function deleteCircleResponse(state: DevAdminUsersMockState, pathname: string): DevMockResponse {
	const circleID = decodeURIComponent(pathname.slice('/admin/api/circles/'.length)).toLowerCase();
	if (circleID === 'staff') return { status: 409, body: { error: 'staff_circle_required' } };
	state.availableCircles = state.availableCircles.filter((circle) => circle.circleID !== circleID);
	state.records = state.records.map((record) => ({
		...record,
		circles: (record.circles ?? []).filter((candidate) => candidate !== circleID)
	}));
	return { status: 200, body: usersResponse(state) };
}

function normalizeUserRecord(parsed: Record<string, unknown>, existingRecord: UserRecord | undefined): UserRecord {
	const role = roleField(parsed, existingRecord?.role ?? defaultRole);
	const circles = stringArrayField(parsed, 'circles', existingRecord?.circles ?? ['staff']);
	return {
		userID: stringField(parsed, 'userID') || existingRecord?.userID || `dev-user-${Date.now()}`,
		handle: stringField(parsed, 'handle').toLowerCase(),
		name: stringField(parsed, 'name'),
		email: stringField(parsed, 'email').toLowerCase(),
		hireDate: stringField(parsed, 'hireDate'),
		note: stringField(parsed, 'note'),
		role,
		circles: normalizeCircles(circles, role),
		jobTitle: existingRecord?.jobTitle,
		group: existingRecord?.group,
		positionLevel: existingRecord?.positionLevel,
		primaryGroupID: existingRecord?.primaryGroupID,
		groupIDs: existingRecord?.groupIDs,
		teamRole: existingRecord?.teamRole,
		employmentStatus: existingRecord?.employmentStatus,
		isOrgchartVisible: existingRecord?.isOrgchartVisible,
		mattermostUserID: stringField(parsed, 'mattermostUserID') || existingRecord?.mattermostUserID,
		mattermostUsername: stringField(parsed, 'mattermostUsername') || existingRecord?.mattermostUsername,
		status: stringField(parsed, 'status') || existingRecord?.status
	};
}

function createDevUserRecords(): UserRecord[] {
	return [
		{
			userID: 'dev-user-mohyeong',
			handle: 'mohyeong',
			name: '최견본',
			email: 'mohyeong@example.com',
			hireDate: '2026-05-01',
			note: 'HR 보상 기준 확인 필요. C-level 권한과 대표 권한 유지.',
			role: 'admin',
			circles: ['staff', 'admin', 'c-level', 'representative', 'hr-compensation'],
			jobTitle: 'Representative',
			group: 'C-Level',
			positionLevel: 10,
			primaryGroupID: 'c-level',
			groupIDs: ['c-level'],
			teamRole: 'Representative',
			employmentStatus: 'active',
			isOrgchartVisible: true,
			mattermostUserID: 'dev-mm-mohyeong',
			mattermostUsername: 'mohyeong',
			status: 'active'
		}
	];
}

function createDevCircles(): CircleRecord[] {
	return [
		{ circleID: 'staff', displayName: 'Staff' },
		{ circleID: 'c-level', displayName: 'C-level', isMattermostManaged: true },
		{ circleID: 'representative', displayName: 'Representative', isMattermostManaged: true },
		{ circleID: 'hr-compensation', displayName: 'HR Compensation', isMattermostManaged: true }
	];
}

function createDevGroups(): OrgGroup[] {
	return [{ id: 'c-level', name: 'C-Level' }];
}

function normalizeCircles(circles: string[], role: UserRole): string[] {
	const normalizedCircles = new Set(['staff', ...circles.map((circle) => circle.trim().toLowerCase()).filter(Boolean)]);
	if (role === 'admin') normalizedCircles.add('admin');
	return [...normalizedCircles];
}

function stringField(record: Record<string, unknown>, key: string): string {
	const value = record[key];
	return typeof value === 'string' ? value.trim() : '';
}

function booleanField(record: Record<string, unknown>, key: string, fallback: boolean): boolean {
	const value = record[key];
	return typeof value === 'boolean' ? value : fallback;
}

function roleField(record: Record<string, unknown>, fallback: UserRole): UserRole {
	const value = stringField(record, 'role');
	return value === 'admin' || value === 'member' ? value : fallback;
}

function stringArrayField(record: Record<string, unknown>, key: string, fallback: string[]): string[] {
	const value = record[key];
	if (!Array.isArray(value)) return fallback;
	return value.filter((item): item is string => typeof item === 'string');
}
