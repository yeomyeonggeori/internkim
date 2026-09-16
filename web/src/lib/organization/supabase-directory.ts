import { invokeTool } from '$lib/public-api-call';
import { memberRoleOf } from '$lib/member-vocabulary';
import type { OrgGroup, UserRecord, UsersResponse } from '$lib/organization/types';

type AnsweredPerson = {
	personID: string;
	name: string;
	email: string;
	handle?: string;
	isAdmin?: boolean;
	clearance?: number;
	jobTitle?: string;
	teamID?: string;
	supervisorID?: string;
	phoneNumber?: string;
	hireDate?: string;
};

type AnsweredPeople = { count: number; people: AnsweredPerson[] };

type AnsweredTeam = {
	teamID: string;
	name: string;
	parentTeamID: string;
	position: number;
};

type AnsweredTeams = { count: number; teams: AnsweredTeam[] };

export async function supabaseOrganizationDirectory(): Promise<UsersResponse> {
	const [people, teams] = await Promise.all([
		invokeTool<AnsweredPeople>('person_list', {}),
		invokeTool<AnsweredTeams>('team_list', {})
	]);
	return {
		records: people.people.map(recordOf),
		availableGroups: teams.teams.map(groupOf)
	};
}

function recordOf(person: AnsweredPerson): UserRecord {
	return {
		memberID: person.personID,
		email: person.email,
		handle: handleOf(person),
		name: person.name,
		role: memberRoleOf(person.isAdmin === true),
		clearance: person.clearance,
		jobTitle: person.jobTitle,
		phoneNumber: person.phoneNumber,
		hireDate: person.hireDate,
		groupID: person.teamID,
		supervisorID: person.supervisorID
	};
}

function handleOf(person: AnsweredPerson): string {
	const named = (person.handle ?? '').replace(/^@/, '');
	return named || person.email.split('@')[0];
}

function groupOf(team: AnsweredTeam): OrgGroup {
	return {
		id: team.teamID,
		name: team.name,
		...(team.parentTeamID ? { parentID: team.parentTeamID } : {})
	};
}

export type MemberProfileUpdate = {
	memberID: string;
	jobTitle: string;
	groupID: string;
	hireDate: string;
	phoneNumber: string;
	supervisorID: string;
	clearance?: number;
};

export async function saveSupabaseMemberProfiles(profiles: MemberProfileUpdate[]): Promise<UsersResponse> {
	for (const profile of profiles) {
		await invokeTool<AnsweredPerson>('person_update', {
			personHint: profile.memberID,
			jobTitle: profile.jobTitle,
			teamHint: profile.groupID,
			supervisorHint: profile.supervisorID,
			phoneNumber: profile.phoneNumber,
			hireDate: profile.hireDate,
			...(profile.clearance === undefined ? {} : { clearance: profile.clearance })
		});
	}
	return supabaseOrganizationDirectory();
}

export async function saveSupabaseTeams(groups: OrgGroup[]): Promise<UsersResponse> {
	const held = await invokeTool<AnsweredTeams>('team_list', {});
	await reconcileTeams(held.teams, groups);
	return supabaseOrganizationDirectory();
}

async function reconcileTeams(held: AnsweredTeam[], offered: OrgGroup[]): Promise<void> {
	const known = new Map(held.map((team) => [team.teamID, team]));
	const knownByName = new Map(held.map((team) => [normalizedName(team.name), team]));
	const settledIDByOfferedID = new Map<string, string>();

	for (const [position, group] of offered.entries()) {
		const match = known.get(group.id) ?? knownByName.get(normalizedName(group.name));
		if (match) {
			settledIDByOfferedID.set(group.id, match.teamID);
			continue;
		}
		const made = await invokeTool<AnsweredTeam>('team_add', { name: group.name, position });
		settledIDByOfferedID.set(group.id, made.teamID);
		known.set(made.teamID, made);
	}

	for (const [position, group] of offered.entries()) {
		const teamID = settledIDByOfferedID.get(group.id);
		if (!teamID) continue;
		const parentTeamID = group.parentID ? (settledIDByOfferedID.get(group.parentID) ?? '') : '';
		const settled = known.get(teamID);
		if (
			settled &&
			settled.name === group.name &&
			settled.parentTeamID === parentTeamID &&
			settled.position === position
		) {
			continue;
		}
		await invokeTool<AnsweredTeam>('team_update', {
			teamHint: teamID,
			name: group.name,
			parentHint: parentTeamID,
			position
		});
	}

	for (const team of doomedTeamsDeepestFirst(held, new Set(settledIDByOfferedID.values()))) {
		await invokeTool('team_delete', { teamHint: team.teamID });
	}
}

function doomedTeamsDeepestFirst(held: AnsweredTeam[], kept: Set<string>): AnsweredTeam[] {
	const byID = new Map(held.map((team) => [team.teamID, team]));
	const depthOf = (team: AnsweredTeam): number => {
		let depth = 0;
		let walked = team;
		while (walked.parentTeamID && depth < held.length) {
			const parent = byID.get(walked.parentTeamID);
			if (!parent) break;
			walked = parent;
			depth += 1;
		}
		return depth;
	};
	return held
		.filter((team) => !kept.has(team.teamID))
		.sort((first, second) => depthOf(second) - depthOf(first));
}

function normalizedName(name: string): string {
	return name.trim().toLowerCase();
}

export type SavedMemberProfile = { phoneNumber: string; hireDate: string };

export async function saveOwnSupabaseProfile(
	memberID: string,
	phoneNumber: string,
	hireDate: string
): Promise<SavedMemberProfile> {
	const saved = await invokeTool<AnsweredPerson>('person_update', {
		personHint: memberID,
		phoneNumber,
		hireDate
	});
	return { phoneNumber: saved.phoneNumber ?? '', hireDate: saved.hireDate ?? '' };
}
