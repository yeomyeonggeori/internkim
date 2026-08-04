import { supabase } from '$lib/supabase';
import type { MessengerPerson } from './messenger-api';

export type MessengerDirectory = {
	nameOfMember: Map<string, string>;
	nameOfExternal: Map<string, string>;
	memberOfExternal: Map<string, string>;
};

type ContactRow = { platform: string; external_id: string; name: string; member_id: string | null };
type MemberRow = { id: string; name: string | null; email: string | null };

export async function fetchMessengerDirectory(): Promise<MessengerDirectory> {
	const client = supabase();
	const members = await client
		.from('member')
		.select('id, name, email')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const people = await client
		.from('contact')
		.select('platform, external_id, name, member_id')
		.returns<ContactRow[]>();
	if (people.error) throw new Error(people.error.message);

	return {
		nameOfMember: new Map(members.data.map((member) => [member.id, displayNameOf(member)])),
		nameOfExternal: new Map(people.data.map((person) => [person.external_id, person.name])),
		memberOfExternal: new Map(
			people.data.filter((person) => person.member_id).map((person) => [person.external_id, person.member_id as string])
		)
	};
}

export function personKey(person: MessengerPerson): string {
	if (person.memberID) return `member:${person.memberID}`;
	return `external:${person.externalID ?? ''}`;
}

export function personLabel(person: MessengerPerson, directory: MessengerDirectory): string {
	const memberID = person.memberID ?? (person.externalID ? directory.memberOfExternal.get(person.externalID) : undefined);
	if (memberID) return directory.nameOfMember.get(memberID) ?? '';
	return directory.nameOfExternal.get(person.externalID ?? '') ?? '';
}

function displayNameOf(member: MemberRow): string {
	return member.name || (member.email ?? '').split('@')[0];
}
