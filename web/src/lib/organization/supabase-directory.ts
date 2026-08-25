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

export async function saveOwnSupabaseProfile(phoneNumber: string, hireDate: string) {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) throw new Error('sign in first');

	const saved = await supabase()
		.from('member')
		.update({ phone_number: phoneNumber || null, joined_at: hireDate || null })
		.eq('user_id', accountID)
		.select('phone_number, joined_at')
		.single();
	if (saved.error) throw new Error(saved.error.message);
	return {
		phoneNumber: saved.data.phone_number ?? '',
		hireDate: saved.data.joined_at ? String(saved.data.joined_at).slice(0, 10) : '',
	};
}
