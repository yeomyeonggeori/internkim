import type { SupabaseClient } from '@supabase/supabase-js';

export type MemberCredential = {
	kind: string;
	externalID: string;
	secret: string;
};

type CredentialRow = { external_id: string | null };

export async function memberCredential(
	client: SupabaseClient,
	memberID: string,
	kind: string,
): Promise<MemberCredential | null> {
	const row = await readRow(client, memberID, kind);
	if (!row?.external_id) return null;

	const { data, error } = await client.rpc('read_member_secret', {
		target_member: memberID,
		target_kind: kind,
	});
	if (error) throw new Error(error.message);
	if (typeof data !== 'string' || data === '') return null;
	return { kind, externalID: row.external_id, secret: data };
}

export async function keepMemberCredential(
	client: SupabaseClient,
	memberID: string,
	credential: MemberCredential,
): Promise<void> {
	const vaultSecretID = await storeSecret(client, memberID, credential.kind, credential.secret);

	const { error } = await client.from('credential').upsert(
		{
			member_id: memberID,
			company_id: null,
			kind: credential.kind,
			external_id: credential.externalID,
			vault_secret_id: vaultSecretID,
		},
		{ onConflict: 'member_id,kind' },
	);
	if (error) throw new Error(error.message);
}

export type ConnectedMessengerAccount = {
	memberID: string;
	kind: string;
	externalID: string;
	name: string;
	secret: string;
};

export async function connectMessengerAccount(
	client: SupabaseClient,
	companyID: string,
	account: ConnectedMessengerAccount,
): Promise<void> {
	await keepMemberCredential(client, account.memberID, {
		kind: account.kind,
		externalID: account.externalID,
		secret: account.secret,
	});

	const { error } = await client.from('contact').upsert(
		{
			company_id: companyID,
			platform: account.kind,
			external_id: account.externalID,
			name: account.name,
			member_id: account.memberID,
		},
		{ onConflict: 'company_id,platform,external_id' },
	);
	if (error) throw new Error(error.message);
}

export async function externalIDsWithACredential(
	client: SupabaseClient,
	companyID: string,
	kind: string,
): Promise<string[]> {
	const linked = await membersOfCompanyByExternalID(client, companyID, kind);
	if (linked.size === 0) return [];

	const { data, error } = await client
		.from('credential')
		.select('member_id, external_id')
		.eq('kind', kind)
		.in('member_id', [...linked.values()])
		.returns<{ member_id: string; external_id: string | null }[]>();
	if (error) throw new Error(error.message);

	const held: string[] = [];
	for (const row of data) {
		if (!row.external_id) continue;
		if (!(await secretIsReadable(client, row.member_id, kind))) continue;
		held.push(row.external_id);
	}
	return held;
}

async function secretIsReadable(
	client: SupabaseClient,
	memberID: string,
	kind: string,
): Promise<boolean> {
	const { data, error } = await client.rpc('read_member_secret', {
		target_member: memberID,
		target_kind: kind,
	});
	if (error) throw new Error(error.message);
	return typeof data === 'string' && data !== '';
}

export async function membersOfCompanyByExternalID(
	client: SupabaseClient,
	companyID: string,
	platform: string,
): Promise<Map<string, string>> {
	const { data, error } = await client
		.from('contact')
		.select('external_id, member_id')
		.eq('company_id', companyID)
		.eq('platform', platform)
		.not('member_id', 'is', null)
		.returns<{ external_id: string; member_id: string }[]>();
	if (error) throw new Error(error.message);
	return new Map(data.map((contact) => [contact.external_id, contact.member_id]));
}

async function readRow(
	client: SupabaseClient,
	memberID: string,
	kind: string,
): Promise<CredentialRow | null> {
	const { data, error } = await client
		.from('credential')
		.select('external_id')
		.eq('member_id', memberID)
		.eq('kind', kind)
		.maybeSingle<CredentialRow>();
	if (error) throw new Error(error.message);
	return data;
}

async function storeSecret(
	client: SupabaseClient,
	memberID: string,
	kind: string,
	secret: string,
): Promise<string> {
	const { data, error } = await client.rpc('write_member_secret', {
		target_member: memberID,
		target_kind: kind,
		secret_value: secret,
	});
	if (error) throw new Error(error.message);
	if (typeof data !== 'string') throw new Error('the vault returned no secret id');
	return data;
}
