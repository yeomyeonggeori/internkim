import {
	addMember,
	AddressBelongsToAnotherCompany,
	AlreadyAMember,
	inviteMember,
	settleSignInOfMember,
	type Invitation
} from '$lib/server/control-plane';
import { personName } from '$lib/person-name';
import type { DirectoryPerson, PersonInviteResult } from '../catalog/people';
import type { RecordContext } from './company';
import {
	handleOf,
	mentionOf,
	personOfCompanyByID,
	personOfHint,
	type RecordPerson
} from './people';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import { teamOfHint, teamsOfCompany, type RecordTeam } from './teams';

export type PersonUpdateInput = {
	personHint?: string;
	name?: string;
	isAdmin?: boolean;
	clearance?: number;
	jobTitle?: string;
	teamHint?: string;
	supervisorHint?: string;
	phoneNumber?: string;
	hireDate?: string;
	employmentStatus?: string;
};

export type PersonInviteInput = {
	email?: string;
	name?: string;
	jobTitle?: string;
	teamHint?: string;
};

export type PersonUpdateHomes = {
	account: Record<string, string | boolean | number>;
	organization: Record<string, string>;
};

export function homesOfPersonUpdate(
	input: PersonUpdateInput,
	resolved: { teamID?: string; supervisorID?: string }
): PersonUpdateHomes {
	const account: Record<string, string | boolean | number> = {};
	if (input.name !== undefined) account.name = input.name;
	if (input.isAdmin !== undefined) account.isAdmin = input.isAdmin;
	if (input.clearance !== undefined) account.clearance = input.clearance;

	const organization: Record<string, string> = {};
	if (input.jobTitle !== undefined) organization.jobTitle = input.jobTitle;
	if (resolved.teamID !== undefined) organization.groupID = resolved.teamID;
	if (resolved.supervisorID !== undefined) organization.supervisorID = resolved.supervisorID;
	if (input.phoneNumber !== undefined) organization.phoneNumber = input.phoneNumber;
	if (input.hireDate !== undefined) organization.hireDate = input.hireDate;
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
		clearance: person.clearance,
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
		...(person.timeZone ? { timeZone: person.timeZone } : {})
	};
}

export async function personList(context: RecordContext) {
	const teams = await teamsOfCompany(context.caller);
	return {
		requesterID: context.requesterID,
		count: context.people.length,
		people: context.people.map((person) => answeredPerson(person, context.people, teams, context.locale))
	};
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

function refuseUnlessTheRequesterAdministers(context: RecordContext, doing: string): void {
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
