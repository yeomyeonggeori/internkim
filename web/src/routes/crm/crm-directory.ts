import type { UsersResponse } from '$lib/organization/types';

export async function fetchCRMOrganizationDirectory(): Promise<UsersResponse> {
	const response = await fetch('/organization/api/people', { credentials: 'include' });
	if (!response.ok) throw new Error((await response.text()).trim() || `directory returned ${response.status}`);
	return (await response.json()) as UsersResponse;
}
