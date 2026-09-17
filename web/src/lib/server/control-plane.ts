import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import {
	fullPublicAPIPermission,
	publicAPIPermissionOf,
	type PublicAPIPermission,
} from '$lib/public-api-permission';
import { memberOfCompanyByEmail, membersOfCompanyByExternalID } from './member-credential';
import { personalAccessTokenCredentialKind } from './public-api/catalog/credential';
import { recordTokenFor } from './record-token';

export type ControlPlaneCredentials = {
	projectURL: string;
	serviceRoleKey: string;
};

export type MemberCredentials = {
	projectURL: string;
	publishableKey: string;
};

export type SigningCredentials = ControlPlaneCredentials & { signingKey: string };

export type PlaneCredentials = SigningCredentials & MemberCredentials;

export function planeCredentialsOf(
	environment: Record<string, string | undefined>,
): PlaneCredentials | null {
	const plane = {
		projectURL: environment.SUPABASE_URL ?? '',
		publishableKey: environment.SUPABASE_PUBLISHABLE_KEY ?? '',
		serviceRoleKey: environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '',
		signingKey: environment.SUPABASE_JWT_SIGNING_KEY ?? '',
	};
	if (!plane.projectURL || !plane.publishableKey || !plane.serviceRoleKey || !plane.signingKey) {
		return null;
	}
	return plane;
}

export type CompanyInput = {
	name: string;
	slug: string;
	country: string;
	locale: string;
	timezone: string;
	workLocations?: { name: string; color?: string }[];
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

export function asMember(credentials: MemberCredentials, accessToken: string): SupabaseClient {
	return createClient(credentials.projectURL, credentials.publishableKey, {
		auth: { autoRefreshToken: false, persistSession: false },
		global: { headers: { Authorization: `Bearer ${accessToken}` } },
	});
}

export type AdminCaller = { memberID: string; companyID: string };

export async function adminCallerOf(client: SupabaseClient): Promise<AdminCaller | null> {
	const { data: account } = await client.auth.getUser();
	if (!account.user) return null;
	const { data: member } = await client
		.from('member')
		.select('id, company_id, is_admin')
		.eq('user_id', account.user.id)
		.maybeSingle();
	if (!member?.is_admin) return null;
	return { memberID: member.id, companyID: member.company_id };
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
	options: { isAdmin?: boolean; name?: string } = {},
): Promise<string> {
	const name = (options.name ?? '').trim();
	const { data, error } = await client
		.from('member')
		.upsert(
			{
				company_id: companyID,
				email,
				is_admin: options.isAdmin ?? false,
				status: 'pending',
				...(name ? { name } : {}),
			},
			{ onConflict: 'email' },
		)
		.select('id')
		.single();
	if (error) throw new Error(`member ${email}: ${error.message}`);
	return data.id;
}

// Somebody who worked here leaves their attendance, their leave and the tasks
// they carried behind, and those records name the row they belong to. Marking
// them withdrawn is what a colleague leaving looks like: the record still
// reads and every screen stops showing them.
//
// Purging is the other case, an address typed by mistake with nothing to keep.
// Postgres decides whether it may go: a reference that refuses the delete is a
// record worth keeping, so it falls back to withdrawing and says so.
export async function removeMember(
	client: SupabaseClient,
	companyID: string,
	memberID: string,
	options: { purge?: boolean } = {},
): Promise<{ wasRemoved: boolean }> {
	if (options.purge) {
		const removed = await client.from('member').delete().eq('company_id', companyID).eq('id', memberID);
		if (!removed.error) return { wasRemoved: true };
		if (removed.error.code !== foreignKeyViolation) throw new Error(removed.error.message);
	}
	const withdrawn = await client
		.from('member')
		.update({ status: 'withdrawn' })
		.eq('company_id', companyID)
		.eq('id', memberID);
	if (withdrawn.error) throw new Error(withdrawn.error.message);
	return { wasRemoved: false };
}

const foreignKeyViolation = '23503';

export type FoundedCompany = {
	companyID: string;
	adminMemberID: string;
	invitations: Invitation[];
};

export type ClaimedMember = { memberID: string; companyID: string; hasJustArrived: boolean };

type MemberRow = { id: string; company_id: string; status: string };

export async function claimMemberFor(
	client: SupabaseClient,
	accountID: string,
	email: string,
): Promise<ClaimedMember | null> {
	const found = (await memberHeldByAccount(client, accountID)) ?? (await memberWaitingForAddress(client, email));
	if (!found) return null;
	const hasJustArrived = hasNotArrivedYet(found.status);
	if (hasJustArrived) await markArrived(client, found.id, accountID, email);
	return { memberID: found.id, companyID: found.company_id, hasJustArrived };
}

async function memberHeldByAccount(client: SupabaseClient, accountID: string): Promise<MemberRow | null> {
	const { data, error } = await client
		.from('member')
		.select('id, company_id, status')
		.eq('user_id', accountID)
		.maybeSingle();
	if (error) throw new Error(error.message);
	return data;
}

async function memberWaitingForAddress(client: SupabaseClient, email: string): Promise<MemberRow | null> {
	const { data, error } = await client.from('member').select('id, company_id, status').eq('email', email).maybeSingle();
	if (error) throw new Error(error.message);
	return data;
}

function hasNotArrivedYet(status: string): boolean {
	return status !== 'active' && !hasLeftTheCompany(status);
}

function hasLeftTheCompany(status: string): boolean {
	return status === 'departed' || status === 'withdrawn';
}

async function markArrived(client: SupabaseClient, memberID: string, accountID: string, email: string): Promise<void> {
	const claimed = await client.from('member').update({ user_id: accountID, status: 'active' }).eq('id', memberID);
	if (claimed.error) throw new Error(`claiming ${email}: ${claimed.error.message}`);

	const firstDay = await client
		.from('member')
		.update({ joined_at: new Date().toISOString() })
		.eq('id', memberID)
		.is('joined_at', null);
	if (firstDay.error) throw new Error(`claiming ${email}: ${firstDay.error.message}`);
}

export async function foundCompany(
	client: SupabaseClient,
	founder: { accountID: string; email: string },
	company: CompanyInput,
	invited: string[],
): Promise<FoundedCompany> {
	const { companyID, adminMemberID } = await provisionCompany(client, company, founder.email);

	const { error } = await client
		.from('member')
		.update({ user_id: founder.accountID, status: 'active', joined_at: new Date().toISOString() })
		.eq('id', adminMemberID);
	if (error) throw new Error(`founder: ${error.message}`);

	const invitations: Invitation[] = [];
	for (const email of invited) {
		if (email === founder.email) continue;
		const memberID = await addMember(client, companyID, email);
		invitations.push(await inviteMember(client, memberID));
	}
	return { companyID, adminMemberID, invitations };
}

export type Invitation = {
	memberID: string;
	email: string;
	temporaryPassword: string;
};

export async function inviteMember(client: SupabaseClient, memberID: string): Promise<Invitation> {
	const invitation = await issueTemporaryPassword(client, memberID);

	const { error } = await client.from('member').update({ status: 'invited' }).eq('id', memberID);
	if (error) throw new Error(`member ${memberID}: ${error.message}`);

	return invitation;
}

export async function resetMemberPassword(
	client: SupabaseClient,
	memberID: string,
): Promise<Invitation> {
	return issueTemporaryPassword(client, memberID);
}

async function issueTemporaryPassword(
	client: SupabaseClient,
	memberID: string,
): Promise<Invitation> {
	const { data: member, error: readError } = await client
		.from('member')
		.select('email')
		.eq('id', memberID)
		.single();
	if (readError) throw new Error(`member ${memberID}: ${readError.message}`);
	if (!member.email) throw new Error(`member ${memberID} has no address`);

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
		.upsert({ member_id: memberID, kind, name: '', external_id: externalID }, { onConflict: 'member_id,kind,name' });
	if (error) throw new Error(`credential ${kind}: ${error.message}`);
}

export type MemberSession = {
	memberID: string;
	accessToken: string;
	expiresAt: number;
};

type MemberAccount = { user_id: string | null; email: string | null; status: string };

async function memberAccountOf(client: SupabaseClient, memberID: string): Promise<MemberAccount> {
	const { data, error } = await client
		.from('member')
		.select('user_id, email, status')
		.eq('id', memberID)
		.single<MemberAccount>();
	if (error) throw new Error(`member ${memberID}: ${error.message}`);
	return data;
}

async function openAccountFor(client: SupabaseClient, memberID: string, email: string | null): Promise<string> {
	if (!email) throw new Error(`member ${memberID} has no address to open an account with`);
	const { data, error } = await client.auth.admin.createUser({ email, email_confirm: true });
	if (error) throw new Error(`account for ${email}: ${error.message}`);
	return data.user.id;
}

export async function sessionForMember(
	credentials: SigningCredentials,
	memberID: string,
): Promise<MemberSession> {
	const client = controlPlane(credentials);
	const member = await memberAccountOf(client, memberID);
	if (hasLeftTheCompany(member.status)) {
		throw new Error(`member ${memberID} has left and cannot be acted for`);
	}
	const userID = member.user_id ?? (await openAccountFor(client, memberID, member.email));
	const token = await recordTokenFor(credentials.signingKey, credentials.projectURL, {
		userID,
		email: member.email,
	});
	return { memberID, ...token };
}

export type AgentKey = { agentID: string; companyID: string; apiKey: string };

// A company holds one agent per name, and revoking does not give the name back.
// So reissuing writes the new hash onto the row that stands, the way
// digest_agent_key_keep does; a plain insert answers a duplicate key to anyone
// who rotates a key twice.
export async function issueAgentKey(
	client: SupabaseClient,
	companyID: string,
	name: string,
	options: { replaceStanding?: boolean } = {},
): Promise<AgentKey> {
	const apiKey = [...crypto.getRandomValues(new Uint8Array(32))]
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
	const record = { company_id: companyID, name, api_key_hash: await hashOf(apiKey) };
	const written = options.replaceStanding
		? client.from('agent').upsert({ ...record, revoked_at: null }, { onConflict: 'company_id,name' })
		: client.from('agent').insert(record);
	const { data, error } = await written.select('id').single();
	if (error) throw new Error(`agent ${name}: ${error.message}`);
	return { agentID: data.id, companyID, apiKey };
}

const personalAccessTokenPrefix = 'ik_';

function storedTokenPermission(stored: unknown): PublicAPIPermission {
	const permission = publicAPIPermissionOf(stored);
	if (!permission) throw new Error(`personal access token: ${String(stored)} is no rung of the ladder`);
	return permission;
}

export type PersonalAccessTokenSession = MemberSession & {
	permission: PublicAPIPermission;
	tokenName: string;
};

export type PersonalAccessToken = {
	name: string;
	permission: PublicAPIPermission;
};

export async function issuePersonalAccessToken(
	client: SupabaseClient,
	memberID: string,
	name: string,
	permission: PublicAPIPermission = fullPublicAPIPermission,
): Promise<string> {
	const apiKey =
		personalAccessTokenPrefix +
		[...crypto.getRandomValues(new Uint8Array(32))]
			.map((byte) => byte.toString(16).padStart(2, '0'))
			.join('');
	const { error } = await client
		.from('credential')
		.upsert(
			{
				member_id: memberID,
				kind: personalAccessTokenCredentialKind,
				name,
				external_id: await hashOf(apiKey),
				permission,
			},
			{ onConflict: 'member_id,kind,name' },
		);
	if (error) throw new Error(`personal access token ${name}: ${error.message}`);
	return apiKey;
}

export async function personalAccessTokens(client: SupabaseClient, memberID: string): Promise<PersonalAccessToken[]> {
	const { data, error } = await client
		.from('credential')
		.select('name, permission')
		.eq('member_id', memberID)
		.eq('kind', personalAccessTokenCredentialKind)
		.order('name');
	if (error) throw new Error(`personal access tokens: ${error.message}`);
	return (data ?? []).map((row) => ({
		name: row.name as string,
		permission: storedTokenPermission(row.permission),
	}));
}

export async function forgetPersonalAccessToken(
	client: SupabaseClient,
	memberID: string,
	name: string,
): Promise<boolean> {
	const { data, error } = await client
		.from('credential')
		.delete()
		.eq('member_id', memberID)
		.eq('kind', personalAccessTokenCredentialKind)
		.eq('name', name)
		.select('name');
	if (error) throw new Error(`personal access token ${name}: ${error.message}`);
	return (data ?? []).length > 0;
}

export function isPersonalAccessToken(presented: string): boolean {
	return presented.startsWith(personalAccessTokenPrefix);
}

export class TokenOwnerHasLeft extends Error {
	constructor() {
		super('token owner is not active member');
	}
}

type PersonalAccessTokenRow = {
	member_id: string;
	permission: unknown;
	name: string | null;
	member: { status: string } | null;
};

// The caller of a personal access token is the member it belongs to, so this issues that
// member's own session and never the company's. The row that names the member
// names the rung too, so nobody downstream asks for it a second time.
export async function sessionForPersonalAccessToken(
	credentials: SigningCredentials,
	apiKey: string,
): Promise<PersonalAccessTokenSession | null> {
	const client = controlPlane(credentials);
	const { data, error } = await client
		.from('credential')
		.select('member_id, permission, name, member(status)')
		.eq('kind', personalAccessTokenCredentialKind)
		.eq('external_id', await hashOf(apiKey))
		.maybeSingle<PersonalAccessTokenRow>();
	if (error) throw new Error(`personal access token: ${error.message}`);
	if (!data) return null;
	if (hasLeftTheCompany(data.member?.status ?? '')) throw new TokenOwnerHasLeft();
	const session = await sessionForMember(credentials, data.member_id);
	return {
		...session,
		permission: storedTokenPermission(data.permission),
		tokenName: typeof data.name === 'string' ? data.name : '',
	};
}

const fleetCredentialKind = 'fleet';

export async function claimFleetForCompany(
	client: SupabaseClient,
	companyID: string,
	fleetID: string,
): Promise<void> {
	const { error } = await client
		.from('credential')
		.upsert(
			{ company_id: companyID, kind: fleetCredentialKind, external_id: fleetID },
			{ onConflict: 'company_id,kind' },
		);
	if (error) throw new Error(`fleet ${fleetID}: ${error.message}`);
}

export async function companyOfFleet(client: SupabaseClient, fleetID: string): Promise<string | null> {
	const { data, error } = await client
		.from('credential')
		.select('company_id')
		.eq('kind', fleetCredentialKind)
		.eq('external_id', fleetID)
		.maybeSingle();
	if (error) throw new Error(`fleet ${fleetID}: ${error.message}`);
	return data?.company_id ?? null;
}

export async function replaceFleetAgentKey(
	client: SupabaseClient,
	companyID: string,
	fleetID: string,
): Promise<AgentKey> {
	const name = `${fleetCredentialKind} ${fleetID}`;
	return issueAgentKey(client, companyID, name, { replaceStanding: true });
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
		.select('id, company_id, revoked_at, last_seen_at')
		.eq('api_key_hash', await hashOf(apiKey))
		.maybeSingle();
	if (error) throw new Error(`agent key: ${error.message}`);
	if (!data || data.revoked_at) return null;

	if (isLastSeenStale(data.last_seen_at)) {
		await client.from('agent').update({ last_seen_at: new Date().toISOString() }).eq('id', data.id);
	}
	return { agentID: data.id, companyID: data.company_id };
}

const lastSeenFreshForMilliseconds = 60_000;

export function isLastSeenStale(lastSeenAt: string | null, now: number = Date.now()): boolean {
	if (!lastSeenAt) return true;
	const seenAt = Date.parse(lastSeenAt);
	return Number.isNaN(seenAt) || now - seenAt >= lastSeenFreshForMilliseconds;
}

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

export async function sessionForPlatformIdentity(
	credentials: SigningCredentials,
	apiKey: string,
	kind: string,
	externalID: string,
): Promise<MemberSession> {
	const client = controlPlane(credentials);
	const agent = await agentOfKey(client, apiKey);
	if (!agent) throw new Error('that key belongs to no agent');

	const memberID =
		kind === 'email'
			? await memberOfCompanyByEmail(client, agent.companyID, externalID)
			: (await membersOfCompanyByExternalID(client, agent.companyID, kind)).get(externalID);
	if (!memberID) throw new Error(`no member here has ${kind} identity ${externalID}`);

	return sessionForMember(credentials, memberID);
}

export type HostSession = {
	companyID: string;
	accessToken: string;
	expiresAt: number;
};

export function hostAddressOf(companyID: string): string {
	return `host.${companyID}@agent.internkim.invalid`;
}

export async function sessionForHost(
	credentials: SigningCredentials,
	apiKey: string,
): Promise<HostSession> {
	const client = controlPlane(credentials);
	const agent = await agentOfKey(client, apiKey);
	if (!agent) throw new Error('that key belongs to no agent');

	const address = hostAddressOf(agent.companyID);
	const userID = await keepHostAccount(client, address, agent.companyID);
	const token = await recordTokenFor(credentials.signingKey, credentials.projectURL, {
		userID,
		email: address,
		appMetadata: { company_id: agent.companyID },
	});
	return { companyID: agent.companyID, ...token };
}

async function keepHostAccount(
	client: SupabaseClient,
	address: string,
	companyID: string,
): Promise<string> {
	const { data: accounts, error: listError } = await client.auth.admin.listUsers();
	if (listError) throw new Error(listError.message);
	const account = accounts.users.find((user) => user.email === address);
	const appMetadata = { company_id: companyID };

	if (!account) {
		const { data, error } = await client.auth.admin.createUser({
			email: address,
			email_confirm: true,
			app_metadata: appMetadata,
		});
		if (error) throw new Error(`host account for ${companyID}: ${error.message}`);
		return data.user.id;
	}
	if (account.app_metadata?.company_id === companyID) return account.id;
	const { error } = await client.auth.admin.updateUserById(account.id, {
		app_metadata: appMetadata,
	});
	if (error) throw new Error(`host account for ${companyID}: ${error.message}`);
	return account.id;
}
