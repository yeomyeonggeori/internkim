import { createClient, type SupabaseClient } from '@supabase/supabase-js';

export type ControlPlaneCredentials = {
	projectURL: string;
	serviceRoleKey: string;
};

export type CompanyInput = {
	name: string;
	slug: string;
	country: string;
	locale: string;
	timezone: string;
	workLocations?: string[];
};

export type ProvisionedCompany = {
	companyID: string;
	adminMemberID: string;
};

export function controlPlane(credentials: ControlPlaneCredentials): SupabaseClient {
	return createClient(credentials.projectURL, credentials.serviceRoleKey, {
		auth: { autoRefreshToken: false, persistSession: false },
	});
}

export async function provisionCompany(
	client: SupabaseClient,
	company: CompanyInput,
	adminEmail: string,
): Promise<ProvisionedCompany> {
	const { data: created, error: companyError } = await client
		.from('company')
		.insert({
			name: company.name,
			slug: company.slug,
			country: company.country,
			locale: company.locale,
			timezone: company.timezone,
			work_locations: company.workLocations ?? null,
		})
		.select('id')
		.single();
	if (companyError) throw new Error(`company: ${companyError.message}`);

	const adminMemberID = await addMember(client, created.id, adminEmail, { isAdmin: true });
	return { companyID: created.id, adminMemberID };
}

export async function addMember(
	client: SupabaseClient,
	companyID: string,
	email: string,
	options: { isAdmin?: boolean } = {},
): Promise<string> {
	const { data, error } = await client
		.from('member')
		.upsert(
			{ company_id: companyID, email, is_admin: options.isAdmin ?? false, status: 'pending' },
			{ onConflict: 'email' },
		)
		.select('id')
		.single();
	if (error) throw new Error(`member ${email}: ${error.message}`);
	return data.id;
}

export async function inviteMember(client: SupabaseClient, memberID: string): Promise<void> {
	const { data: member, error: readError } = await client
		.from('member')
		.select('email, status')
		.eq('id', memberID)
		.single();
	if (readError) throw new Error(`member ${memberID}: ${readError.message}`);
	if (!member.email) throw new Error(`member ${memberID} has no address to invite`);

	const { error: inviteError } = await client.auth.admin.inviteUserByEmail(member.email);
	if (inviteError) throw new Error(`invite ${member.email}: ${inviteError.message}`);

	const { error: statusError } = await client
		.from('member')
		.update({ status: 'invited' })
		.eq('id', memberID);
	if (statusError) throw new Error(`member ${memberID}: ${statusError.message}`);
}

export async function linkCredential(
	client: SupabaseClient,
	memberID: string,
	kind: string,
	externalID: string,
): Promise<void> {
	const { error } = await client
		.from('credential')
		.upsert({ member_id: memberID, kind, external_id: externalID }, { onConflict: 'member_id,kind' });
	if (error) throw new Error(`credential ${kind}: ${error.message}`);
}

export async function memberOfPlatformIdentity(
	client: SupabaseClient,
	kind: string,
	externalID: string,
): Promise<string | null> {
	const { data, error } = await client
		.from('credential')
		.select('member_id')
		.eq('kind', kind)
		.eq('external_id', externalID)
		.maybeSingle();
	if (error) throw new Error(`identity ${kind}:${externalID}: ${error.message}`);
	return data?.member_id ?? null;
}
