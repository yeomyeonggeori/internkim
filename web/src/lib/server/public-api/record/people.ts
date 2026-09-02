import type { SupabaseClient } from '@supabase/supabase-js';
import { handleFromEmail } from '$lib/server/fleet-user-directory';
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
};

type MemberRow = { id: string; name: string | null; email: string | null };

export function displayNameOf(member: { name: string | null; email: string | null }): string {
	return member.name?.trim() || (member.email ?? '').split('@')[0];
}

export function mentionOf(email: string): string {
	const handle = handleFromEmail(email);
	return handle ? `@${handle}` : '';
}

export async function peopleOfCompany(caller: SupabaseClient): Promise<RecordPerson[]> {
	const { data, error } = await caller
		.from('member')
		.select('id, name, email')
		.neq('status', 'withdrawn')
		.order('name')
		.returns<MemberRow[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((member) => ({
		personID: member.id,
		name: displayNameOf(member),
		email: member.email ?? ''
	}));
}

const personMatcher: HintMatcher<RecordPerson> = {
	identifiersOf: (person) => [person.personID, person.email, mentionOf(person.email)],
	titleOf: (person) => person.name,
	nearnessTo: (person, hint) =>
		Math.max(
			typoNearness(normalized(hint), normalized(person.name)),
			emailNearness(normalized(hint), normalized(person.email)),
			typoNearness(withoutAtSign(normalized(hint)), handleFromEmail(person.email))
		)
};

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
	const mention = mentionOf(person.email);
	return {
		id: person.personID,
		label: person.name,
		...(person.email ? { email: person.email } : {}),
		...(mention ? { mention } : {})
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
