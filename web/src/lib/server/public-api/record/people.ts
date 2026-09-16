import type { SupabaseClient } from '@supabase/supabase-js';
import { handleFromEmail } from '$lib/server/fleet-user-directory';
import { personName } from '$lib/person-name';
import { emailNearness, typoNearness } from './hint-nearness';
import {
	HintRefused,
	normalized,
	resolveHint,
	type HintCandidate,
	type HintMatcher,
	type HintSubject
} from './hint-resolution';

export type RecordPerson = {
	personID: string;
	name: string;
	email: string;
	isAdmin: boolean;
	clearance: number;
	employmentStatus: string;
	jobTitle: string;
	teamID: string;
	supervisorID: string;
	phoneNumber: string;
	hireDate: string;
	timeZone: string;
};

type MemberRow = {
	id: string;
	name: string | null;
	email: string | null;
	is_admin: boolean;
	clearance: number;
	status: string;
	job_title: string | null;
	team_id: string | null;
	supervisor_id: string | null;
	phone_number: string | null;
	joined_at: string | null;
	timezone: string | null;
};

const directoryColumns =
	'id, name, email, is_admin, clearance, status, job_title, team_id, supervisor_id, phone_number, joined_at, timezone';

export function displayNameOf(member: { name: string | null; email: string | null }): string {
	return member.name?.trim() || (member.email ?? '').split('@')[0];
}

export function mentionOf(displayName: string): string {
	const named = displayName.trim();
	return named ? `@${named}` : '';
}

export function handleOf(email: string): string {
	const handle = handleFromEmail(email);
	return handle ? `@${handle}` : '';
}

export async function peopleOfCompany(caller: SupabaseClient): Promise<RecordPerson[]> {
	const { data, error } = await caller
		.from('member')
		.select(directoryColumns)
		.neq('status', 'withdrawn')
		.order('name')
		.returns<MemberRow[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map(recordPersonOf);
}

function recordPersonOf(member: MemberRow): RecordPerson {
	return {
		personID: member.id,
		name: displayNameOf(member),
		email: member.email ?? '',
		isAdmin: member.is_admin,
		clearance: member.clearance,
		employmentStatus: member.status,
		jobTitle: member.job_title ?? '',
		teamID: member.team_id ?? '',
		supervisorID: member.supervisor_id ?? '',
		phoneNumber: member.phone_number ?? '',
		hireDate: member.joined_at ? String(member.joined_at).slice(0, 10) : '',
		timeZone: member.timezone ?? ''
	};
}

export async function personOfCompanyByID(
	caller: SupabaseClient,
	personID: string
): Promise<RecordPerson | null> {
	const { data, error } = await caller
		.from('member')
		.select(directoryColumns)
		.eq('id', personID)
		.maybeSingle<MemberRow>();
	if (error) throw new Error(error.message);
	return data ? recordPersonOf(data) : null;
}

const personMatcher: HintMatcher<RecordPerson> = {
	identifiersOf: (person) => [person.personID, person.email, handleOf(person.email)],
	exactTitleAliasesOf: localizedPersonTitles,
	titleOf: (person) => person.name,
	nearnessTo: (person, hint) =>
		Math.max(
			typoNearness(normalized(hint), normalized(person.name)),
			emailNearness(normalized(hint), normalized(person.email)),
			typoNearness(withoutAtSign(normalized(hint)), handleFromEmail(person.email))
		)
};

function localizedPersonTitles(person: RecordPerson): string[] {
	const koreanName = personName(person.name, 'ko');
	const englishName = personName(person.name, 'en');
	return [koreanName, `@${koreanName}`, englishName, `@${englishName}`];
}

function withoutAtSign(value: string): string {
	return value.startsWith('@') ? value.slice(1) : value;
}

export function personOfHint(
	people: RecordPerson[],
	hint: string,
	subject: HintSubject = 'person'
): RecordPerson {
	const resolution = resolveHint(hint, people, personMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(subject, hint.trim(), resolution.outcome, resolution.candidates.map(candidateOf));
}

export function candidateOf(person: RecordPerson): HintCandidate {
	const handle = handleOf(person.email);
	return {
		id: person.personID,
		label: person.name,
		...(person.email ? { email: person.email } : {}),
		...(handle ? { handle } : {})
	};
}

export function peopleOfHints(
	people: RecordPerson[],
	hints: string[],
	subject: HintSubject = 'person'
): RecordPerson[] {
	const resolved: RecordPerson[] = [];
	for (const hint of hints) {
		const person = personOfHint(people, hint, subject);
		if (!resolved.some((held) => held.personID === person.personID)) resolved.push(person);
	}
	return resolved;
}
