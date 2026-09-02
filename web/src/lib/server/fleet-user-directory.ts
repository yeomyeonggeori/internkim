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

type CircleRow = { name: string; circle_member: { member_id: string }[] | null };

export function circleNamesByMemberID(circles: CircleRow[]): Map<string, string[]> {
	const namesByMemberID = new Map<string, string[]>();
	for (const circle of circles) {
		for (const membership of circle.circle_member ?? []) {
			const held = namesByMemberID.get(membership.member_id) ?? [];
			held.push(circle.name);
			namesByMemberID.set(membership.member_id, held);
		}
	}
	return namesByMemberID;
}

async function circlesOfTheCompany(directory: FleetDirectory): Promise<Map<string, string[]>> {
	const circles = await directory.client
		.from('circle')
		.select('name, circle_member(member_id)')
		.eq('company_id', directory.companyID)
		.order('name')
		.returns<CircleRow[]>();
	if (circles.error) throw new Error(circles.error.message);
	return circleNamesByMemberID(circles.data ?? []);
}

export async function fleetUserRecords(directory: FleetDirectory): Promise<FleetUserRecord[]> {
	const members = await directory.client
		.from('member')
		.select(memberColumns)
		.eq('company_id', directory.companyID)
		.neq('status', 'withdrawn')
		.order('email')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);
	const circles = await circlesOfTheCompany(directory);
	return (members.data ?? [])
		.filter((member) => member.email)
		.map((member) => recordOf(member, circles.get(member.id) ?? []));
}

export async function saveFleetUserRecord(
	directory: FleetDirectory,
	record: FleetUserRecord
): Promise<FleetUserRecord[]> {
	const held = await memberByEmail(directory, record.email);
	const messenger = messengerOf(held?.messenger ?? null);
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

// Withdrawing is right for somebody who worked here: their attendance, their
// leave and the tasks they carried are the company's record, and the row is what
// those rows point at. Somebody added by mistake has no record to keep, so the
// row goes. Postgres decides which of the two this is - a reference that refuses
// the delete is a record worth keeping - rather than a list of tables kept here
// that the schema would outgrow.
export async function removeFleetUser(
	directory: FleetDirectory,
	email: string
): Promise<{ records: FleetUserRecord[]; wasRemoved: boolean }> {
	const held = await memberByEmail(directory, email);
	if (!held) return { records: await fleetUserRecords(directory), wasRemoved: false };
	const removed = await directory.client.from('member').delete().eq('id', held.id);
	if (removed.error) {
		if (removed.error.code !== foreignKeyViolation) throw new Error(removed.error.message);
		return { records: await withdrawFleetUser(directory, email), wasRemoved: false };
	}
	return { records: await fleetUserRecords(directory), wasRemoved: true };
}

const foreignKeyViolation = '23503';

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

function recordOf(member: MemberRow, circles: string[]): FleetUserRecord {
	const email = (member.email ?? '').toLowerCase();
	const messenger = member.messenger ?? {};
	const name = member.name?.trim() ?? '';
	return {
		memberID: member.id,
		handle: handleFromEmail(email),
		...(name ? { name } : {}),
		email,
		...(member.note ? { note: member.note } : {}),
		role: (member.is_admin ? 'admin' : 'member') as UserRole,
		...(circles.length > 0 ? { circles } : {}),
		status: member.status,
		isIncomplete: !name
	};
}

// A handle is the company's own, derived from the address, so nothing writes
// it into the map of messenger accounts. What a person already has there is
// carried forward untouched: those are their accounts, not this app's to edit.
function messengerOf(held: Record<string, string> | null): Record<string, string> {
	return { ...(held ?? {}) };
}

export function handleFromEmail(email: string): string {
	return email.trim().toLowerCase().split('@')[0] ?? '';
}
