import { apiErrorMessage } from '../admin/admin-api';
import type { OrgGroup, OrgProfileUpdate, UsersResponse } from '../../lib/organization/types';
import {
	saveOwnSupabaseProfile,
	saveSupabaseMemberProfiles,
	saveSupabaseTeams,
	supabaseOrganizationDirectory
} from '$lib/organization/supabase-directory';

export async function fetchOrganizationDirectory(): Promise<UsersResponse> {
	return supabaseOrganizationDirectory();
}

export async function saveOrganizationProfiles(profiles: OrgProfileUpdate[]): Promise<UsersResponse> {
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

export async function saveOrganizationGroups(groups: OrgGroup[]): Promise<UsersResponse> {
	return saveSupabaseTeams(groups);
}

export function organizationApiErrorMessage(error: unknown, fallbackMessage: string): string {
	return apiErrorMessage(error, fallbackMessage);
}

export type OwnOrganizationProfile = {
	phoneNumber: string;
	hireDate: string;
};

export async function saveOwnOrganizationProfile(
	memberID: string,
	profile: OwnOrganizationProfile
): Promise<OwnOrganizationProfile> {
	return saveOwnSupabaseProfile(memberID, profile.phoneNumber, profile.hireDate);
}
