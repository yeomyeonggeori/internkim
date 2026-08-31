import type { SupabaseClient } from '@supabase/supabase-js';

export type RecordPerson = {
	personID: string;
	name: string;
	email: string;
};

type MemberRow = { id: string; name: string | null; email: string | null };

export class HintUnresolved extends Error {
	constructor(
		readonly hint: string,
		readonly candidates: string[]
	) {
		super(
			candidates.length === 0
				? `nobody here goes by ${hint}`
				: `${hint} could be ${candidates.join(', ')}; name one of them exactly`
		);
		this.name = 'HintUnresolved';
	}
}

export function displayNameOf(member: { name: string | null; email: string | null }): string {
	return member.name?.trim() || (member.email ?? '').split('@')[0];
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

// An identifier is matched exactly, because a near miss is a different person.
// A name is matched by containment, and only a single match resolves: two
// people who both answer to the hint are an ambiguity the caller settles.
export function personOfHint(people: RecordPerson[], hint: string): RecordPerson {
	const asked = hint.trim();
	if (!asked) throw new HintUnresolved(hint, []);

	const exact = people.find(
		(person) => person.personID === asked || person.email.toLowerCase() === asked.toLowerCase()
	);
	if (exact) return exact;

	const named = people.filter((person) => person.name.toLowerCase() === asked.toLowerCase());
	if (named.length === 1) return named[0];

	const contained = named.length > 0 ? named : people.filter((person) => person.name.includes(asked));
	if (contained.length === 1) return contained[0];
	throw new HintUnresolved(hint, contained.map((person) => person.name));
}

export function peopleOfHints(people: RecordPerson[], hints: string[]): RecordPerson[] {
	const resolved: RecordPerson[] = [];
	for (const hint of hints) {
		const person = personOfHint(people, hint);
		if (!resolved.some((held) => held.personID === person.personID)) resolved.push(person);
	}
	return resolved;
}
