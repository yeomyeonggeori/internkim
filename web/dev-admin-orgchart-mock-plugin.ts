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
import type { OrgProfileUpdate } from './src/routes/admin/admin-api';
import type { OrgGroup, OrgchartEmploymentStatus, UserRecord, UsersResponse } from './src/routes/admin/admin-types';

type DevAdminOrgchartMockPluginOptions = {
	isEnabled: boolean;
	userEmail: string;
};

type DevAdminOrgchartMockState = DevAdminMockState & {
	groups: OrgGroup[];
	users: UserRecord[];
};

export function devAdminOrgchartMockPlugin(options: DevAdminOrgchartMockPluginOptions): Plugin {
	const state = createDevAdminOrgchartMockState(options.userEmail);

	return {
		name: 'internkim-dev-admin-orgchart-mock',
		configureServer(server) {
			if (!options.isEnabled) return;

			server.middlewares.use((request, response, next) => {
				const requestURL = new URL(request.url ?? '/', 'http://localhost');
				const method = request.method ?? 'GET';
				if (!shouldHandleDevAdminOrgchartMockRequest(method, requestURL.pathname)) {
					next();
					return;
				}
				readRequestBody(request, (body) => {
					const mockResponse = createDevAdminOrgchartMockResponse(state, {
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

export function createDevAdminOrgchartMockState(userEmail: string): DevAdminOrgchartMockState {
	return {
		...createDevAdminMockState(userEmail),
		groups: createDevAdminOrgchartGroups(),
		users: createDevAdminOrgchartUsers(userEmail)
	};
}

export function createDevAdminOrgchartMockResponse(
	state: DevAdminOrgchartMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/admin/api/session') {
		return {
			status: 200,
			body: {
				email: state.userEmail,
				claimedAdminEmail: state.userEmail,
				isAdmin: true,
				isClaimed: true,
				bootstrapStatus: 'claimed',
				deviceManaged: true,
				mattermostURL: 'https://demo.intern.kim'
			}
		};
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/users') {
		return { status: 200, body: usersResponse(state) };
	}
	if (request.method === 'PUT' && request.pathname === '/admin/api/org-groups') {
		state.groups = normalizeOrgGroups(groupsFromBody(request.body));
		return { status: 200, body: usersResponse(state) };
	}
	if (request.method === 'POST' && request.pathname === '/admin/api/users/org-profiles') {
		state.users = applyOrgProfileUpdates(state.users, profilesFromBody(request.body));
		return { status: 200, body: usersResponse(state) };
	}
	return createDevAdminMockResponse(state, request);
}

function shouldHandleDevAdminOrgchartMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/admin/api/users') return true;
	if (method === 'PUT' && pathname === '/admin/api/org-groups') return true;
	if (method === 'POST' && pathname === '/admin/api/users/org-profiles') return true;
	return shouldHandleDevAdminMockRequest(method, pathname);
}

function createDevAdminOrgchartGroups(): OrgGroup[] {
	return [
		{ id: 'group-leadership', name: 'Leadership' },
		{ id: 'group-engineering', name: 'Engineering' },
		{ id: 'group-operations', name: 'Operations' }
	];
}

function createDevAdminOrgchartUsers(userEmail: string): UserRecord[] {
	return [
		{
			userID: 'dev-user-ada',
			handle: 'ada',
			name: '김인턴',
			email: userEmail,
			hireDate: '2026-01-03',
			role: 'admin',
			jobTitle: 'Founder',
			group: 'group-leadership',
			positionLevel: 1,
			primaryGroupID: 'group-leadership',
			groupIDs: ['group-leadership'],
			projectIDs: ['blueclaw'],
			teamRole: 'owner',
			employmentStatus: 'active',
			isOrgchartVisible: true,
			status: 'active'
		},
		{
			userID: 'dev-user-grace',
			handle: 'grace',
			name: '이지원',
			email: 'grace@example.com',
			hireDate: '2026-02-10',
			role: 'member',
			jobTitle: 'Engineer',
			group: 'group-engineering',
			positionLevel: 2,
			primaryGroupID: 'group-engineering',
			groupIDs: ['group-engineering'],
			supervisorID: 'dev-user-ada',
			projectIDs: ['blueclaw', 'admin'],
			teamRole: 'frontend',
			employmentStatus: 'active',
			isOrgchartVisible: true,
			status: 'active'
		},
		{
			userID: 'dev-user-min',
			handle: 'min',
			name: '박민수',
			email: 'min@example.com',
			hireDate: '2026-03-15',
			role: 'member',
			jobTitle: 'Operations Manager',
			group: 'group-operations',
			positionLevel: 3,
			primaryGroupID: 'group-operations',
			groupIDs: ['group-operations'],
			supervisorID: 'dev-user-ada',
			projectIDs: ['ops'],
			teamRole: 'operations',
			employmentStatus: 'active',
			isOrgchartVisible: true,
			status: 'active'
		}
	];
}

function usersResponse(state: DevAdminOrgchartMockState): UsersResponse {
	return {
		users: state.users.map((user) => user.email),
		records: state.users.map((user) => ({ ...user, groupIDs: [...(user.groupIDs ?? [])], projectIDs: [...(user.projectIDs ?? [])] })),
		availableGroups: state.groups.map((group) => ({ ...group })),
		availableCircles: []
	};
}

function groupsFromBody(body: string | undefined): OrgGroup[] {
	const groups = parseJSONRecord(body).groups;
	if (!Array.isArray(groups)) return [];
	return groups.flatMap((group) => {
		const record = recordFromUnknown(group);
		if (!record) return [];
		const id = stringFromUnknown(record.id).trim();
		const name = stringFromUnknown(record.name).trim();
		return id && name ? [{ id, name }] : [];
	});
}

function normalizeOrgGroups(groups: OrgGroup[]): OrgGroup[] {
	const seenIDs = new Set<string>();
	const seenNames = new Set<string>();
	const normalizedGroups: OrgGroup[] = [];
	for (const group of groups) {
		const id = group.id.trim();
		const name = group.name.trim();
		const normalizedName = name.toLowerCase();
		if (!id || !name || seenIDs.has(id) || seenNames.has(normalizedName)) continue;
		seenIDs.add(id);
		seenNames.add(normalizedName);
		normalizedGroups.push({ id, name });
	}
	return normalizedGroups;
}

function profilesFromBody(body: string | undefined): OrgProfileUpdate[] {
	const profiles = parseJSONRecord(body).profiles;
	if (!Array.isArray(profiles)) return [];
	return profiles.flatMap((profile) => {
		const record = recordFromUnknown(profile);
		if (!record) return [];
		const userID = stringFromUnknown(record.userID).trim();
		const email = stringFromUnknown(record.email).trim();
		if (!userID && !email) return [];
		return [
			{
				userID,
				email,
				jobTitle: stringFromUnknown(record.jobTitle),
				group: optionalStringFromUnknown(record.group),
				positionLevel: optionalNumberFromUnknown(record.positionLevel),
				primaryGroupID: optionalStringFromUnknown(record.primaryGroupID),
				groupIDs: stringArrayFromUnknown(record.groupIDs),
				supervisorID: optionalStringFromUnknown(record.supervisorID),
				projectIDs: stringArrayFromUnknown(record.projectIDs),
				teamRole: optionalStringFromUnknown(record.teamRole),
				employmentStatus: employmentStatusFromUnknown(record.employmentStatus),
				isOrgchartVisible: optionalBooleanFromUnknown(record.isOrgchartVisible)
			}
		];
	});
}

function applyOrgProfileUpdates(users: UserRecord[], profiles: OrgProfileUpdate[]): UserRecord[] {
	return users.map((user) => {
		const profile = profiles.find((candidate) => candidate.userID === user.userID || candidate.email === user.email);
		if (!profile) return user;
		const primaryGroupID = profile.primaryGroupID ?? profile.group ?? user.primaryGroupID ?? user.group ?? '';
		return {
			...user,
			jobTitle: profile.jobTitle,
			group: primaryGroupID,
			positionLevel: profile.positionLevel,
			primaryGroupID,
			groupIDs: profile.groupIDs ?? (primaryGroupID ? [primaryGroupID] : []),
			supervisorID: profile.supervisorID,
			projectIDs: profile.projectIDs,
			teamRole: profile.teamRole,
			employmentStatus: profile.employmentStatus,
			isOrgchartVisible: profile.isOrgchartVisible
		};
	});
}

function recordFromUnknown(value: unknown): Record<string, unknown> | undefined {
	if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
	return value as Record<string, unknown>;
}

function stringFromUnknown(value: unknown): string {
	return typeof value === 'string' ? value : '';
}

function optionalStringFromUnknown(value: unknown): string | undefined {
	return typeof value === 'string' ? value : undefined;
}

function optionalNumberFromUnknown(value: unknown): number | undefined {
	return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function optionalBooleanFromUnknown(value: unknown): boolean | undefined {
	return typeof value === 'boolean' ? value : undefined;
}

function stringArrayFromUnknown(value: unknown): string[] | undefined {
	if (!Array.isArray(value)) return undefined;
	return [...new Set(value.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean))];
}

function employmentStatusFromUnknown(value: unknown): OrgchartEmploymentStatus | undefined {
	if (value === 'active' || value === 'leave' || value === 'resigned') return value;
	return undefined;
}
