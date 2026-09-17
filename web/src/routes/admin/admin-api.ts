import type {
	AdminJob,
	AdminSession,
	AttendanceLeavePolicy,
	AttendanceWorkPolicy,
	AttendanceWorkPolicyResponse,
	AttendanceWorkPolicyRevision,
	BlueclawUpdateStatus,
	AgentSoul,
	CircleRecord,
	CompanyHoliday,
	CompanyHolidayInput,
	CompanyHolidaysResponse,
	CompanyShareSettings,
	CompanyShareSettingsUpdate,
	CredentialProviderStatus,
	CredentialProvidersResponse,
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
	'supervisor hierarchy cannot contain cycles'
]);
const fallbackNetworkErrorMessages = new Set(['Failed to fetch', 'Load failed', 'NetworkError when attempting to fetch resource.', 'fetch failed']);

export async function fetchAdminSession(adminBaseURL: string, fallbackMessage: string): Promise<AdminSession> {
	const response = await fetch(`${adminBaseURL}/session`, { credentials: 'include' });
	return readJSON<AdminSession>(response, fallbackMessage);
}

export async function fetchSoul(adminBaseURL: string, fallbackMessage: string): Promise<AgentSoul> {
	const response = await fetch(`${adminBaseURL}/soul`, { credentials: 'include' });
	return readJSON<AgentSoul>(response, fallbackMessage);
}

export async function updateSoul(adminBaseURL: string, soul: AgentSoul, fallbackMessage: string): Promise<AgentSoul> {
	const response = await fetch(`${adminBaseURL}/soul`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(soul)
	});
	return readJSON<AgentSoul>(response, fallbackMessage);
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

export async function fetchCompanyShareSettings(adminBaseURL: string, fallbackMessage: string): Promise<CompanyShareSettings> {
	const response = await fetch(`${adminBaseURL}/company-share`, { credentials: 'include' });
	return readJSON<CompanyShareSettings>(response, fallbackMessage);
}

export async function updateCompanyShareSettings(
	adminBaseURL: string,
	settings: CompanyShareSettingsUpdate,
	fallbackMessage: string
): Promise<CompanyShareSettings> {
	const response = await fetch(`${adminBaseURL}/company-share`, {
		method: 'PUT',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(settings)
	});
	return readJSON<CompanyShareSettings>(response, fallbackMessage);
}

export async function publishCompanyShare(adminBaseURL: string, fallbackMessage: string): Promise<CompanyShareSettings> {
	const response = await fetch(`${adminBaseURL}/company-share/publish`, { method: 'POST', credentials: 'include' });
	return readJSON<CompanyShareSettings>(response, fallbackMessage);
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

export async function removeUser(adminBaseURL: string, email: string, fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch(`${adminBaseURL}/users/${encodeURIComponent(email)}?includePolicy=true`, {
		method: 'DELETE',
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

export type UserSaveRequest = Pick<UserRecord, 'memberID' | 'handle' | 'email' | 'status'> & {
	name: string;
	hireDate: string;
	note: string;
	role: UserRole;
	circles: string[];
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
	const text = (await response.text()).trim();
	if (!text || text.startsWith('<') || text.length > 300) return fallbackMessage;
	return text;
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
