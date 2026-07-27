import { AdminApiError, apiErrorMessage } from '../admin/admin-api';
import type { UsersResponse } from '../../lib/organization/types';

export async function fetchOrganizationDirectory(fallbackMessage: string): Promise<UsersResponse> {
	const response = await fetch('/organization/api/people', { credentials: 'include' });
	if (!response.ok) throw new AdminApiError(await responseErrorMessage(response, fallbackMessage), response.status);
	return (await response.json()) as UsersResponse;
}

export function organizationApiErrorMessage(error: unknown, fallbackMessage: string): string {
	return apiErrorMessage(error, fallbackMessage);
}

async function responseErrorMessage(response: Response, fallbackMessage: string): Promise<string> {
	const text = await response.text();
	return text.trim() || fallbackMessage;
}
