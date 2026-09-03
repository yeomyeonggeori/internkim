import type { TeamDeleteResult, TeamListResult, TeamResult } from '../catalog/people';
import type { RecordContext } from './company';
import type { RecordPerson } from './people';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import { teamOfHint, teamsOfCompany, type RecordTeam } from './teams';

export type TeamAddInput = { name?: string; parentHint?: string; position?: number };

export type TeamUpdateInput = {
	teamHint?: string;
	name?: string;
	parentHint?: string;
	position?: number;
};

export type TeamDeleteInput = { teamHint?: string };

function answeredTeam(team: RecordTeam, teams: RecordTeam[], people: RecordPerson[]): TeamResult {
	const parent = teams.find((candidate) => candidate.teamID === team.parentTeamID);
	return {
		teamID: team.teamID,
		name: team.name,
		parentTeamID: team.parentTeamID,
		parentTeamName: parent?.name ?? '',
		position: team.position,
		peopleCount: people.filter((person) => person.teamID === team.teamID).length
	};
}

export async function teamList(context: RecordContext): Promise<TeamListResult> {
	const teams = await teamsOfCompany(context.caller);
	return {
		count: teams.length,
		teams: teams.map((team) => answeredTeam(team, teams, context.people))
	};
}

export async function teamAdd(context: RecordContext, input: TeamAddInput): Promise<TeamResult> {
	const name = (input.name ?? '').trim();
	if (!name) throw new Error('a new organization needs a name');

	const teams = await teamsOfCompany(context.caller);
	const { data, error } = await context.caller.rpc('team_add', {
		new_name: name,
		parent_team: parentTeamIDOfHint(teams, input.parentHint) || null,
		new_position: input.position ?? null
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	return teamWrittenBack(context, String(data));
}

export async function teamUpdate(context: RecordContext, input: TeamUpdateInput): Promise<TeamResult> {
	if (!input.teamHint?.trim()) throw new Error('an organization change names the organization it changes');

	const teams = await teamsOfCompany(context.caller);
	const team = teamOfHint(teams, input.teamHint);
	const changes: Record<string, string | number> = {};
	if (input.name !== undefined) changes.name = input.name;
	if (input.parentHint !== undefined) changes.parentID = parentTeamIDOfHint(teams, input.parentHint);
	if (input.position !== undefined) changes.position = input.position;
	if (Object.keys(changes).length === 0) {
		throw new Error('an organization change names at least one field to change');
	}

	const { error } = await context.caller.rpc('team_update', {
		target_team: team.teamID,
		changes
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	return teamWrittenBack(context, team.teamID);
}

export async function teamDelete(
	context: RecordContext,
	input: TeamDeleteInput
): Promise<TeamDeleteResult> {
	if (!input.teamHint?.trim()) throw new Error('a removal names the organization it removes');

	const teams = await teamsOfCompany(context.caller);
	const team = teamOfHint(teams, input.teamHint);
	const { data, error } = await context.caller.rpc('team_delete', { target_team: team.teamID });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	return {
		teamID: team.teamID,
		name: team.name,
		deleted: true,
		peopleLeftWithNoOrganization: Number(data ?? 0)
	};
}

function parentTeamIDOfHint(teams: RecordTeam[], hint: string | undefined): string {
	const asked = (hint ?? '').trim();
	if (asked === '') return '';
	return teamOfHint(teams, asked).teamID;
}

async function teamWrittenBack(context: RecordContext, teamID: string): Promise<TeamResult> {
	const teams = await teamsOfCompany(context.caller);
	const written = teams.find((team) => team.teamID === teamID);
	if (!written) {
		throw new RecordRefusedTheWrite('the record saved this organization and did not answer with it', 502);
	}
	return answeredTeam(written, teams, context.people);
}
