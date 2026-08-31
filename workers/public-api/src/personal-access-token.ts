export type CallerPermission = 'read' | 'write' | 'delete';

export type Caller = {
	email: string;
	companyID: string;
	memberID: string;
	tokenName: string;
	permission: CallerPermission;
};

export type ControlPlaneCredentials = {
	projectURL: string;
	serviceRoleKey: string;
};

export type FetchDocument = (
	url: string,
	options?: { method?: string; headers?: Record<string, string>; body?: string }
) => Promise<{ ok: boolean; status: number; json: () => Promise<unknown> }>;

export class RecordRefused extends Error {
	constructor(reason: string) {
		super(reason);
		this.name = 'RecordRefused';
	}
}

const fetchThroughTheRuntime: FetchDocument = (url, options) => fetch(url, options);

const personalAccessTokenPrefix = 'ik_';
const personalAccessTokenKind = 'api_key';
const permissionColumn = 'permission';

export function isPersonalAccessToken(presented: string): boolean {
	return presented.startsWith(personalAccessTokenPrefix);
}

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

type CredentialRow = {
	name?: unknown;
	permission?: unknown;
	member?: { id?: unknown; email?: unknown; company_id?: unknown } | null;
};

export function permissionNamed(offered: unknown): CallerPermission {
	if (offered === 'delete' || offered === 'write') return offered;
	return 'read';
}

function callerOfRow(row: CredentialRow): Caller | null {
	const member = row.member;
	if (typeof member !== 'object' || member === null) return null;
	const { id: memberID, email, company_id: companyID } = member;
	if (typeof email !== 'string' || email.trim() === '') return null;
	if (typeof companyID !== 'string' || companyID.trim() === '') return null;
	if (typeof memberID !== 'string' || memberID.trim() === '') return null;
	return {
		email: email.trim().toLowerCase(),
		companyID,
		memberID,
		tokenName: typeof row.name === 'string' ? row.name : '',
		permission: permissionNamed(row.permission)
	};
}

export async function callerOfPersonalAccessToken(
	credentials: ControlPlaneCredentials,
	presented: string,
	fetchDocument: FetchDocument = fetchThroughTheRuntime
): Promise<Caller | null> {
	if (!isPersonalAccessToken(presented)) return null;
	const query = new URLSearchParams({
		kind: `eq.${personalAccessTokenKind}`,
		external_id: `eq.${await hashOf(presented)}`,
		select: `name,${permissionColumn},member(id,email,company_id)`
	});
	const response = await fetchDocument(
		`${credentials.projectURL.replace(/\/+$/, '')}/rest/v1/credential?${query.toString()}`,
		{
			headers: {
				apikey: credentials.serviceRoleKey,
				Authorization: `Bearer ${credentials.serviceRoleKey}`
			}
		}
	);
	if (!response.ok) throw new RecordRefused(`the record answered ${response.status} for this key`);
	const rows = (await response.json()) as CredentialRow[];
	if (!Array.isArray(rows) || rows.length !== 1) return null;
	return callerOfRow(rows[0]);
}

export type TokenSummary = { name: string; permission: CallerPermission };

const permissionRungs: CallerPermission[] = ['read', 'write', 'delete'];

export function reachesRung(held: CallerPermission, asked: CallerPermission): boolean {
	return permissionRungs.indexOf(asked) <= permissionRungs.indexOf(held);
}

function recordHeaders(credentials: ControlPlaneCredentials, extra: Record<string, string> = {}) {
	return {
		apikey: credentials.serviceRoleKey,
		Authorization: `Bearer ${credentials.serviceRoleKey}`,
		...extra
	};
}

function recordURL(credentials: ControlPlaneCredentials, query: URLSearchParams): string {
	return `${credentials.projectURL.replace(/\/+$/, '')}/rest/v1/credential?${query.toString()}`;
}

function mineQuery(memberID: string, extra: Record<string, string> = {}): URLSearchParams {
	return new URLSearchParams({
		member_id: `eq.${memberID}`,
		kind: `eq.${personalAccessTokenKind}`,
		...extra
	});
}

function mintedToken(): string {
	const bytes = crypto.getRandomValues(new Uint8Array(32));
	return personalAccessTokenPrefix + [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

export async function tokensOfMember(
	credentials: ControlPlaneCredentials,
	memberID: string,
	fetchDocument: FetchDocument = fetchThroughTheRuntime
): Promise<TokenSummary[]> {
	const query = mineQuery(memberID, { select: `name,${permissionColumn}`, order: 'name' });
	const response = await fetchDocument(recordURL(credentials, query), { headers: recordHeaders(credentials) });
	if (!response.ok) throw new RecordRefused(`the record answered ${response.status} for this member's tokens`);
	const rows = (await response.json()) as { name?: unknown; permission?: unknown }[];
	if (!Array.isArray(rows)) return [];
	return rows
		.filter((row) => typeof row.name === 'string')
		.map((row) => ({ name: row.name as string, permission: permissionNamed(row.permission) }));
}

export async function issueToken(
	credentials: ControlPlaneCredentials,
	memberID: string,
	name: string,
	permission: CallerPermission,
	fetchDocument: FetchDocument = fetchThroughTheRuntime
): Promise<string> {
	const token = mintedToken();
	const query = new URLSearchParams({ on_conflict: 'member_id,kind,name' });
	const response = await fetchDocument(recordURL(credentials, query), {
		method: 'POST',
		headers: recordHeaders(credentials, {
			'Content-Type': 'application/json',
			Prefer: 'resolution=merge-duplicates'
		}),
		body: JSON.stringify({
			member_id: memberID,
			kind: personalAccessTokenKind,
			name,
			external_id: await hashOf(token),
			permission
		})
	});
	if (!response.ok) throw new RecordRefused(`the record answered ${response.status} making the token ${name}`);
	return token;
}

export async function revokeToken(
	credentials: ControlPlaneCredentials,
	memberID: string,
	name: string,
	fetchDocument: FetchDocument = fetchThroughTheRuntime
): Promise<boolean> {
	const query = mineQuery(memberID, { name: `eq.${name}`, select: 'name' });
	const response = await fetchDocument(recordURL(credentials, query), {
		method: 'DELETE',
		headers: recordHeaders(credentials, { Prefer: 'return=representation' })
	});
	if (!response.ok) throw new RecordRefused(`the record answered ${response.status} revoking the token ${name}`);
	const rows = (await response.json()) as unknown;
	return Array.isArray(rows) && rows.length > 0;
}

export const keyCacheSeconds = 60;

const keysKeptInMemory = 4096;

export class PersonalTokenCache {
	private readonly callers = new Map<string, { caller: Caller; staleAt: number }>();

	constructor(
		private readonly readCaller: (presented: string) => Promise<Caller | null>,
		private readonly livesForMilliseconds = keyCacheSeconds * 1000
	) {}

	async callerOf(presented: string, nowMilliseconds: number): Promise<Caller | null> {
		const remembered = this.callers.get(presented);
		if (remembered && remembered.staleAt > nowMilliseconds) return remembered.caller;
		const caller = await this.readCaller(presented);
		if (!caller) {
			this.callers.delete(presented);
			return null;
		}
		this.forgetStale(nowMilliseconds);
		this.callers.set(presented, { caller, staleAt: nowMilliseconds + this.livesForMilliseconds });
		return caller;
	}

	private forgetStale(nowMilliseconds: number): void {
		if (this.callers.size < keysKeptInMemory) return;
		for (const [presented, remembered] of this.callers) {
			if (remembered.staleAt <= nowMilliseconds) this.callers.delete(presented);
		}
	}
}
