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

export class MemberOfAnotherCompany extends Error {
	constructor(readonly memberID: string) {
		super(`member ${memberID} belongs to another company`);
	}
}

export async function memberBelongsToCompany(
	client: SupabaseClient,
	memberID: string,
	companyID: string,
): Promise<boolean> {
	const member = await client
		.from('member')
		.select('company_id')
		.eq('id', memberID)
		.maybeSingle<{ company_id: string }>();
	if (member.error) throw new Error(member.error.message);
	return member.data?.company_id === companyID;
}

export async function connectMessengerAccount(
	client: SupabaseClient,
	companyID: string,
	account: ConnectedMessengerAccount,
): Promise<void> {
	if (!(await memberBelongsToCompany(client, account.memberID, companyID))) {
		throw new MemberOfAnotherCompany(account.memberID);
	}
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

// Who to tell is a question about people, and the company's own directory is
// what answers it. Asking a messenger's account list instead means anybody
// without an account there is silently not told.
export async function membersOfCompanyByEmail(
	client: SupabaseClient,
	companyID: string,
): Promise<Map<string, string>> {
	const { data, error } = await client
		.from('member')
		.select('id, email')
		.eq('company_id', companyID)
		.neq('status', 'withdrawn')
		.returns<{ id: string; email: string | null }[]>();
	if (error) throw new Error(error.message);
	return new Map(
		data
			.map((member) => [(member.email ?? '').trim().toLowerCase(), member.id] as const)
			.filter(([email]) => email !== ''),
	);
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

// The messenger account on the member is a projection of the credential the
// person was issued. Any row the projection misses or contradicts is set from
// the credential, so an account issued before the projection existed still
// resolves.
export async function reconcileMessengerAccounts(
	client: SupabaseClient,
	companyID: string,
	platform: string,
	kind: string,
): Promise<string[]> {
	const members = await client
		.from('member')
		.select('id, email, messenger')
		.eq('company_id', companyID)
		.returns<{ id: string; email: string | null; messenger: Record<string, string> | null }[]>();
	if (members.error) throw new Error(members.error.message);

	const credentials = await client
		.from('credential')
		.select('member_id, external_id')
		.eq('kind', kind)
		.returns<{ member_id: string; external_id: string | null }[]>();
	if (credentials.error) throw new Error(credentials.error.message);
	const issuedByMember = new Map(
		credentials.data
			.filter((credential) => credential.external_id)
			.map((credential) => [credential.member_id, credential.external_id as string]),
	);

	const healed: string[] = [];
	for (const member of members.data) {
		const issued = issuedByMember.get(member.id);
		if (!issued || member.messenger?.[platform] === issued) continue;
		const { error } = await client
			.from('member')
			.update({ messenger: { ...(member.messenger ?? {}), [platform]: issued } })
			.eq('id', member.id);
		if (error) throw new Error(error.message);
		healed.push(member.email ?? member.id);
	}
	return healed;
}
