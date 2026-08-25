import { AdminApiError, apiErrorMessage, saveOrgGroups, saveOrgProfiles, type OrgProfileUpdate } from '../admin/admin-api';
import type { OrgGroup, UsersResponse } from '../../lib/organization/types';
import {
	saveOwnSupabaseProfile,
	saveSupabaseMemberProfiles,
	saveSupabaseTeams,
	supabaseOrganizationDirectory
} from '$lib/organization/supabase-directory';
import { isSupabaseConfigured } from '$lib/supabase';

export async function fetchOrganizationDirectory(fallbackMessage: string): Promise<UsersResponse> {
	if (isSupabaseConfigured()) return supabaseOrganizationDirectory();
	const response = await fetch('/organization/api/people', { credentials: 'include' });
	if (!response.ok) throw new AdminApiError(await responseErrorMessage(response, fallbackMessage), response.status);
	return (await response.json()) as UsersResponse;
}

export async function saveOrganizationProfiles(
	adminBaseURL: string,
	profiles: OrgProfileUpdate[],
	fallbackMessage: string
): Promise<UsersResponse> {
	if (!isSupabaseConfigured()) return saveOrgProfiles(adminBaseURL, profiles, fallbackMessage);
	return saveSupabaseMemberProfiles(
		profiles.map((profile) => ({
			memberID: profile.memberID,
			jobTitle: profile.jobTitle,
			groupID: profile.groupID ?? '',
			hireDate: profile.hireDate ?? '',
			phoneNumber: profile.phoneNumber ?? '',
			supervisorID: profile.supervisorID ?? ''
		}))
	);
}

export async function saveOrganizationGroups(
	adminBaseURL: string,
	groups: OrgGroup[],
	fallbackMessage: string
): Promise<UsersResponse> {
	if (!isSupabaseConfigured()) return saveOrgGroups(adminBaseURL, groups, fallbackMessage);
	return saveSupabaseTeams(groups);
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
	if (isSupabaseConfigured()) return saveOwnSupabaseProfile(profile.phoneNumber, profile.hireDate);
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
