import type { SupabaseClient } from '@supabase/supabase-js';
import { titleNearness, typoNearness } from './hint-nearness';
import {
	HintRefused,
	normalized,
	resolveHint,
	type HintCandidate,
	type HintMatcher
} from './hint-resolution';

export type RecordTeam = {
	teamID: string;
	name: string;
	parentTeamID: string;
	position: number;
};

type TeamRow = { id: string; name: string; parent_team_id: string | null; position: number | null };

export async function teamsOfCompany(caller: SupabaseClient): Promise<RecordTeam[]> {
	const { data, error } = await caller
		.from('team')
		.select('id, name, parent_team_id, position')
		.order('position')
		.returns<TeamRow[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((team) => ({
		teamID: team.id,
		name: team.name,
		parentTeamID: team.parent_team_id ?? '',
		position: team.position ?? 0
	}));
}

const teamMatcher: HintMatcher<RecordTeam> = {
	identifiersOf: (team) => [team.teamID],
	titleOf: (team) => team.name,
	nearnessTo: (team, hint) =>
		Math.max(
			typoNearness(normalized(hint), normalized(team.name)),
			titleNearness(normalized(hint), normalized(team.name))
		)
};

export function teamOfHint(teams: RecordTeam[], hint: string): RecordTeam {
	const resolution = resolveHint(hint, teams, teamMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		'organization',
		hint.trim(),
		resolution.outcome,
		resolution.candidates.map(candidateOfTeam)
	);
}

export function candidateOfTeam(team: RecordTeam): HintCandidate {
	return { id: team.teamID, label: team.name };
}

export function descendantsOfTeam(teams: RecordTeam[], teamID: string): Set<string> {
	const held = new Set([teamID]);
	let grew = true;
	while (grew) {
		grew = false;
		for (const team of teams) {
			if (held.has(team.teamID) || !held.has(team.parentTeamID)) continue;
			held.add(team.teamID);
			grew = true;
		}
	}
	return held;
}
