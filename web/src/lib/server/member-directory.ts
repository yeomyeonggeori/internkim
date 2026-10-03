import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import { memberRoles, memberRoleOf, type MemberRole, type MemberStatus } from '$lib/member-vocabulary';
import { settleSignInOfMember } from './control-plane';

export type CompanyDirectory = { client: SupabaseClient; companyID: string };

type CircleMembership = { member_id: string; circle_id: string };

export function circlesByMemberID(memberships: CircleMembership[]): Map<string, string[]> {
	const circlesOfMember = new Map<string, string[]>();
	for (const { member_id: memberID, circle_id: circle } of memberships) {
		circlesOfMember.set(memberID, [...(circlesOfMember.get(memberID) ?? []), circle]);
	}
	return circlesOfMember;
}

export async function circlesOfTheCompany(directory: CompanyDirectory): Promise<Map<string, string[]>> {
	const memberships = await directory.client
		.from('circle_member')
		.select('member_id, circle_id')
		.eq('company_id', directory.companyID)
		.order('circle_id')
		.returns<CircleMembership[]>();
	if (memberships.error) throw new Error(memberships.error.message);
	return circlesByMemberID(memberships.data ?? []);
}

export const memberWriteSchema = z.object({
	email: z.string().trim().toLowerCase().min(1),
	name: z.string().trim().optional(),
	role: z.enum(memberRoles).optional(),
	note: z.string().trim().optional(),
	messenger: z.record(z.string(), z.string()).optional()
});

export type MemberWrite = z.infer<typeof memberWriteSchema>;

export type SavedMember = {
	memberID: string;
	email: string;
	name: string;
	role: MemberRole;
	status: MemberStatus;
};

export type WithdrawnMember = { memberID: string; email: string; status: 'withdrawn' };

type HeldMember = {
	id: string;
	company_id: string;
	name: string | null;
	note: string | null;
	is_admin: boolean;
	status: MemberStatus;
	messenger: Record<string, string> | null;
};

type SavedRow = { id: string; name: string | null; is_admin: boolean; status: MemberStatus };

const heldColumns = 'id, company_id, name, note, is_admin, status, messenger';
const savedColumns = 'id, name, is_admin, status';

export async function saveMember(directory: CompanyDirectory, write: MemberWrite): Promise<SavedMember> {
	const held = await memberByAddress(directory, write.email);
	const row = {
		company_id: directory.companyID,
		email: write.email,
		name: write.name || held?.name || null,
		note: write.note || held?.note || null,
		is_admin: write.role === undefined ? (held?.is_admin ?? false) : write.role === 'admin',
		messenger: { ...(held?.messenger ?? {}), ...offeredMessengerAccounts(write.messenger) }
	};
	const saved = held
		? await directory.client.from('member').update(row).eq('id', held.id).select(savedColumns).single<SavedRow>()
		: await directory.client.from('member').insert(row).select(savedColumns).single<SavedRow>();
	if (saved.error) throw new Error(saved.error.message);
	return {
		memberID: saved.data.id,
		email: write.email,
		name: saved.data.name ?? '',
		role: memberRoleOf(saved.data.is_admin),
		status: saved.data.status
	};
}

export async function withdrawMember(directory: CompanyDirectory, email: string): Promise<WithdrawnMember | null> {
	const held = await directory.client
		.from('member')
		.select('id')
		.eq('company_id', directory.companyID)
		.eq('email', email)
		.maybeSingle<{ id: string }>();
	if (held.error) throw new Error(held.error.message);
	if (!held.data) return null;
	const withdrawn = await directory.client.from('member').update({ status: 'withdrawn' }).eq('id', held.data.id);
	if (withdrawn.error) throw new Error(withdrawn.error.message);
	await settleSignInOfMember(directory.client, held.data.id);
	return { memberID: held.data.id, email, status: 'withdrawn' };
}

async function memberByAddress(directory: CompanyDirectory, email: string): Promise<HeldMember | null> {
	const held = await directory.client.from('member').select(heldColumns).eq('email', email).maybeSingle<HeldMember>();
	if (held.error) throw new Error(held.error.message);
	if (held.data && held.data.company_id !== directory.companyID) error(409, 'that address belongs to another company');
	return held.data;
}

function offeredMessengerAccounts(offered: Record<string, string> | undefined): Record<string, string> {
	return Object.fromEntries(
		Object.entries(offered ?? {})
			.map(([platform, account]) => [platform, account.trim()])
			.filter(([, account]) => account !== '')
	);
}

export function handleFromEmail(email: string): string {
	return email.trim().toLowerCase().split('@')[0] ?? '';
}
