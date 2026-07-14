import { AdminApiError, apiErrorMessage } from '../admin/admin-api';
import type { UsersResponse } from './orgchart-types';

export async function fetchOrgchartDirectory(fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch('/orgchart/api/people', { credentials: 'include' });
	if (!response.ok) throw new AdminApiError(await responseErrorMessage(response, fallbackMessage), response.status);
	return (await response.json()) as UsersResponse;
}

export function orgchartApiErrorMessage(error: unknown, fallbackMessage: string): string {
	return apiErrorMessage(error, fallbackMessage);
}

async function responseErrorMessage(response: Response, fallbackMessage: string): Promise<string> {
	const text = await response.text();
	return text.trim() || fallbackMessage;
}
