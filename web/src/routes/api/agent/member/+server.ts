import { json, error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	// Without an address this answers the whole directory. Who works here is one
	// question with one answer, and a caller that has to ask name by name cannot
	// know about somebody it has never heard of - which is every person invited
	// since it last looked.
	const email = (url.searchParams.get('email') ?? '').trim().toLowerCase();
	const everyone = await client
		.from('member')
		.select(memberColumns)
		.eq('company_id', companyID)
		.returns<MemberRow[]>();
	if (everyone.error) return json({ error: everyone.error.message }, { status: 502 });

	const rows = everyone.data ?? [];
	const teams = await client
		.from('team')
		.select('id, name')
		.eq('company_id', companyID)
		.returns<{ id: string; name: string }[]>();
	if (teams.error) return json({ error: teams.error.message }, { status: 502 });

	const named = namedMembers(rows, teams.data ?? []);
	if (!email) return json({ members: named });
	return json({ member: named.find((member) => member.email === email) ?? null });
};

const memberColumns =
	'id, email, name, messenger, is_admin, status, job_title, phone_number, joined_at, team_id, supervisor_id';

type MemberRow = {
	id: string;
	email: string | null;
	name: string | null;
	messenger: Record<string, string> | null;
	is_admin: boolean;
	status: string;
	job_title: string | null;
	phone_number: string | null;
	joined_at: string | null;
	team_id: string | null;
	supervisor_id: string | null;
};

function namedMembers(rows: MemberRow[], teams: { id: string; name: string }[]) {
	const emailByMemberID = new Map(rows.map((row) => [row.id, (row.email ?? '').toLowerCase()]));
	const nameByTeamID = new Map(teams.map((team) => [team.id, team.name]));
	return rows.map((row) => ({
		memberID: row.id,
		email: (row.email ?? '').toLowerCase(),
		name: row.name ?? '',
		messenger: row.messenger ?? {},
		role: row.is_admin ? 'admin' : 'member',
		status: row.status,
		jobTitle: row.job_title ?? '',
		phoneNumber: row.phone_number ?? '',
		hireDate: row.joined_at ? String(row.joined_at).slice(0, 10) : '',
		teamID: row.team_id ?? '',
		teamName: row.team_id ? (nameByTeamID.get(row.team_id) ?? '') : '',
		supervisorEmail: row.supervisor_id ? (emailByMemberID.get(row.supervisor_id) ?? '') : ''
	}));
}

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = (await request.json().catch(() => ({}))) as { email?: unknown; name?: unknown };
	const email = typeof asked.email === 'string' ? asked.email.trim().toLowerCase() : '';
	if (!email) error(400, 'email required');
	const name = typeof asked.name === 'string' ? asked.name.trim() : '';

	const existing = await client
		.from('member')
		.select('id, email, is_admin, status, company_id')
		.eq('email', email)
		.maybeSingle();
	if (existing.error) return json({ error: existing.error.message }, { status: 502 });
	if (existing.data && existing.data.company_id !== companyID) error(409, 'that address belongs to another company');
	if (existing.data) {
		if (name) await client.from('member').update({ name }).eq('id', existing.data.id);
		return json({
			member: {
				memberID: existing.data.id,
				email,
				role: existing.data.is_admin ? 'admin' : 'member',
				status: existing.data.status
			}
		});
	}

	const created = await client
		.from('member')
		.insert({ company_id: companyID, email, ...(name ? { name } : {}) })
		.select('id, is_admin, status')
		.single();
	if (created.error) return json({ error: created.error.message }, { status: 502 });

	return json({
		member: {
			memberID: created.data.id,
			email,
			role: created.data.is_admin ? 'admin' : 'member',
			status: created.data.status
		}
	});
};

type ProfileUpdate = {
	email?: unknown;
	jobTitle?: unknown;
	phoneNumber?: unknown;
	hireDate?: unknown;
	supervisorEmail?: unknown;
	teamName?: unknown;
};

export const PATCH: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = (await request.json().catch(() => ({}))) as { profiles?: unknown };
	if (!Array.isArray(asked.profiles)) error(400, 'profiles required');

	const written: string[] = [];
	for (const offered of asked.profiles as ProfileUpdate[]) {
		const email = typeof offered.email === 'string' ? offered.email.trim().toLowerCase() : '';
		if (!email) error(400, 'every profile needs an email');

		const member = await memberOfCompany(client, companyID, email);
		if (!member) continue;

		const update = await profileColumns(client, companyID, member.id, offered);
		if (Object.keys(update).length === 0) continue;

		const saved = await client.from('member').update(update).eq('id', member.id);
		if (saved.error) return json({ error: saved.error.message }, { status: 502 });
		written.push(email);
	}

	return json({ written });
};

async function profileColumns(
	client: SupabaseClient,
	companyID: string,
	memberID: string,
	offered: ProfileUpdate
): Promise<Record<string, string | null>> {
	const update: Record<string, string | null> = {};
	if (offered.jobTitle !== undefined) update.job_title = textOrNull(offered.jobTitle);
	if (offered.phoneNumber !== undefined) update.phone_number = textOrNull(offered.phoneNumber);
	if (offered.hireDate !== undefined) update.joined_at = textOrNull(offered.hireDate);
	if (offered.teamName !== undefined) {
		update.team_id = await teamOfCompany(client, companyID, textOrNull(offered.teamName));
	}
	if (offered.supervisorEmail !== undefined) {
		update.supervisor_id = await supervisorOfCompany(client, companyID, memberID, offered.supervisorEmail);
	}
	return update;
}

async function supervisorOfCompany(
	client: SupabaseClient,
	companyID: string,
	memberID: string,
	offered: unknown
): Promise<string | null> {
	const email = textOrNull(offered);
	if (!email) return null;
	const supervisor = await memberOfCompany(client, companyID, email.toLowerCase());
	if (!supervisor) error(409, `${email} is not in this company`);
	return supervisor.id === memberID ? null : supervisor.id;
}

async function memberOfCompany(client: SupabaseClient, companyID: string, email: string) {
	const { data, error: queryError } = await client
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.eq('email', email)
		.maybeSingle<{ id: string }>();
	if (queryError) throw new Error(queryError.message);
	return data;
}

async function teamOfCompany(client: SupabaseClient, companyID: string, name: string | null) {
	if (!name) return null;
	const existing = await client
		.from('team')
		.select('id')
		.eq('company_id', companyID)
		.eq('name', name)
		.maybeSingle<{ id: string }>();
	if (existing.error) throw new Error(existing.error.message);
	if (existing.data) return existing.data.id;

	const created = await client
		.from('team')
		.insert({ company_id: companyID, name })
		.select('id')
		.single<{ id: string }>();
	if (created.error) throw new Error(created.error.message);
	return created.data.id;
}

function textOrNull(offered: unknown): string | null {
	if (typeof offered !== 'string') return null;
	const text = offered.trim();
	return text === '' ? null : text;
}
