import type { AdminSession } from './admin-types';

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
