import { json, error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';

type TeamRow = { id: string; name: string; parent_team_id: string | null };

export const GET: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));
	const teams = await heldTeams(client, companyID);
	return json({ teams: teams.map(namedTeam) });
};

type OfferedTeam = { teamID?: unknown; name?: unknown; parentName?: unknown };

export const PUT: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = (await request.json().catch(() => ({}))) as { teams?: unknown };
	if (!Array.isArray(asked.teams)) error(400, 'teams required');
	const offered = (asked.teams as OfferedTeam[]).map(offeredTeam);

	const held = await heldTeams(client, companyID);
	const settled = await settleTeams(client, companyID, held, offered);
	const dropped = await dropTeamsNobodyOffered(client, held, settled);

	return json({ teams: settled.map(namedTeam), dropped });
};

async function heldTeams(client: SupabaseClient, companyID: string): Promise<TeamRow[]> {
	const teams = await client
		.from('team')
		.select('id, name, parent_team_id')
		.eq('company_id', companyID)
		.order('position')
		.returns<TeamRow[]>();
	if (teams.error) throw new Error(teams.error.message);
	return teams.data ?? [];
}

async function settleTeams(
	client: SupabaseClient,
	companyID: string,
	held: TeamRow[],
	offered: { teamID: string; name: string; parentName: string }[]
): Promise<TeamRow[]> {
	const heldByID = new Map(held.map((team) => [team.id, team]));
	const heldByName = new Map(held.map((team) => [team.name.toLowerCase(), team]));

	const settled: TeamRow[] = [];
	for (const [position, team] of offered.entries()) {
		const known = heldByID.get(team.teamID) ?? heldByName.get(team.name.toLowerCase());
		settled.push(
			known
				? await renameTeam(client, known, team.name, position)
				: await createTeam(client, companyID, team.name, position)
		);
	}
	await parentTeams(client, settled, offered);
	return settled;
}

async function renameTeam(client: SupabaseClient, held: TeamRow, name: string, position: number): Promise<TeamRow> {
	const renamed = await client.from('team').update({ name, position }).eq('id', held.id);
	if (renamed.error) throw new Error(renamed.error.message);
	return { ...held, name };
}

async function createTeam(client: SupabaseClient, companyID: string, name: string, position: number): Promise<TeamRow> {
	const created = await client
		.from('team')
		.insert({ company_id: companyID, name, position })
		.select('id, name, parent_team_id')
		.single<TeamRow>();
	if (created.error) throw new Error(created.error.message);
	return created.data;
}

async function parentTeams(
	client: SupabaseClient,
	settled: TeamRow[],
	offered: { parentName: string }[]
): Promise<void> {
	const idByName = new Map(settled.map((team) => [team.name.toLowerCase(), team.id]));
	for (const [index, team] of settled.entries()) {
		const parentID = idByName.get(offered[index].parentName.toLowerCase()) ?? null;
		if ((team.parent_team_id ?? null) === (parentID === team.id ? null : parentID)) continue;
		const parented = await client
			.from('team')
			.update({ parent_team_id: parentID === team.id ? null : parentID })
			.eq('id', team.id);
		if (parented.error) throw new Error(parented.error.message);
		team.parent_team_id = parentID === team.id ? null : parentID;
	}
}

async function dropTeamsNobodyOffered(
	client: SupabaseClient,
	held: TeamRow[],
	settled: TeamRow[]
): Promise<number> {
	const kept = new Set(settled.map((team) => team.id));
	const dropped = held.filter((team) => !kept.has(team.id)).map((team) => team.id);
	if (dropped.length === 0) return 0;
	const removed = await client.from('team').delete().in('id', dropped);
	if (removed.error) throw new Error(removed.error.message);
	return dropped.length;
}

function offeredTeam(team: OfferedTeam) {
	const name = typeof team.name === 'string' ? team.name.trim() : '';
	if (!name) error(400, 'every team needs a name');
	return {
		teamID: typeof team.teamID === 'string' ? team.teamID.trim() : '',
		name,
		parentName: typeof team.parentName === 'string' ? team.parentName.trim() : ''
	};
}

function namedTeam(team: TeamRow) {
	return { teamID: team.id, name: team.name, parentTeamID: team.parent_team_id ?? '' };
}
