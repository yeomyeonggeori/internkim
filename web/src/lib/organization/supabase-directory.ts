import { supabase } from '$lib/supabase';
import type { OrgGroup, UserRecord, UsersResponse } from '$lib/organization/types';

type MemberRow = {
	id: string;
	name: string | null;
	email: string | null;
	job_title: string | null;
	phone_number: string | null;
	joined_at: string | null;
	team_id: string | null;
	supervisor_id: string | null;
	is_admin: boolean;
};

type TeamRow = { id: string; name: string; parent_team_id: string | null };

export async function supabaseOrganizationDirectory(): Promise<UsersResponse> {
	const members = await supabase()
		.from('member')
		.select('id, name, email, job_title, phone_number, joined_at, team_id, supervisor_id, is_admin')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const teams = await supabase()
		.from('team')
		.select('id, name, parent_team_id')
		.order('position')
		.returns<TeamRow[]>();
	if (teams.error) throw new Error(teams.error.message);

	return {
		records: (members.data ?? []).map(recordOf),
		availableGroups: (teams.data ?? []).map(groupOf),
	};
}

function recordOf(member: MemberRow): UserRecord {
	const email = member.email ?? '';
	const handle = email.split('@')[0];
	return {
		memberID: member.id,
		email,
		handle,
		name: member.name || handle,
		role: member.is_admin ? 'admin' : 'member',
		jobTitle: member.job_title ?? undefined,
		phoneNumber: member.phone_number ?? undefined,
		hireDate: member.joined_at ? member.joined_at.slice(0, 10) : undefined,
		groupID: member.team_id ?? undefined,
		supervisorID: member.supervisor_id ?? undefined,
	};
}

function groupOf(team: TeamRow): OrgGroup {
	return { id: team.id, name: team.name, parentID: team.parent_team_id ?? undefined };
}

export type MemberProfileUpdate = {
	memberID: string;
	jobTitle: string;
	groupID: string;
	hireDate: string;
	phoneNumber: string;
	supervisorID: string;
};

export async function saveSupabaseMemberProfiles(profiles: MemberProfileUpdate[]): Promise<UsersResponse> {
	const saved = await supabase().rpc('save_member_profiles', { profiles });
	if (saved.error) throw new Error(saved.error.message);
	return supabaseOrganizationDirectory();
}

export async function saveSupabaseTeams(groups: OrgGroup[]): Promise<UsersResponse> {
	const teams = groups.map((group) => ({ id: group.id, name: group.name, parentID: group.parentID ?? '' }));
	const saved = await supabase().rpc('save_teams', { teams });
	if (saved.error) throw new Error(saved.error.message);
	return supabaseOrganizationDirectory();
}

export type SavedMemberProfile = { phoneNumber: string; hireDate: string };

function savedMemberProfileOf(value: unknown): SavedMemberProfile {
	if (!value || typeof value !== 'object' || Array.isArray(value)) {
		throw new Error('save_own_member_profile answered with no member profile');
	}
	const fields = value as Record<string, unknown>;
	return {
		phoneNumber: typeof fields.phoneNumber === 'string' ? fields.phoneNumber : '',
		hireDate: typeof fields.hireDate === 'string' ? fields.hireDate : '',
	};
}

export async function saveOwnSupabaseProfile(
	phoneNumber: string,
	hireDate: string,
): Promise<SavedMemberProfile> {
	const saved = await supabase().rpc('save_own_member_profile', {
		new_phone_number: phoneNumber || null,
		new_hire_date: hireDate || null,
	});
	if (saved.error) throw new Error(saved.error.message);
	return savedMemberProfileOf(saved.data);
}
