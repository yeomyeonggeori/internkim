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
			name: '',
		},
		{ onConflict: 'member_id,kind,name' },
	);
	if (error) throw new Error(error.message);
}

export type ConnectedMessengerAccount = {
	memberID: string;
	platform: string;
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

	await rememberMessengerAccount(client, account.memberID, account.platform, account.externalID);
}

// Which account on which messenger this member is, kept on the member. The name
// the messenger has for them is not carried across: the member row already says
// what this company calls them.
async function rememberMessengerAccount(
	client: SupabaseClient,
	memberID: string,
	platform: string,
	externalID: string,
): Promise<void> {
	const member = await client
		.from('member')
		.select('messenger')
		.eq('id', memberID)
		.single<{ messenger: Record<string, string> | null }>();
	if (member.error) throw new Error(member.error.message);

	const { error } = await client
		.from('member')
		.update({ messenger: { ...(member.data.messenger ?? {}), [platform]: externalID } })
		.eq('id', memberID);
	if (error) throw new Error(error.message);
}

// A member is reached by the messenger handle a platform knows them by, or by
// the address the company itself holds, which is the one identity every member
// has whether or not they have been given a messenger account yet.
export async function memberOfCompanyByEmail(
	client: SupabaseClient,
	companyID: string,
	email: string,
): Promise<string | null> {
	const { data, error } = await client
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.ilike('email', email.trim())
		.maybeSingle<{ id: string }>();
	if (error) throw new Error(error.message);
	return data?.id ?? null;
}

export async function membersOfCompanyByExternalID(
	client: SupabaseClient,
	companyID: string,
	platform: string,
): Promise<Map<string, string>> {
	const { data, error } = await client
		.from('member')
		.select('id, messenger')
		.eq('company_id', companyID)
		.returns<{ id: string; messenger: Record<string, string> | null }[]>();
	if (error) throw new Error(error.message);
	return new Map(
		data
			.map((member) => [member.messenger?.[platform] ?? '', member.id] as const)
			.filter(([externalID]) => externalID !== ''),
	);
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

// A company runs one messenger, so a member holds one credential for it, and
// which kind that is belongs to the messenger rather than to the caller.
