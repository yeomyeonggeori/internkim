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

export type OwnOrganizationProfile = {
	phoneNumber: string;
	hireDate: string;
};

export async function saveOwnOrganizationProfile(profile: OwnOrganizationProfile): Promise<OwnOrganizationProfile> {
	const response = await fetch('/organization/api/me/profile', {
		method: 'PUT',
		headers: { 'content-type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify(profile)
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `profile update returned ${response.status}`);
	const saved = (await response.json()) as Partial<OwnOrganizationProfile>;
	return { phoneNumber: saved.phoneNumber ?? '', hireDate: saved.hireDate ?? '' };
}

export async function fetchWebSessionEmail(): Promise<string> {
	const response = await fetch('/auth/session', { credentials: 'include' });
	if (!response.ok) return '';
	const session = (await response.json()) as { email?: string };
	return (session.email ?? '').trim().toLowerCase();
}
