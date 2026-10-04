import { createClient, type SupabaseClient, type User } from '@supabase/supabase-js';
import {
	connectedAppPermissionOf,
	fullPublicAPIPermission,
	publicAPIPermissionOf,
	unchosenConnectedAppPermission,
	type ConnectedAppPermission,
	type PublicAPIPermission,
} from '$lib/public-api-permission';
import { memberOfCompanyByEmail, membersOfCompanyByExternalID } from './member-credential';
import { connectedAppCredentialKind, personalAccessTokenCredentialKind } from './public-api/catalog/credential';
import { recordTokenFor, verifiedRecordToken } from './record-token';
import { defaultTokenLifetimeDays, expiryAfter } from '$lib/token-lifetime';
import { z } from 'zod';

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
		serviceRoleKey: environment.SUPABASE_SECRET_KEY ?? '',
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
	const { data: isAdministrator, error: adminError } = await client.rpc('is_company_admin');
	if (adminError) throw new Error(`administrator: ${adminError.message}`);
	if (isAdministrator !== true) return null;
	const { data: member } = await client
		.from('member')
		.select('id, company_id')
		.eq('user_id', account.user.id)
		.maybeSingle();
	if (!member) return null;
	return { memberID: member.id, companyID: member.company_id };
}

export async function promoteToAdministrator(caller: SupabaseClient, memberID: string): Promise<void> {
	const { error } = await caller.rpc('person_set', {
		target_member: memberID,
		account_changes: { isAdmin: true },
		organization_changes: {},
	});
	if (error) throw new Error(`administrator ${memberID}: ${error.message}`);
}

export class CompanyAddressTaken extends Error {
	constructor(readonly slug: string) {
		super(`${slug} is already a company's address`);
	}
}

const uniqueViolation = '23505';

export async function provisionCompany(
	client: SupabaseClient,
	company: CompanyInput,
	adminEmail: string,
	adminName = '',
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
	if (companyError?.code === uniqueViolation) throw new CompanyAddressTaken(company.slug);
	if (companyError) throw new Error(`company: ${companyError.message}`);

	try {
		const adminMemberID = await addMember(client, created.id, adminEmail, { isAdmin: true, name: adminName });
		return { companyID: created.id, adminMemberID };
	} catch (refusal) {
		await client.from('company').delete().eq('id', created.id);
		throw refusal;
	}
}

export class AddressBelongsToAnotherCompany extends Error {
	constructor(readonly email: string) {
		super(`${email} belongs to another company`);
	}
}

export class AlreadyAMember extends Error {
	constructor(readonly email: string) {
		super(`${email} is already a member of this company`);
	}
}

export async function addMember(
	client: SupabaseClient,
	companyID: string,
	email: string,
	options: { isAdmin?: boolean; name?: string } = {},
): Promise<string> {
	if (isHostAddress(email)) throw new AddressBelongsToAnotherCompany(email);
	const held = await memberWaitingForAddress(client, email);
	if (held) return memberWhoMayBeInvitedAgain(held, companyID, email);

	const name = (options.name ?? '').trim();
	const { data, error } = await client
		.from('member')
		.insert({
			company_id: companyID,
			email,
			is_admin: options.isAdmin ?? false,
			status: 'pending',
			...(name ? { name } : {}),
		})
		.select('id')
		.single();
	if (error) throw new Error(`member ${email}: ${error.message}`);
	return data.id;
}

function memberWhoMayBeInvitedAgain(held: MemberRow, companyID: string, email: string): string {
	if (held.company_id !== companyID) throw new AddressBelongsToAnotherCompany(email);
	if (held.status === 'active') throw new AlreadyAMember(email);
	return held.id;
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
	const accountID = await accountOfMember(client, memberID);
	if (options.purge) {
		const removed = await client.from('member').delete().eq('company_id', companyID).eq('id', memberID);
		if (!removed.error) {
			await keepAccountSignedIn(client, accountID, false);
			return { wasRemoved: true };
		}
		if (removed.error.code !== foreignKeyViolation) throw new Error(removed.error.message);
	}
	const withdrawn = await client
		.from('member')
		.update({ status: 'withdrawn' })
		.eq('company_id', companyID)
		.eq('id', memberID);
	if (withdrawn.error) throw new Error(withdrawn.error.message);
	await keepAccountSignedIn(client, accountID, false);
	return { wasRemoved: false };
}

const foreignKeyViolation = '23503';

const bannedForGood = '876000h';

async function accountOfMember(client: SupabaseClient, memberID: string): Promise<string | null> {
	const { data, error } = await client
		.from('member')
		.select('user_id')
		.eq('id', memberID)
		.maybeSingle<{ user_id: string | null }>();
	if (error) throw new Error(`account of ${memberID}: ${error.message}`);
	return data?.user_id ?? null;
}

async function keepAccountSignedIn(client: SupabaseClient, accountID: string | null, isMember: boolean): Promise<void> {
	if (!accountID) return;
	const { error } = await client.auth.admin.updateUserById(accountID, {
		ban_duration: isMember ? 'none' : bannedForGood,
	});
	if (error) throw new Error(`sign-in of ${accountID}: ${error.message}`);
}

export async function settleSignInOfMember(client: SupabaseClient, memberID: string): Promise<void> {
	const { data, error } = await client
		.from('member')
		.select('user_id, status')
		.eq('id', memberID)
		.maybeSingle<{ user_id: string | null; status: string }>();
	if (error) throw new Error(`sign-in of ${memberID}: ${error.message}`);
	if (!data) return;
	await keepAccountSignedIn(client, data.user_id, !hasLeftTheCompany(data.status));
}

export type LeaverAccount = { memberID: string; accountID: string };

export type SignInOfMembersWhoLeft = {
	closed: LeaverAccount[];
	alreadyClosed: LeaverAccount[];
	withoutAccount: string[];
};

export async function closeSignInOfMembersWhoLeft(
	client: SupabaseClient,
	options: { isDryRun: boolean },
): Promise<SignInOfMembersWhoLeft> {
	const leavers = await membersWhoLeft(client);
	const settled: SignInOfMembersWhoLeft = { closed: [], alreadyClosed: [], withoutAccount: [] };
	for (const leaver of leavers) {
		if (!leaver.user_id) {
			settled.withoutAccount.push(leaver.id);
			continue;
		}
		const account = { memberID: leaver.id, accountID: leaver.user_id };
		if (await isAccountClosed(client, account.accountID)) {
			settled.alreadyClosed.push(account);
			continue;
		}
		if (!options.isDryRun) await keepAccountSignedIn(client, account.accountID, false);
		settled.closed.push(account);
	}
	return settled;
}

async function membersWhoLeft(client: SupabaseClient): Promise<{ id: string; user_id: string | null }[]> {
	const { data, error } = await client
		.from('member')
		.select('id, user_id')
		.in('status', statusesOfMembersWhoLeft)
		.order('id')
		.returns<{ id: string; user_id: string | null }[]>();
	if (error) throw new Error(`members who left: ${error.message}`);
	return data;
}

async function isAccountClosed(client: SupabaseClient, accountID: string): Promise<boolean> {
	const { data, error } = await client.auth.admin.getUserById(accountID);
	if (error) throw new Error(`account ${accountID}: ${error.message}`);
	const bannedUntil = data.user.banned_until;
	return bannedUntil !== undefined && new Date(bannedUntil).getTime() > Date.now();
}

export type FoundedCompany = {
	companyID: string;
	adminMemberID: string;
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

const statusesOfMembersWhoLeft: string[] = ['departed', 'withdrawn'];

function hasLeftTheCompany(status: string): boolean {
	return statusesOfMembersWhoLeft.includes(status);
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
	founder: { accountID: string; email: string; name: string },
	company: CompanyInput,
): Promise<FoundedCompany> {
	const { companyID, adminMemberID } = await provisionCompany(client, company, founder.email, founder.name);

	const { error } = await client
		.from('member')
		.update({ user_id: founder.accountID, status: 'active', joined_at: new Date().toISOString() })
		.eq('id', adminMemberID);
	if (error) {
		await client.from('company').delete().eq('id', companyID);
		throw new Error(`founder: ${error.message}`);
	}
	return { companyID, adminMemberID };
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
	await settleSignInOfMember(client, memberID);

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
		.select('id, email, user_id')
		.eq('id', memberID)
		.single<{ id: string; email: string | null; user_id: string | null }>();
	if (readError) throw new Error(`member ${memberID}: ${readError.message}`);
	if (!member.email) throw new Error(`member ${memberID} has no address`);

	const temporaryPassword = temporaryPasswordValue();
	const account = await accountOfAddress(client, member.email);
	if (account && !(await isAccountOpenTo(client, account, member))) {
		throw new AddressBelongsToAnotherCompany(member.email);
	}

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

async function isAccountOpenTo(
	client: SupabaseClient,
	account: User,
	member: { id: string; user_id: string | null },
): Promise<boolean> {
	if (account.app_metadata?.company_id) return false;
	if (member.user_id) return member.user_id === account.id;
	return (await memberHeldByAccount(client, account.id)) === null;
}

export async function accountOfAddress(client: SupabaseClient, address: string): Promise<User | null> {
	const { data: accountID, error } = await client.rpc('account_of_address', { address });
	if (error) throw new Error(`account of ${address}: ${error.message}`);
	if (typeof accountID !== 'string') return null;
	const { data, error: readError } = await client.auth.admin.getUserById(accountID);
	if (readError) throw new Error(`account of ${address}: ${readError.message}`);
	return data.user;
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
// So reissuing writes the new hash onto the row that stands; a plain insert
// answers a duplicate key to anyone who rotates a key twice.
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
	expiresAt: string;
	lastUsedAt: string | null;
};

const tokenSettingsSchema = z.object({
	expiresAt: z.iso.datetime({ offset: true }),
	lastUsedAt: z.iso.datetime({ offset: true }).optional(),
});

type TokenSettings = z.infer<typeof tokenSettingsSchema>;

function storedTokenSettings(stored: unknown): TokenSettings {
	const settings = tokenSettingsSchema.safeParse(stored);
	if (!settings.success) throw new Error('personal access token: its settings carry no expiry');
	return settings.data;
}

export async function issuePersonalAccessToken(
	client: SupabaseClient,
	memberID: string,
	name: string,
	permission: PublicAPIPermission = fullPublicAPIPermission,
	lifetimeDays: number = defaultTokenLifetimeDays,
	now: Date = new Date(),
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
				settings: { expiresAt: expiryAfter(lifetimeDays, now) },
			},
			{ onConflict: 'member_id,kind,name' },
		);
	if (error) throw new Error(`personal access token ${name}: ${error.message}`);
	return apiKey;
}

export async function personalAccessTokens(client: SupabaseClient, memberID: string): Promise<PersonalAccessToken[]> {
	const { data, error } = await client
		.from('credential')
		.select('name, permission, settings')
		.eq('member_id', memberID)
		.eq('kind', personalAccessTokenCredentialKind)
		.order('name');
	if (error) throw new Error(`personal access tokens: ${error.message}`);
	return (data ?? []).map((row) => {
		const settings = storedTokenSettings(row.settings);
		return {
			name: row.name as string,
			permission: storedTokenPermission(row.permission),
			expiresAt: settings.expiresAt,
			lastUsedAt: settings.lastUsedAt ?? null,
		};
	});
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

export class TokenHasExpired extends Error {
	constructor() {
		super('this token has expired; make another by the same name to renew it');
	}
}

type PersonalAccessTokenRow = {
	id: string;
	member_id: string;
	permission: unknown;
	name: string | null;
	settings: unknown;
	member: { status: string } | null;
};

// The caller of a personal access token is the member it belongs to, so this issues that
// member's own session and never the company's. The row that names the member
// names the rung too, so nobody downstream asks for it a second time.
export async function sessionForPersonalAccessToken(
	credentials: SigningCredentials,
	apiKey: string,
	now: Date = new Date(),
): Promise<PersonalAccessTokenSession | null> {
	const client = controlPlane(credentials);
	const { data, error } = await client
		.from('credential')
		.select('id, member_id, permission, name, settings, member(status)')
		.eq('kind', personalAccessTokenCredentialKind)
		.eq('external_id', await hashOf(apiKey))
		.maybeSingle<PersonalAccessTokenRow>();
	if (error) throw new Error(`personal access token: ${error.message}`);
	if (!data) return null;
	const settings = storedTokenSettings(data.settings);
	if (Date.parse(settings.expiresAt) <= now.getTime()) throw new TokenHasExpired();
	if (hasLeftTheCompany(data.member?.status ?? '')) throw new TokenOwnerHasLeft();
	await noteTokenUse(client, data.id, settings, now);
	const session = await sessionForMember(credentials, data.member_id);
	return {
		...session,
		permission: storedTokenPermission(data.permission),
		tokenName: typeof data.name === 'string' ? data.name : '',
	};
}

async function noteTokenUse(
	client: SupabaseClient,
	credentialID: string,
	settings: TokenSettings,
	now: Date,
): Promise<void> {
	if (!isLastSeenStale(settings.lastUsedAt ?? null, now.getTime())) return;
	const { error } = await client
		.from('credential')
		.update({ settings: { ...settings, lastUsedAt: now.toISOString() } })
		.eq('id', credentialID);
	if (error) throw new Error(`personal access token: ${error.message}`);
}

export async function keepConnectedAppPermission(
	client: SupabaseClient,
	memberID: string,
	clientID: string,
	permission: ConnectedAppPermission,
): Promise<void> {
	const { error } = await client
		.from('credential')
		.upsert(
			{ member_id: memberID, kind: connectedAppCredentialKind, name: clientID, permission },
			{ onConflict: 'member_id,kind,name' },
		);
	if (error) throw new Error(`connected app ${clientID}: ${error.message}`);
}

export async function forgetConnectedAppPermission(
	client: SupabaseClient,
	memberID: string,
	clientID: string,
): Promise<void> {
	const { error } = await client
		.from('credential')
		.delete()
		.eq('member_id', memberID)
		.eq('kind', connectedAppCredentialKind)
		.eq('name', clientID);
	if (error) throw new Error(`connected app ${clientID}: ${error.message}`);
}

export async function connectedAppPermissionFor(
	client: SupabaseClient,
	accountID: string,
	clientID: string,
): Promise<ConnectedAppPermission> {
	const { data, error } = await client
		.from('credential')
		.select('permission, member!inner(user_id)')
		.eq('kind', connectedAppCredentialKind)
		.eq('name', clientID)
		.eq('member.user_id', accountID)
		.maybeSingle<{ permission: unknown }>();
	if (error) throw new Error(`connected app ${clientID}: ${error.message}`);
	return connectedAppPermissionOf(data?.permission) ?? unchosenConnectedAppPermission;
}

export const fleetCredentialKind = 'fleet';

export async function claimFleetForCompany(
	client: SupabaseClient,
	companyID: string,
	fleetID: string,
	settings: Record<string, unknown> = {},
): Promise<void> {
	const { error } = await client
		.from('credential')
		.upsert(
			{ company_id: companyID, kind: fleetCredentialKind, external_id: fleetID, settings },
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

export async function spendAgentKey(client: SupabaseClient, apiKey: string, name: string): Promise<string | null> {
	const { data, error } = await client
		.from('agent')
		.update({ revoked_at: new Date().toISOString() })
		.eq('api_key_hash', await hashOf(apiKey))
		.eq('name', name)
		.is('revoked_at', null)
		.select('company_id')
		.maybeSingle<{ company_id: string }>();
	if (error) throw new Error(`agent key: ${error.message}`);
	return data?.company_id ?? null;
}

const lastSeenFreshForMilliseconds = 60_000;

export function isLastSeenStale(lastSeenAt: string | null, now: number = Date.now()): boolean {
	if (!lastSeenAt) return true;
	const seenAt = Date.parse(lastSeenAt);
	return Number.isNaN(seenAt) || now - seenAt >= lastSeenFreshForMilliseconds;
}

export async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

export async function sessionForPlatformIdentity(
	credentials: PlaneCredentials,
	apiKey: string,
	kind: string,
	externalID: string,
): Promise<MemberSession> {
	const client = controlPlane(credentials);
	const companyID = await companyOfHostCredential(credentials, apiKey);
	if (!companyID) throw new Error('that credential belongs to no company computer');

	const memberID =
		kind === 'email'
			? await memberOfCompanyByEmail(client, companyID, externalID)
			: (await membersOfCompanyByExternalID(client, companyID, kind)).get(externalID);
	if (!memberID) throw new Error(`no member here has ${kind} identity ${externalID}`);

	return sessionForMember(credentials, memberID);
}

export type HostSession = {
	companyID: string;
	accessToken: string;
	expiresAt: number;
};

const hostAddressDomain = 'agent.internkim.invalid';

export function hostAddressOf(companyID: string): string {
	return `host.${companyID}@${hostAddressDomain}`;
}

function isHostAddress(address: string): boolean {
	return address.trim().toLowerCase().endsWith(`@${hostAddressDomain}`);
}

export async function sessionForHost(
	credentials: PlaneCredentials,
	presented: string,
): Promise<HostSession> {
	if (presented.split('.').length === 3) return presentedHostSession(credentials, presented);
	const agent = await agentOfKey(controlPlane(credentials), presented);
	if (!agent) throw new Error('that key belongs to no agent');
	return hostSessionOfCompany(credentials, agent.companyID, { agentKeyHash: await hashOf(presented) });
}

async function presentedHostSession(credentials: PlaneCredentials, accessToken: string): Promise<HostSession> {
	const session = await verifiedHostSession(credentials, accessToken);
	if (!session) throw new Error('that session belongs to no company computer');
	return session;
}

type HostComputer = { fleetID: string } | { agentKeyHash: string };

function claimsNaming(computer: HostComputer): Record<string, string> {
	if ('fleetID' in computer) return { fleet_id: computer.fleetID };
	return { agent_key_hash: computer.agentKeyHash };
}

export async function hostSessionOfCompany(
	credentials: SigningCredentials,
	companyID: string,
	computer: HostComputer,
): Promise<HostSession> {
	const address = hostAddressOf(companyID);
	const userID = await keepHostAccount(controlPlane(credentials), address, companyID);
	const token = await recordTokenFor(credentials.signingKey, credentials.projectURL, {
		userID,
		email: address,
		appMetadata: { company_id: companyID, ...claimsNaming(computer) },
	});
	return { companyID, ...token };
}

const hostSessionClaimsSchema = z.object({
	email: z.string(),
	exp: z.number(),
	app_metadata: z.object({ company_id: z.string() }),
});

async function verifiedHostSession(
	credentials: PlaneCredentials,
	accessToken: string,
): Promise<HostSession | null> {
	const payload = await verifiedRecordToken(credentials.signingKey, credentials.projectURL, accessToken);
	const claims = hostSessionClaimsSchema.safeParse(payload);
	if (!claims.success) return null;
	const companyID = claims.data.app_metadata.company_id;
	if (claims.data.email !== hostAddressOf(companyID)) return null;
	if ((await companyTheRecordGrantsTo(credentials, accessToken)) !== companyID) return null;
	return { companyID, accessToken, expiresAt: claims.data.exp };
}

async function companyTheRecordGrantsTo(credentials: MemberCredentials, accessToken: string): Promise<string | null> {
	const { data, error, status } = await asMember(credentials, accessToken).rpc('my_app_company');
	if (error && status !== 401) throw new Error(`host session: ${error.message}`);
	return typeof data === 'string' ? data : null;
}

export async function companyOfHostSession(
	credentials: PlaneCredentials,
	accessToken: string,
): Promise<string | null> {
	return (await verifiedHostSession(credentials, accessToken))?.companyID ?? null;
}

export async function companyOfHostCredential(
	credentials: PlaneCredentials,
	presented: string,
): Promise<string | null> {
	if (presented.split('.').length === 3) return companyOfHostSession(credentials, presented);
	const agent = await agentOfKey(controlPlane(credentials), presented);
	return agent?.companyID ?? null;
}

async function keepHostAccount(
	client: SupabaseClient,
	address: string,
	companyID: string,
): Promise<string> {
	const account = await accountOfAddress(client, address);
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
