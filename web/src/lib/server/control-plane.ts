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

export type Invitation = {
	memberID: string;
	email: string;
	temporaryPassword: string;
};

// Mail cannot be relied on here — the built-in sender only reaches the project team
// and reports success either way — so an invitation is a password the admin hands
// over. The person changes it once they are in.
export async function inviteMember(client: SupabaseClient, memberID: string): Promise<Invitation> {
	const { data: member, error: readError } = await client
		.from('member')
		.select('email')
		.eq('id', memberID)
		.single();
	if (readError) throw new Error(`member ${memberID}: ${readError.message}`);
	if (!member.email) throw new Error(`member ${memberID} has no address to invite`);

	const temporaryPassword = temporaryPasswordValue();
	const { data: accounts, error: listError } = await client.auth.admin.listUsers();
	if (listError) throw new Error(listError.message);
	const account = accounts.users.find((user) => user.email === member.email);

	if (account) {
		const { error } = await client.auth.admin.updateUserById(account.id, {
			password: temporaryPassword,
			email_confirm: true,
		});
		if (error) throw new Error(`password for ${member.email}: ${error.message}`);
	} else {
		const { error } = await client.auth.admin.createUser({
			email: member.email,
			password: temporaryPassword,
			email_confirm: true,
		});
		if (error) throw new Error(`account for ${member.email}: ${error.message}`);
	}

	const { error: statusError } = await client
		.from('member')
		.update({ status: 'invited' })
		.eq('id', memberID);
	if (statusError) throw new Error(`member ${memberID}: ${statusError.message}`);

	return { memberID, email: member.email, temporaryPassword };
}

function temporaryPasswordValue(): string {
	const alphabet = 'abcdefghijkmnpqrstuvwxyz23456789';
	const bytes = crypto.getRandomValues(new Uint8Array(12));
	const value = [...bytes].map((byte) => alphabet[byte % alphabet.length]).join('');
	return `${value.slice(0, 4)}-${value.slice(4, 8)}-${value.slice(8, 12)}`;
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

export type MemberSession = {
	memberID: string;
	accessToken: string;
	expiresAt: number;
};

// The host acts as the member who spoke, so it needs that member's session rather
// than a key of its own. Supabase can hand one over without sending mail: generate
// the link it would have mailed, then redeem it here.
export async function sessionForMember(
	credentials: ControlPlaneCredentials,
	memberID: string,
): Promise<MemberSession> {
	const client = controlPlane(credentials);
	const { data: member, error: memberError } = await client
		.from('member')
		.select('email, status')
		.eq('id', memberID)
		.single();
	if (memberError) throw new Error(`member ${memberID}: ${memberError.message}`);
	if (!member.email) throw new Error(`member ${memberID} has no address to sign in as`);
	if (member.status === 'departed' || member.status === 'withdrawn') {
		throw new Error(`member ${memberID} has left and cannot be acted for`);
	}

	const { data: link, error: linkError } = await client.auth.admin.generateLink({
		type: 'magiclink',
		email: member.email,
	});
	if (linkError) throw new Error(`link for ${member.email}: ${linkError.message}`);

	// Redeeming the link signs this client in as the member, so it happens on a
	// throwaway one. Doing it on the caller's client would quietly drop the control
	// plane to that member's privileges for everything afterwards.
	const redeemer = controlPlane(credentials);
	const { data: session, error: verifyError } = await redeemer.auth.verifyOtp({
		token_hash: link.properties.hashed_token,
		type: 'magiclink',
	});
	if (verifyError) throw new Error(`session for ${member.email}: ${verifyError.message}`);
	if (!session.session) throw new Error(`no session came back for ${member.email}`);

	return {
		memberID,
		accessToken: session.session.access_token,
		expiresAt: session.session.expires_at ?? 0,
	};
}

export type AgentKey = { agentID: string; companyID: string; apiKey: string };

// Issued once when an agent is installed. Only the hash is stored, so what the
// control plane holds cannot be used to be an agent.
export async function issueAgentKey(
	client: SupabaseClient,
	companyID: string,
	name: string,
): Promise<AgentKey> {
	const apiKey = [...crypto.getRandomValues(new Uint8Array(32))]
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
	const { data, error } = await client
		.from('agent')
		.insert({ company_id: companyID, name, api_key_hash: await hashOf(apiKey) })
		.select('id')
		.single();
	if (error) throw new Error(`agent ${name}: ${error.message}`);
	return { agentID: data.id, companyID, apiKey };
}

export async function revokeAgent(client: SupabaseClient, agentID: string): Promise<void> {
	const { error } = await client
		.from('agent')
		.update({ revoked_at: new Date().toISOString() })
		.eq('id', agentID);
	if (error) throw new Error(`agent ${agentID}: ${error.message}`);
}

export async function agentOfKey(
	client: SupabaseClient,
	apiKey: string,
): Promise<{ agentID: string; companyID: string } | null> {
	const { data, error } = await client
		.from('agent')
		.select('id, company_id, revoked_at')
		.eq('api_key_hash', await hashOf(apiKey))
		.maybeSingle();
	if (error) throw new Error(`agent key: ${error.message}`);
	if (!data || data.revoked_at) return null;

	await client.from('agent').update({ last_seen_at: new Date().toISOString() }).eq('id', data.id);
	return { agentID: data.id, companyID: data.company_id };
}

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

// What an agent actually asks for: the member behind a platform identity, and a
// session to act as them. It proves which company it belongs to first, and the
// member has to be in that same company — otherwise one company's agent could act
// for another's people.
export async function sessionForPlatformIdentity(
	credentials: ControlPlaneCredentials,
	apiKey: string,
	kind: string,
	externalID: string,
): Promise<MemberSession> {
	const client = controlPlane(credentials);
	const agent = await agentOfKey(client, apiKey);
	if (!agent) throw new Error('that key belongs to no agent');

	const memberID = await memberOfPlatformIdentity(client, kind, externalID);
	if (!memberID) throw new Error(`no member has ${kind} identity ${externalID}`);

	const { data: member, error } = await client
		.from('member')
		.select('company_id')
		.eq('id', memberID)
		.single();
	if (error) throw new Error(error.message);
	if (member.company_id !== agent.companyID) {
		throw new Error('that member belongs to another company');
	}

	return sessionForMember(credentials, memberID);
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
