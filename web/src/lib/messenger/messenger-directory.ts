import { supabase } from '$lib/supabase';
import type { MessengerPerson } from './messenger-api';

export type MessengerDirectory = {
	nameOfMember: Map<string, string>;
	nameOfExternal: Map<string, string>;
	memberOfExternal: Map<string, string>;
	externalOfMember: Map<string, string>;
	memberOfEmail: Map<string, string>;
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
		),
		externalOfMember: new Map(
			people.data.filter((person) => person.member_id).map((person) => [person.member_id as string, person.external_id])
		),
		memberOfEmail: new Map(
			members.data.filter((member) => member.email).map((member) => [(member.email as string).toLowerCase(), member.id])
		)
	};
}

export function externalIDFor(person: { memberID?: string; email?: string }, directory: MessengerDirectory): string {
	const memberID = person.memberID || directory.memberOfEmail.get((person.email ?? '').trim().toLowerCase());
	return memberID ? (directory.externalOfMember.get(memberID) ?? '') : '';
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

export async function isMessengerConnected(): Promise<boolean> {
	const { count, error } = await supabase().from('contact').select('external_id', { count: 'exact', head: true });
	if (error) return false;
	return (count ?? 0) > 0;
}

export async function haveIConnectedMyMessenger(): Promise<boolean> {
	const client = supabase();
	const { data } = await client.auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return false;
	const member = await client.from('member').select('id').eq('user_id', accountID).maybeSingle<{ id: string }>();
	if (member.error || !member.data) return false;
	const { count } = await client
		.from('contact')
		.select('external_id', { count: 'exact', head: true })
		.eq('member_id', member.data.id);
	return (count ?? 0) > 0;
}
