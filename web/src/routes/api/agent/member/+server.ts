import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { circleNamesByMemberID, memberWriteSchema, saveMember, withdrawMember } from '$lib/server/member-directory';

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

	const circles = await client
		.from('circle')
		.select('name, circle_member(member_id)')
		.eq('company_id', companyID)
		.order('name')
		.returns<{ name: string; circle_member: { member_id: string }[] | null }[]>();
	if (circles.error) return json({ error: circles.error.message }, { status: 502 });

	const named = namedMembers(rows, teams.data ?? [], circleNamesByMemberID(circles.data ?? []));
	if (!email) return json({ members: named });
	return json({ member: named.find((member) => member.email === email) ?? null });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = memberWriteSchema.safeParse(await request.json().catch(() => ({})));
	if (!asked.success) error(400, 'a member write names an address and may carry a name, a role, a note and messenger accounts');

	return json({ member: await saveMember({ client, companyID }, asked.data) });
};

export const DELETE: RequestHandler = async ({ request, url, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const email = (url.searchParams.get('email') ?? '').trim().toLowerCase();
	if (!email) error(400, 'email required');

	const withdrawn = await withdrawMember({ client, companyID }, email);
	if (!withdrawn) error(404, 'nobody here goes by that address');
	return json({ member: withdrawn });
};

const memberColumns =
	'id, email, name, note, messenger, is_admin, status, job_title, phone_number, joined_at, team_id, supervisor_id';

type MemberRow = {
	id: string;
	email: string | null;
	name: string | null;
	note: string | null;
	messenger: Record<string, string> | null;
	is_admin: boolean;
	status: string;
	job_title: string | null;
	phone_number: string | null;
	joined_at: string | null;
	team_id: string | null;
	supervisor_id: string | null;
};

function namedMembers(
	rows: MemberRow[],
	teams: { id: string; name: string }[],
	circlesByMemberID: Map<string, string[]>
) {
	const emailByMemberID = new Map(rows.map((row) => [row.id, (row.email ?? '').toLowerCase()]));
	const nameByTeamID = new Map(teams.map((team) => [team.id, team.name]));
	return rows.map((row) => ({
		memberID: row.id,
		email: (row.email ?? '').toLowerCase(),
		name: row.name ?? '',
		note: row.note ?? '',
		messenger: row.messenger ?? {},
		role: row.is_admin ? 'admin' : 'member',
		circles: circlesByMemberID.get(row.id) ?? [],
		status: row.status,
		jobTitle: row.job_title ?? '',
		phoneNumber: row.phone_number ?? '',
		hireDate: row.joined_at ? String(row.joined_at).slice(0, 10) : '',
		teamID: row.team_id ?? '',
		teamName: row.team_id ? (nameByTeamID.get(row.team_id) ?? '') : '',
		supervisorEmail: row.supervisor_id ? (emailByMemberID.get(row.supervisor_id) ?? '') : ''
	}));
}
