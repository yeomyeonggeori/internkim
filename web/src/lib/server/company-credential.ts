import type { SupabaseClient } from '@supabase/supabase-js';

// What a company shares about a connection: enough to reach the server, never
// the password. The password lives in the vault and is read only when a request
// actually needs to talk to that server.
export type CompanyConnection = {
	kind: string;
	host: string;
	settings: Record<string, unknown>;
	hasSecret: boolean;
};

type CredentialRow = { kind: string; external_id: string | null; settings: Record<string, unknown>; vault_secret_id: string | null };

export async function companyConnections(client: SupabaseClient, companyID: string): Promise<CompanyConnection[]> {
	const { data, error } = await client
		.from('credential')
		.select('kind, external_id, settings, vault_secret_id')
		.eq('company_id', companyID)
		.returns<CredentialRow[]>();
	if (error) throw new Error(error.message);
	return data.map((row) => ({
		kind: row.kind,
		host: row.external_id ?? '',
		settings: row.settings,
		hasSecret: row.vault_secret_id !== null
	}));
}

export async function saveCompanyConnection(
	client: SupabaseClient,
	companyID: string,
	connection: { kind: string; host: string; settings: Record<string, unknown>; secret?: string },
): Promise<void> {
	const existing = await client
		.from('credential')
		.select('vault_secret_id')
		.eq('company_id', companyID)
		.eq('kind', connection.kind)
		.maybeSingle<{ vault_secret_id: string | null }>();
	if (existing.error) throw new Error(existing.error.message);

	const vaultSecretID = connection.secret
		? await storeSecret(client, `${companyID}:${connection.kind}`, connection.secret, existing.data?.vault_secret_id ?? null)
		: (existing.data?.vault_secret_id ?? null);

	const { error } = await client.from('credential').upsert(
		{
			company_id: companyID,
			member_id: null,
			kind: connection.kind,
			external_id: connection.host,
			settings: connection.settings,
			vault_secret_id: vaultSecretID
		},
		{ onConflict: 'company_id,kind' }
	);
	if (error) throw new Error(error.message);
}

export async function forgetCompanyConnection(client: SupabaseClient, companyID: string, kind: string): Promise<void> {
	const { error } = await client.from('credential').delete().eq('company_id', companyID).eq('kind', kind);
	if (error) throw new Error(error.message);
}

export async function secretOfConnection(
	client: SupabaseClient,
	companyID: string,
	kind: string,
): Promise<string | null> {
	const credential = await client
		.from('credential')
		.select('vault_secret_id')
		.eq('company_id', companyID)
		.eq('kind', kind)
		.maybeSingle<{ vault_secret_id: string | null }>();
	if (credential.error) throw new Error(credential.error.message);
	if (!credential.data?.vault_secret_id) return null;

	const { data, error } = await client.rpc('read_company_secret', { secret_id: credential.data.vault_secret_id });
	if (error) throw new Error(error.message);
	return typeof data === 'string' ? data : null;
}

async function storeSecret(
	client: SupabaseClient,
	name: string,
	secret: string,
	replacing: string | null,
): Promise<string> {
	const { data, error } = await client.rpc('write_company_secret', {
		secret_id: replacing,
		secret_name: name,
		secret_value: secret
	});
	if (error) throw new Error(error.message);
	if (typeof data !== 'string') throw new Error('the vault returned no secret id');
	return data;
}
