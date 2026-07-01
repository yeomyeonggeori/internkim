import type {
	AdminJob,
	AdminSession,
	AttendanceLocation,
	AttendanceLocationsResponse,
	BlueclawUpdateStatus,
	BotProfile,
	CircleRecord,
	CredentialProviderStatus,
	CredentialProvidersResponse,
	OrgchartEmploymentStatus,
	OrgGroup,
	ReleaseHistoryResponse,
	RestoreUploadResponse,
	UserRecord,
	UserRole,
	UsersResponse,
	WifiProfilesResponse,
	WorkspaceSettings
} from './admin-types';

export class AdminApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
	}
}

const fallbackAdminApiErrorMessages = new Set([
	'employmentStatus must be active, leave, or resigned',
	'supervisor hierarchy cannot contain cycles'
]);
const fallbackNetworkErrorMessages = new Set(['Failed to fetch', 'Load failed', 'NetworkError when attempting to fetch resource.', 'fetch failed']);

export async function fetchAdminSession(adminBaseURL: string, fallbackMessage: string): Promise<AdminSession> {
	const response = await fetch(`${adminBaseURL}/session`, { credentials: 'include' });
	return readJSON<AdminSession>(response, fallbackMessage);
}

export async function fetchBotProfile(adminBaseURL: string, fallbackMessage: string): Promise<BotProfile> {
	const response = await fetch(`${adminBaseURL}/bot-profile`, { credentials: 'include' });
	return readJSON<BotProfile>(response, fallbackMessage);
}

export async function updateBotProfile(adminBaseURL: string, profile: BotProfile, fallbackMessage: string): Promise<BotProfile> {
	const response = await fetch(`${adminBaseURL}/bot-profile`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(profile)
	});
	return readJSON<BotProfile>(response, fallbackMessage);
}

export async function fetchCredentialProviders(adminBaseURL: string, fallbackMessage: string): Promise<CredentialProvidersResponse> {
	const response = await fetch(`${adminBaseURL}/credentials/providers`, { credentials: 'include' });
	return readJSON<CredentialProvidersResponse>(response, fallbackMessage);
}

export async function saveOpenRouterCredential(adminBaseURL: string, apiKey: string, fallbackMessage: string): Promise<CredentialProviderStatus> {
	const response = await fetch(`${adminBaseURL}/credentials/openrouter-key`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ apiKey })
	});
	return readJSON<CredentialProviderStatus>(response, fallbackMessage);
}

export async function deleteOpenRouterCredential(adminBaseURL: string, fallbackMessage: string): Promise<CredentialProviderStatus> {
	const response = await fetch(`${adminBaseURL}/credentials/openrouter-key`, {
		method: 'DELETE',
		credentials: 'include'
	});
	return readJSON<CredentialProviderStatus>(response, fallbackMessage);
}

export async function fetchWorkspaceSettings(adminBaseURL: string, fallbackMessage: string): Promise<WorkspaceSettings> {
	const response = await fetch(`${adminBaseURL}/workspace-settings`, { credentials: 'include' });
	return readJSON<WorkspaceSettings>(response, fallbackMessage);
}

export async function updateWorkspaceSettings(adminBaseURL: string, settings: WorkspaceSettings, fallbackMessage: string): Promise<WorkspaceSettings> {
	const response = await fetch(`${adminBaseURL}/workspace-settings`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(settings)
	});
	return readJSON<WorkspaceSettings>(response, fallbackMessage);
}

export async function fetchAttendanceLocations(adminBaseURL: string, fallbackMessage: string): Promise<AttendanceLocationsResponse> {
	const response = await fetch(`${adminBaseURL}/attendance-locations`, { credentials: 'include' });
	return readJSON<AttendanceLocationsResponse>(response, fallbackMessage);
}

export async function updateAttendanceLocations(adminBaseURL: string, locations: AttendanceLocation[], fallbackMessage: string): Promise<AttendanceLocationsResponse> {
	const response = await fetch(`${adminBaseURL}/attendance-locations`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ locations })
	});
	return readJSON<AttendanceLocationsResponse>(response, fallbackMessage);
}

export async function fetchUsers(adminBaseURL: string, fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users?includePolicy=true`, { credentials: 'include' });
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function createCircle(adminBaseURL: string, circle: CircleRecord, fallbackMessage: string): Promise<void> {
	const response = await fetch(`${adminBaseURL}/circles`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(circle)
	});
	await readVoid(response, fallbackMessage);
}

export async function deleteCircle(adminBaseURL: string, circleID: string, fallbackMessage: string): Promise<void> {
	const response = await fetch(`${adminBaseURL}/circles/${encodeURIComponent(circleID)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
	await readVoid(response, fallbackMessage);
}

export async function createUser(adminBaseURL: string, user: NewUserRequest, fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users?includePolicy=true`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(user)
	});
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function saveUser(adminBaseURL: string, user: UserSaveRequest, fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users?includePolicy=true`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(user)
	});
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function saveUsers(adminBaseURL: string, users: UserSaveRequest[], fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users/batch?includePolicy=true`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ users })
	});
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function saveOrgProfiles(adminBaseURL: string, profiles: OrgProfileUpdate[], fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users/org-profiles?includePolicy=true`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ profiles })
	});
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function saveOrgGroups(adminBaseURL: string, groups: OrgGroup[], fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/org-groups?includePolicy=true`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ groups })
	});
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function removeUser(adminBaseURL: string, email: string, fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users/${encodeURIComponent(email)}?includePolicy=true`, {
		method: 'DELETE',
		credentials: 'include'
	});
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function resetUserPassword(adminBaseURL: string, email: string, fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users/${encodeURIComponent(email)}/password-reset`, {
		method: 'POST',
		credentials: 'include'
	});
	return readJSON<UsersResponse>(response, fallbackMessage);
}

export async function fetchWifiProfiles(adminBaseURL: string, fallbackMessage: string): Promise<WifiProfilesResponse> {
	const response = await fetch(`${adminBaseURL}/wifi-profiles`, { credentials: 'include' });
	return readJSON<WifiProfilesResponse>(response, fallbackMessage);
}

export async function addWifiProfile(adminBaseURL: string, ssid: string, password: string, fallbackMessage: string): Promise<WifiProfilesResponse> {
	const response = await fetch(`${adminBaseURL}/wifi-profiles`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ ssid, password })
	});
	return readJSON<WifiProfilesResponse>(response, fallbackMessage);
}

export async function updateWifiPassword(adminBaseURL: string, connectionName: string, password: string, fallbackMessage: string): Promise<WifiProfilesResponse> {
	const response = await fetch(`${adminBaseURL}/wifi-profiles/${encodeURIComponent(connectionName)}`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ password })
	});
	return readJSON<WifiProfilesResponse>(response, fallbackMessage);
}

export async function removeWifiProfile(adminBaseURL: string, connectionName: string, fallbackMessage: string): Promise<WifiProfilesResponse> {
	const response = await fetch(`${adminBaseURL}/wifi-profiles/${encodeURIComponent(connectionName)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
	return readJSON<WifiProfilesResponse>(response, fallbackMessage);
}

export async function fetchDeviceHealth(adminBaseURL: string, fallbackMessage: string): Promise<void> {
	const response = await fetch(`${adminBaseURL}/health`, { credentials: 'include' });
	await readVoid(response, fallbackMessage);
}

export async function fetchBlueclawUpdateStatus(adminBaseURL: string, fallbackMessage: string): Promise<BlueclawUpdateStatus> {
	const response = await fetch(`${adminBaseURL}/updates/status`, { credentials: 'include' });
	return readJSON<BlueclawUpdateStatus>(response, fallbackMessage);
}

export async function fetchReleaseHistory(adminBaseURL: string, fallbackMessage: string): Promise<ReleaseHistoryResponse> {
	const response = await fetch(`${adminBaseURL}/updates/releases`, { credentials: 'include' });
	return readJSON<ReleaseHistoryResponse>(response, fallbackMessage);
}

export async function applyBlueclawUpdate(adminBaseURL: string, fallbackMessage: string, releaseID = ''): Promise<AdminJob> {
	const body = releaseID.trim() ? JSON.stringify({ releaseID: releaseID.trim() }) : undefined;
	const response = await fetch(`${adminBaseURL}/updates/apply`, {
		method: 'POST',
		credentials: 'include',
		headers: body ? { 'Content-Type': 'application/json' } : undefined,
		body
	});
	return readJSON<AdminJob>(response, fallbackMessage);
}

export async function fetchBlueclawUpdateJob(adminBaseURL: string, jobID: string, fallbackMessage: string): Promise<AdminJob> {
	const response = await fetch(`${adminBaseURL}/updates/jobs/${encodeURIComponent(jobID)}`, { credentials: 'include' });
	return readJSON<AdminJob>(response, fallbackMessage);
}

export async function createBackup(adminBaseURL: string, passphrase: string, fallbackMessage: string): Promise<AdminJob> {
	const response = await fetch(`${adminBaseURL}/backups`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ passphrase })
	});
	return readJSON<AdminJob>(response, fallbackMessage);
}

export async function createRestoreUpload(adminBaseURL: string, bundle: File, fallbackMessage: string): Promise<RestoreUploadResponse> {
	const response = await fetch(`${adminBaseURL}/restore/uploads`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ filename: bundle.name, size: bundle.size })
	});
	return readJSON<RestoreUploadResponse>(response, fallbackMessage);
}

export async function uploadRestoreChunk(adminBaseURL: string, uploadID: string, chunkIndex: number, chunk: Blob, fallbackMessage: string): Promise<void> {
	const response = await fetch(`${adminBaseURL}/restore/uploads/${uploadID}/chunks/${chunkIndex}`, {
		method: 'PUT',
		credentials: 'include',
		body: chunk
	});
	await readVoid(response, fallbackMessage);
}

export async function completeRestoreUpload(adminBaseURL: string, uploadID: string, request: RestoreCompletionRequest, fallbackMessage: string): Promise<AdminJob> {
	const response = await fetch(`${adminBaseURL}/restore/uploads/${uploadID}/complete`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	return readJSON<AdminJob>(response, fallbackMessage);
}

export async function fetchBackupJob(adminBaseURL: string, jobID: string, fallbackMessage: string): Promise<AdminJob> {
	const response = await fetch(`${adminBaseURL}/backups/${jobID}/status`, { credentials: 'include' });
	return readJSON<AdminJob>(response, fallbackMessage);
}

export async function fetchRestoreJob(adminBaseURL: string, jobID: string, fallbackMessage: string): Promise<AdminJob> {
	const response = await fetch(`${adminBaseURL}/restore/${jobID}/status`, { credentials: 'include' });
	return readJSON<AdminJob>(response, fallbackMessage);
}

export type NewUserRequest = {
	handle: string;
	name: string;
	email: string;
	hireDate: string;
	role: UserRole;
};

export type UserSaveRequest = Pick<UserRecord, 'userID' | 'handle' | 'email' | 'mattermostUserID' | 'mattermostUsername' | 'status'> & {
	name: string;
	hireDate: string;
	note: string;
	role: UserRole;
	circles: string[];
};

export type OrgProfileUpdate = {
	userID: string;
	email: string;
	jobTitle: string;
	group?: string;
	positionLevel?: number;
	primaryGroupID?: string;
	groupIDs?: string[];
	supervisorID?: string;
	projectIDs?: string[];
	teamRole?: string;
	employmentStatus?: OrgchartEmploymentStatus;
	isOrgchartVisible?: boolean;
};

export type RestoreCompletionRequest = {
	passphrase: string;
	confirm: string;
	chunks: number;
};

async function readJSON<ResponseBody>(response: Response, fallbackMessage: string): Promise<ResponseBody> {
	if (!response.ok) throw new AdminApiError(await responseErrorMessage(response, fallbackMessage), response.status);
	return (await response.json()) as ResponseBody;
}

async function readVoid(response: Response, fallbackMessage: string): Promise<void> {
	if (!response.ok) throw new AdminApiError(await responseErrorMessage(response, fallbackMessage), response.status);
}

async function responseErrorMessage(response: Response, fallbackMessage: string): Promise<string> {
	const text = await response.text();
	return text.trim() || fallbackMessage;
}

export function apiErrorMessage(error: unknown, fallbackMessage: string): string {
	if (error instanceof AdminApiError && fallbackAdminApiErrorMessages.has(error.message)) return fallbackMessage;
	if (error instanceof TypeError && fallbackNetworkErrorMessages.has(error.message)) return fallbackMessage;
	if (error instanceof Error && error.message) return error.message;
	return fallbackMessage;
}

export function isAdminApiStatus(error: unknown, status: number): boolean {
	return error instanceof AdminApiError && error.status === status;
}
