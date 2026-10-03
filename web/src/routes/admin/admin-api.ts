import type {
	AdminSession,
	AttendanceLeavePolicy,
	AttendanceWorkPolicy,
	AttendanceWorkPolicyResponse,
	AttendanceWorkPolicyRevision,
	AgentSoul,
	CompanyHoliday,
	CompanyHolidayInput,
	CompanyHolidaysResponse,
	CompanyShareSettings,
	CompanyShareSettingsUpdate,
	CredentialProviderStatus,
	CredentialProvidersResponse,
	OrgGroup,
	UserRecord,
	UserRole,
	UsersResponse,
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

async function readJSON<ResponseBody>(response: Response, fallbackMessage: string): Promise<ResponseBody> {
	if (!response.ok) throw new AdminApiError(await responseErrorMessage(response, fallbackMessage), response.status);
	return (await response.json()) as ResponseBody;
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
