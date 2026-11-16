import { saveOwnSupabaseProfile, supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
import { supabaseMember } from '$lib/supabase-session';

export type OwnProfile = {
	memberID: string;
	name: string;
	email: string;
	jobTitle: string;
	teamName: string;
	phoneNumber: string;
	hireDate: string;
};

export type OwnProfileEdit = { phoneNumber: string; hireDate: string };

export async function loadOwnProfile(): Promise<OwnProfile> {
	const [member, directory] = await Promise.all([supabaseMember(), supabaseOrganizationDirectory()]);
	const own = (directory.records ?? []).find((record) => record.memberID === member.memberID);
	if (!own) throw new Error('the company directory does not list the member who is signed in');
	const team = (directory.availableGroups ?? []).find((group) => group.id === own.groupID);
	return {
		memberID: own.memberID,
		name: own.name ?? '',
		email: own.email,
		jobTitle: own.jobTitle ?? '',
		teamName: team?.name ?? '',
		phoneNumber: own.phoneNumber ?? '',
		hireDate: own.hireDate ?? ''
	};
}

export function saveOwnProfile(memberID: string, edit: OwnProfileEdit): Promise<OwnProfileEdit> {
	return saveOwnSupabaseProfile(memberID, edit.phoneNumber, edit.hireDate);
}
