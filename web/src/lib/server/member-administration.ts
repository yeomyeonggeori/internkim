import type { SupabaseClient } from '@supabase/supabase-js';
import {
	AddressBelongsToAnotherCompany,
	adminCallerOf,
	removeMember,
	resetMemberPassword,
	type AdminCaller,
	type Invitation
} from '$lib/server/control-plane';
import { RecordRefusedTheWrite } from '$lib/server/public-api/record/tasks';

export type MemberRemoval = { memberID: string; wasRemoved: boolean };

type CompanyMember = { id: string; is_admin: boolean };

export async function removeMemberAsAdministrator(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	memberID: string,
	options: { purge: boolean }
): Promise<MemberRemoval> {
	const administrator = await administratorOf(caller, 'removes people');
	if (!memberID) throw new RecordRefusedTheWrite('which member to remove', 400, 'person_not_named');
	if (memberID === administrator.memberID) {
		throw new RecordRefusedTheWrite('an admin does not remove themselves', 400, 'person_is_requester');
	}
	await memberOfCompany(accountDirectory, administrator, memberID);

	const { wasRemoved } = await removeMember(accountDirectory, administrator.companyID, memberID, options);
	return { memberID, wasRemoved };
}

export async function resetPasswordAsAdministrator(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	memberID: string
): Promise<Invitation> {
	const administrator = await administratorOf(caller, 'resets a password');
	if (!memberID) throw new RecordRefusedTheWrite('which member to reset', 400, 'person_not_named');
	const member = await memberOfCompany(accountDirectory, administrator, memberID);
	if (member.is_admin && member.id !== administrator.memberID) {
		throw new RecordRefusedTheWrite(
			'an administrator does not reset the password of another administrator',
			403,
			'administrator_password_reserved'
		);
	}

	try {
		return await resetMemberPassword(accountDirectory, member.id);
	} catch (refusal) {
		if (refusal instanceof AddressBelongsToAnotherCompany) {
			throw new RecordRefusedTheWrite('that address signs in to an account this member does not hold', 409, 'person_address_taken');
		}
		throw refusal;
	}
}

async function administratorOf(caller: SupabaseClient, doing: string): Promise<AdminCaller> {
	const administrator = await adminCallerOf(caller);
	if (!administrator) throw new RecordRefusedTheWrite(`only an admin ${doing}`, 403, 'administrator_only');
	return administrator;
}

async function memberOfCompany(
	accountDirectory: SupabaseClient,
	administrator: AdminCaller,
	memberID: string
): Promise<CompanyMember> {
	const { data: member, error } = await accountDirectory
		.from('member')
		.select('id, is_admin')
		.eq('company_id', administrator.companyID)
		.eq('id', memberID)
		.maybeSingle<CompanyMember>();
	if (error) throw new Error(`member ${memberID}: ${error.message}`);
	if (!member) throw new RecordRefusedTheWrite('no such member in this company', 404, 'person_not_found');
	return member;
}
