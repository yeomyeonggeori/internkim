import {
	createDevAdminMockResponse,
	createDevAdminMockState,
	parseJSONRecord,
	shouldHandleDevAdminMockRequest,
	type DevAdminMockState,
	type DevAdminMockUserRole,
	type DevMockRequest,
	type DevMockResponse
} from './dev-admin-mock';
import { createDevAdminOrgchartGroups, createDevAdminOrgchartUsers } from './dev-admin-orgchart-fixture';
import type { OrgProfileUpdate } from './src/routes/admin/admin-api';
import type { OrgGroup, UserRecord, UsersResponse } from './src/routes/admin/admin-types';

export type DevAdminOrgchartMockState = DevAdminMockState & {
	groups: OrgGroup[];
	users: UserRecord[];
};

export function createDevAdminOrgchartMockState(userEmail: string, userRole: DevAdminMockUserRole = 'admin'): DevAdminOrgchartMockState {
	return {
		...createDevAdminMockState(userEmail, userRole),
		groups: createDevAdminOrgchartGroups(),
		users: createDevAdminOrgchartUsers(userEmail, userRole)
	};
}

export function createDevAdminOrgchartMockResponse(
	state: DevAdminOrgchartMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/admin/api/session') {
		if (state.userRole === 'member') {
			return { status: 403, body: { error: 'admin access required' } };
		}
		return {
			status: 200,
			body: {
				email: state.userEmail,
				claimedAdminEmail: state.userEmail,
				isAdmin: state.userRole === 'admin',
				role: state.userRole,
				isClaimed: true,
				bootstrapStatus: 'claimed',
				deviceManaged: true,
				mattermostURL: 'https://demo.example.test'
			}
		};
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/users') {
		return { status: 200, body: usersResponse(state) };
	}
	if (request.method === 'GET' && request.pathname === '/orgchart/api/people') {
		return { status: 200, body: publicOrgchartUsersResponse(state) };
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

export function shouldHandleDevAdminOrgchartMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/admin/api/users') return true;
	if (method === 'GET' && pathname === '/orgchart/api/people') return true;
	if (method === 'PUT' && pathname === '/admin/api/org-groups') return true;
	if (method === 'POST' && pathname === '/admin/api/users/org-profiles') return true;
	return shouldHandleDevAdminMockRequest(method, pathname);
}

function usersResponse(state: DevAdminOrgchartMockState): UsersResponse {
	return {
		users: state.users.map((user) => user.email),
		records: state.users.map((user) => ({ ...user, groupIDs: [...(user.groupIDs ?? [])] })),
		availableGroups: state.groups.map((group) => ({ ...group })),
		availableCircles: []
	};
}

function publicOrgchartUsersResponse(state: DevAdminOrgchartMockState): UsersResponse {
	return {
		users: state.users.map((user) => user.email),
		records: state.users.map((user) => ({ ...user, groupIDs: [...(user.groupIDs ?? [])] })),
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
		const parentID = stringFromUnknown(record.parentID).trim();
		return id && name ? [{ id, name, ...(parentID ? { parentID } : {}) }] : [];
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
		normalizedGroups.push({ id, name, ...(group.parentID?.trim() ? { parentID: group.parentID.trim() } : {}) });
	}
	const groupIDs = new Set(normalizedGroups.map((group) => group.id));
	return normalizedGroups.map((group) => ({
		id: group.id,
		name: group.name,
		...(group.parentID && group.parentID !== group.id && groupIDs.has(group.parentID) ? { parentID: group.parentID } : {})
	}));
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
				primaryGroupID: optionalStringFromUnknown(record.primaryGroupID),
				groupIDs: stringArrayFromUnknown(record.groupIDs),
				supervisorID: optionalStringFromUnknown(record.supervisorID)
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
			primaryGroupID,
			groupIDs: profile.groupIDs ?? (primaryGroupID ? [primaryGroupID] : []),
			supervisorID: profile.supervisorID ?? user.supervisorID
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

function stringArrayFromUnknown(value: unknown): string[] | undefined {
	if (!Array.isArray(value)) return undefined;
	return [...new Set(value.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean))];
}
