import { supabase } from '$lib/supabase';
import { personName } from '$lib/person-name';
import type { Locale } from '$lib/i18n/locale.svelte';
import type { MessengerPerson } from './messenger-api';

export type MessengerDirectory = {
	nameOfMember: Map<string, string>;
	nameOfExternal: Map<string, string>;
	memberOfExternal: Map<string, string>;
	externalsOfMember: Map<string, string[]>;
	memberOfEmail: Map<string, string>;
};

type ContactRow = { name: string; messenger: Record<string, string> | null };
type MemberRow = { id: string; name: string | null; email: string | null; messenger: Record<string, string> | null };

// A member's messenger account is part of who they are, and is kept on the
// member. contact is the company's address book for people who are not members
// of it, so a contact never names one.
export async function fetchMessengerDirectory(): Promise<MessengerDirectory> {
	const client = supabase();
	const members = await client
		.from('member')
		.select('id, name, email, messenger')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const people = await client.from('contact').select('name, messenger').returns<ContactRow[]>();
	if (people.error) throw new Error(people.error.message);

	const accounts = members.data.flatMap((member) =>
		Object.values(member.messenger ?? {})
			.filter(Boolean)
			.map((externalID) => [externalID, member.id] as const)
	);

	return {
		nameOfMember: new Map(members.data.map((member) => [member.id, displayNameOf(member)])),
		nameOfExternal: new Map(
			people.data.flatMap((person) =>
				Object.values(person.messenger ?? {})
					.filter(Boolean)
					.map((externalID) => [externalID, person.name] as const)
			)
		),
		memberOfExternal: new Map(accounts),
		externalsOfMember: externalsByMember(accounts),
		memberOfEmail: new Map(
			members.data.filter((member) => member.email).map((member) => [(member.email as string).toLowerCase(), member.id])
		)
	};
}

// A member can hold an account on more than one messenger, and a conversation
// names them by whichever one it came from, so every account they hold is kept.
function externalsByMember(accounts: readonly (readonly [string, string])[]): Map<string, string[]> {
	const externals = new Map<string, string[]>();
	for (const [externalID, memberID] of accounts) {
		externals.set(memberID, [...(externals.get(memberID) ?? []), externalID]);
	}
	return externals;
}

export function externalIDsOfMember(directory: MessengerDirectory, memberID: string): string[] {
	return directory.externalsOfMember.get(memberID) ?? [];
}

// Every account a person holds, for a caller that would rather ask after all of
// them than decide which messenger the company is on. Nothing here knows that;
// the host does.
export function accountsHeldBy(
	person: { memberID?: string; email?: string },
	directory: MessengerDirectory
): string[] {
	const memberID = person.memberID || directory.memberOfEmail.get((person.email ?? '').trim().toLowerCase());
	return memberID ? externalIDsOfMember(directory, memberID) : [];
}

export function externalIDFor(person: { memberID?: string; email?: string }, directory: MessengerDirectory): string {
	return accountsHeldBy(person, directory)[0] ?? '';
}

export function personKey(person: MessengerPerson): string {
	if (person.memberID) return `member:${person.memberID}`;
	return `external:${person.externalID ?? ''}`;
}

// A member's name is recorded given name first, so how it is written belongs to
// the language the reader picked; see personName. Everyone else in the address
// book keeps the name they arrived with — an entry there need not be a person's
// name at all, and rejoining "Intern Kim" or "Google Meet" as though it were
// one produces something nobody has ever been called.
export function personLabel(person: MessengerPerson, directory: MessengerDirectory, locale: Locale): string {
	const memberID = person.memberID ?? (person.externalID ? directory.memberOfExternal.get(person.externalID) : undefined);
	if (memberID) return personName(directory.nameOfMember.get(memberID) ?? '', locale);
	return directory.nameOfExternal.get(person.externalID ?? '') ?? '';
}

function displayNameOf(member: MemberRow): string {
	return member.name || (member.email ?? '').split('@')[0];
}

export async function haveIConnectedMyMessenger(): Promise<boolean> {
	const client = supabase();
	const { data } = await client.auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return false;
	const member = await client
		.from('member')
		.select('messenger')
		.eq('user_id', accountID)
		.maybeSingle<{ messenger: Record<string, string> | null }>();
	if (member.error || !member.data) return false;
	return Object.values(member.data.messenger ?? {}).some(Boolean);
}
