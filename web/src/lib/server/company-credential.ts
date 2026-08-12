import type { SupabaseClient } from '@supabase/supabase-js';

export type CompanyConnection = {
	kind: string;
	host: string;
	settings: Record<string, unknown>;
};

type CredentialRow = { kind: string; external_id: string | null; settings: Record<string, unknown> };

export async function companyConnections(client: SupabaseClient, companyID: string): Promise<CompanyConnection[]> {
	const { data, error } = await client
		.from('credential')
		.select('kind, external_id, settings')
		.eq('company_id', companyID)
		.returns<CredentialRow[]>();
	if (error) throw new Error(error.message);
	return data.map((row) => ({
		kind: row.kind,
		host: row.external_id ?? '',
		settings: row.settings
	}));
}

export async function saveCompanyConnection(
	client: SupabaseClient,
	companyID: string,
	connection: { kind: string; host: string; settings: Record<string, unknown> },
): Promise<void> {
	const { error } = await client.from('credential').upsert(
		{
			company_id: companyID,
			member_id: null,
			kind: connection.kind,
			external_id: connection.host,
			settings: connection.settings
		},
		{ onConflict: 'company_id,kind' }
	);
	if (error) throw new Error(error.message);
}

export async function forgetCompanyConnection(client: SupabaseClient, companyID: string, kind: string): Promise<void> {
	const { error } = await client.from('credential').delete().eq('company_id', companyID).eq('kind', kind);
	if (error) throw new Error(error.message);
}
