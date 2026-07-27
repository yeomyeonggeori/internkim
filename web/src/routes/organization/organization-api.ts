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

export async function saveOwnPhoneNumber(phoneNumber: string): Promise<string> {
	const response = await fetch('/organization/api/me/phone-number', {
		method: 'PUT',
		headers: { 'content-type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ phoneNumber })
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `phone number update returned ${response.status}`);
	const saved = (await response.json()) as { phoneNumber?: string };
	return saved.phoneNumber ?? '';
}

export async function fetchWebSessionEmail(): Promise<string> {
	const response = await fetch('/auth/session', { credentials: 'include' });
	if (!response.ok) return '';
	const session = (await response.json()) as { email?: string };
	return (session.email ?? '').trim().toLowerCase();
}
