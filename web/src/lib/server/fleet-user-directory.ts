import type { SupabaseClient } from '@supabase/supabase-js';
import type { FleetUserRecord, UserRole } from '$lib/types';

export type FleetDirectory = { client: SupabaseClient; companyID: string };

type MemberRow = {
	id: string;
	email: string | null;
	name: string | null;
	note: string | null;
	is_admin: boolean;
	status: string;
	messenger: Record<string, string> | null;
};

const memberColumns = 'id, email, name, note, is_admin, status, messenger';

export async function fleetUserRecords(directory: FleetDirectory): Promise<FleetUserRecord[]> {
	const members = await directory.client
		.from('member')
		.select(memberColumns)
		.eq('company_id', directory.companyID)
		.neq('status', 'withdrawn')
		.order('email')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);
	return (members.data ?? []).filter((member) => member.email).map(recordOf);
}

export async function saveFleetUserRecord(
	directory: FleetDirectory,
	record: FleetUserRecord
): Promise<FleetUserRecord[]> {
	const held = await memberByEmail(directory, record.email);
	const messenger = messengerOf(held?.messenger ?? null, record);
	const row = {
		company_id: directory.companyID,
		email: record.email,
		name: record.name ?? held?.name ?? null,
		note: record.note ?? held?.note ?? null,
		is_admin: record.role === 'admin',
		...(Object.keys(messenger).length > 0 ? { messenger } : {})
	};
	const saved = held
		? await directory.client.from('member').update(row).eq('id', held.id)
		: await directory.client.from('member').insert(row);
	if (saved.error) throw new Error(saved.error.message);
	return fleetUserRecords(directory);
}

export async function withdrawFleetUser(directory: FleetDirectory, email: string): Promise<FleetUserRecord[]> {
	const held = await memberByEmail(directory, email);
	if (!held) return fleetUserRecords(directory);
	const withdrawn = await directory.client
		.from('member')
		.update({ status: 'withdrawn' })
		.eq('id', held.id);
	if (withdrawn.error) throw new Error(withdrawn.error.message);
	return fleetUserRecords(directory);
}

async function memberByEmail(directory: FleetDirectory, email: string): Promise<MemberRow | null> {
	const held = await directory.client
		.from('member')
		.select(memberColumns)
		.eq('company_id', directory.companyID)
		.eq('email', email)
		.maybeSingle<MemberRow>();
	if (held.error) throw new Error(held.error.message);
	return held.data;
}

function recordOf(member: MemberRow): FleetUserRecord {
	const email = (member.email ?? '').toLowerCase();
	const messenger = member.messenger ?? {};
	const name = member.name?.trim() ?? '';
	return {
		handle: messenger.mattermostUsername || handleFromEmail(email),
		...(name ? { name } : {}),
		email,
		...(member.note ? { note: member.note } : {}),
		role: (member.is_admin ? 'admin' : 'member') as UserRole,
		...(messenger.mattermost ? { mattermostUserID: messenger.mattermost } : {}),
		...(messenger.mattermostUsername ? { mattermostUsername: messenger.mattermostUsername } : {}),
		status: member.status,
		isIncomplete: !name
	};
}

function messengerOf(held: Record<string, string> | null, record: FleetUserRecord): Record<string, string> {
	const messenger = { ...(held ?? {}) };
	if (record.mattermostUserID) messenger.mattermost = record.mattermostUserID;
	const username = record.mattermostUsername || record.handle;
	if (username) messenger.mattermostUsername = username;
	return messenger;
}

function handleFromEmail(email: string): string {
	return email.split('@')[0] ?? '';
}
