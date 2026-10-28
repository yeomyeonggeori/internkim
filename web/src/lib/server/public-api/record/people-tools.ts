import {
	addMember,
	AddressBelongsToAnotherCompany,
	AlreadyAMember,
	inviteMember,
	settleSignInOfMember,
	type Invitation
} from '$lib/server/control-plane';
import { removeMemberAsAdministrator, resetPasswordAsAdministrator } from '$lib/server/member-administration';
import { personName } from '$lib/person-name';
import {
	personListPageSize,
	type DirectoryPerson,
	type PersonInviteResult,
	type PersonListResult,
	type PersonPasswordResetResult,
	type PersonWithdrawResult
} from '../catalog/people';
import type { RecordContext } from './company';
import {
	handleOf,
	mentionOf,
	personOfCompanyByID,
	personOfHint,
	type RecordPerson
} from './people';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import { descendantsOfTeam, teamOfHint, teamsOfCompany, type RecordTeam } from './teams';

export type PersonUpdateInput = {
	personHint?: string;
	name?: string;
	isAdmin?: boolean;
	kind?: string;
	jobTitle?: string;
	teamHint?: string;
	supervisorHint?: string;
	phoneNumber?: string;
	hireDate?: string;
	departureDate?: string;
	employmentStatus?: string;
};

export type PersonInviteInput = {
	email?: string;
	name?: string;
	jobTitle?: string;
	teamHint?: string;
};

export type PersonUpdateHomes = {
	account: Record<string, string | boolean>;
	organization: Record<string, string>;
};

export function homesOfPersonUpdate(
	input: PersonUpdateInput,
	resolved: { teamID?: string; supervisorID?: string }
): PersonUpdateHomes {
	const account: Record<string, string | boolean> = {};
	if (input.name !== undefined) account.name = input.name;
	if (input.isAdmin !== undefined) account.isAdmin = input.isAdmin;
	if (input.kind !== undefined) account.kind = input.kind;

	const organization: Record<string, string> = {};
	if (input.jobTitle !== undefined) organization.jobTitle = input.jobTitle;
	if (resolved.teamID !== undefined) organization.groupID = resolved.teamID;
	if (resolved.supervisorID !== undefined) organization.supervisorID = resolved.supervisorID;
	if (input.phoneNumber !== undefined) organization.phoneNumber = input.phoneNumber;
	if (input.hireDate !== undefined) organization.hireDate = input.hireDate;
	if (input.departureDate !== undefined) organization.departureDate = input.departureDate;
	if (input.employmentStatus !== undefined) organization.employmentStatus = input.employmentStatus;

	return { account, organization };
}

export function answeredPerson(
	person: RecordPerson,
	people: RecordPerson[],
	teams: RecordTeam[],
	locale: RecordContext['locale']
): DirectoryPerson {
	const displayName = personName(person.name, locale);
	const mention = mentionOf(displayName);
	const handle = handleOf(person.email);
	const team = teams.find((candidate) => candidate.teamID === person.teamID);
	const supervisor = people.find((candidate) => candidate.personID === person.supervisorID);
	return {
		personID: person.personID,
		name: displayName,
		email: person.email,
		isAdmin: person.isAdmin,
		kind: person.kind,
		employmentStatus: person.employmentStatus,
		...(mention ? { mention } : {}),
		...(handle ? { handle } : {}),
		...(person.jobTitle ? { jobTitle: person.jobTitle } : {}),
		...(person.teamID ? { teamID: person.teamID } : {}),
		...(team ? { teamName: team.name } : {}),
		...(person.supervisorID ? { supervisorID: person.supervisorID } : {}),
		...(supervisor ? { supervisorName: personName(supervisor.name, locale) } : {}),
		...(person.phoneNumber ? { phoneNumber: person.phoneNumber } : {}),
		...(person.hireDate ? { hireDate: person.hireDate } : {}),
		...(person.departureDate ? { departureDate: person.departureDate } : {}),
		...(person.timeZone ? { timeZone: person.timeZone } : {})
	};
}

export type PersonListInput = {
	searchText?: string;
	teamHint?: string;
	limit?: number;
	cursor?: string;
};

type PersonListPosition = { name: string; personID: string };

export async function personList(context: RecordContext, input: PersonListInput = {}): Promise<PersonListResult> {
	const teams = await teamsOfCompany(context.caller);
	const matching = context.people
		.filter(personMatchesSearch(input.searchText ?? ''))
		.filter(personInTeams(teamIDsOfListHint(teams, input.teamHint)))
		.sort(comparePeopleByPosition);
	const after = input.cursor === undefined ? undefined : positionOfCursor(input.cursor);
	const remaining = after ? matching.filter((person) => comparePeopleByPosition(person, after) > 0) : matching;
	const page = remaining.slice(0, input.limit ?? personListPageSize);
	const last = page.at(-1);
	return {
		requesterID: context.requesterID,
		count: page.length,
		totalCount: matching.length,
		people: page.map((person) => answeredPerson(person, context.people, teams, context.locale)),
		...(last && remaining.length > page.length ? { nextCursor: cursorOfPosition(last) } : {})
	};
}

function personMatchesSearch(searchText: string): (person: RecordPerson) => boolean {
	const search = searchText.trim().toLowerCase();
	if (!search) return () => true;
	return (person) => person.name.toLowerCase().includes(search) || person.email.toLowerCase().includes(search);
}

function teamIDsOfListHint(teams: RecordTeam[], teamHint: string | undefined): Set<string> | undefined {
	if (teamHint === undefined) return undefined;
	return descendantsOfTeam(teams, teamOfHint(teams, teamHint).teamID);
}

function personInTeams(teamIDs: Set<string> | undefined): (person: RecordPerson) => boolean {
	if (!teamIDs) return () => true;
	return (person) => teamIDs.has(person.teamID);
}

function comparePeopleByPosition(left: PersonListPosition, right: PersonListPosition): number {
	if (left.name !== right.name) return left.name < right.name ? -1 : 1;
	if (left.personID === right.personID) return 0;
	return left.personID < right.personID ? -1 : 1;
}

function cursorOfPosition(person: PersonListPosition): string {
	const bytes = new TextEncoder().encode(JSON.stringify([person.name, person.personID]));
	return btoa(String.fromCharCode(...bytes));
}

function positionOfCursor(cursor: string): PersonListPosition {
	const decoded = decodedCursor(cursor);
	if (!isPositionPair(decoded)) {
		throw new RecordRefusedTheWrite('cursor is not one a person_list result gave; omit it to start from the first page', 400);
	}
	return { name: decoded[0], personID: decoded[1] };
}

function isPositionPair(value: unknown): value is [string, string] {
	return Array.isArray(value) && value.length === 2 && value.every((part) => typeof part === 'string');
}

function decodedCursor(cursor: string): unknown {
	try {
		const bytes = Uint8Array.from(atob(cursor), (character) => character.charCodeAt(0));
		return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
	} catch {
		return undefined;
	}
}

export async function personUpdate(
	context: RecordContext,
	input: PersonUpdateInput
): Promise<DirectoryPerson> {
	if (!input.personHint?.trim()) throw new Error('a directory change names the person it changes');

	const person = personOfHint(context.people, input.personHint);
	const teams = await teamsOfCompany(context.caller);
	const homes = homesOfPersonUpdate(input, {
		teamID: input.teamHint === undefined ? undefined : teamIDOfHint(teams, input.teamHint),
		supervisorID:
			input.supervisorHint === undefined
				? undefined
				: supervisorIDOfHint(context.people, input.supervisorHint)
	});
	if (Object.keys(homes.account).length + Object.keys(homes.organization).length === 0) {
		throw new Error('a directory change names at least one field to change');
	}

	const { error } = await context.caller.rpc('person_set', {
		target_member: person.personID,
		account_changes: homes.account,
		organization_changes: homes.organization
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	if (input.employmentStatus !== undefined) await settleSignInOfMember(context.accountDirectory, person.personID);

	return answeredPerson(await personWrittenBack(context, person.personID), context.people, teams, context.locale);
}

async function invitationOf(context: RecordContext, personID: string): Promise<Invitation> {
	try {
		return await inviteMember(context.accountDirectory, personID);
	} catch (refusal) {
		if (refusal instanceof AddressBelongsToAnotherCompany) {
			throw new RecordRefusedTheWrite('that address signs in to an account another member holds', 409, 'person_address_taken');
		}
		throw refusal;
	}
}

async function personAddedAt(context: RecordContext, email: string, name: string): Promise<string> {
	try {
		return await addMember(context.accountDirectory, context.companyID, email, { name });
	} catch (refusal) {
		if (refusal instanceof AddressBelongsToAnotherCompany) {
			throw new RecordRefusedTheWrite('that address belongs to another company', 409, 'person_address_taken');
		}
		if (refusal instanceof AlreadyAMember) {
			throw new RecordRefusedTheWrite('that person is already a member of this company', 409, 'person_already_member');
		}
		throw refusal;
	}
}

export async function personInvite(
	context: RecordContext,
	input: PersonInviteInput
): Promise<PersonInviteResult> {
	const email = (input.email ?? '').trim().toLowerCase();
	if (!email.includes('@')) throw new Error('an invitation names the address they sign in with');
	const name = (input.name ?? '').trim();
	if (!name) throw new Error('an invitation names the person it invites');
	refuseUnlessTheRequesterAdministers(context, 'invites people');

	const personID = await personAddedAt(context, email, name);
	const invitation = await invitationOf(context, personID);
	await writeTheOrganizationHomeOfANewPerson(context, personID, input);

	const written = await personWrittenBack(context, personID);
	return {
		personID,
		email: written.email || email,
		name: written.name || name,
		employmentStatus: written.employmentStatus,
		temporaryPassword: invitation.temporaryPassword
	};
}

async function writeTheOrganizationHomeOfANewPerson(
	context: RecordContext,
	personID: string,
	input: PersonInviteInput
): Promise<void> {
	const teams = input.teamHint === undefined ? [] : await teamsOfCompany(context.caller);
	const organization = homesOfPersonUpdate(
		{ personHint: personID, jobTitle: input.jobTitle },
		{ teamID: input.teamHint === undefined ? undefined : teamIDOfHint(teams, input.teamHint) }
	).organization;
	if (Object.keys(organization).length === 0) return;

	const { error } = await context.caller.rpc('person_set', {
		target_member: personID,
		account_changes: {},
		organization_changes: organization
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
}

export async function personWithdraw(
	context: RecordContext,
	input: { personHint?: string; purge?: boolean }
): Promise<PersonWithdrawResult> {
	const person = personOfHint(context.people, input.personHint ?? '');
	const { wasRemoved } = await removeMemberAsAdministrator(context.caller, context.accountDirectory, person.personID, {
		purge: input.purge === true
	});
	return { personID: person.personID, name: personName(person.name, context.locale), email: person.email, isDeleted: wasRemoved };
}

export async function personPasswordReset(
	context: RecordContext,
	input: { personHint?: string }
): Promise<PersonPasswordResetResult> {
	const person = personOfHint(context.people, input.personHint ?? '');
	const invitation = await resetPasswordAsAdministrator(context.caller, context.accountDirectory, person.personID);
	return {
		personID: person.personID,
		email: invitation.email,
		name: personName(person.name, context.locale),
		temporaryPassword: invitation.temporaryPassword
	};
}

export function refuseUnlessTheRequesterAdministers(context: RecordContext, doing: string): void {
	const requester = context.people.find((person) => person.personID === context.requesterID);
	if (requester?.isAdmin) return;
	throw new RecordRefusedTheWrite(`only an administrator ${doing}`, 403, 'administrator_only');
}

function teamIDOfHint(teams: RecordTeam[], hint: string): string {
	const asked = hint.trim();
	if (asked === '') return '';
	return teamOfHint(teams, asked).teamID;
}

function supervisorIDOfHint(people: RecordPerson[], hint: string): string {
	const asked = hint.trim();
	if (asked === '') return '';
	return personOfHint(people, asked, 'supervisor').personID;
}

async function personWrittenBack(context: RecordContext, personID: string): Promise<RecordPerson> {
	const written = await personOfCompanyByID(context.caller, personID);
	if (!written) throw new RecordRefusedTheWrite('the record saved this person and did not answer with it', 502);
	return written;
}
