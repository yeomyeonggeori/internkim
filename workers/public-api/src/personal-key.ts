export type CallerPermission = 'read' | 'write' | 'delete';

export type Caller = {
	email: string;
	companyID: string;
	permission: CallerPermission;
};

export type ControlPlaneCredentials = {
	projectURL: string;
	serviceRoleKey: string;
};

export type FetchDocument = (
	url: string,
	options?: { headers?: Record<string, string> }
) => Promise<{ ok: boolean; status: number; json: () => Promise<unknown> }>;

export class RecordRefused extends Error {
	constructor(reason: string) {
		super(reason);
		this.name = 'RecordRefused';
	}
}

const fetchThroughTheRuntime: FetchDocument = (url, options) => fetch(url, options);

const personalKeyPrefix = 'ik_';
const personalKeyKind = 'api_key';
const permissionColumn = 'permission';

export function isPersonalKey(presented: string): boolean {
	return presented.startsWith(personalKeyPrefix);
}

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

type CredentialRow = {
	permission?: unknown;
	member?: { email?: unknown; company_id?: unknown } | null;
};

export function permissionNamed(offered: unknown): CallerPermission {
	if (offered === 'delete' || offered === 'write') return offered;
	return 'read';
}

function callerOfRow(row: CredentialRow): Caller | null {
	const member = row.member;
	if (typeof member !== 'object' || member === null) return null;
	const { email, company_id: companyID } = member;
	if (typeof email !== 'string' || email.trim() === '') return null;
	if (typeof companyID !== 'string' || companyID.trim() === '') return null;
	return {
		email: email.trim().toLowerCase(),
		companyID,
		permission: permissionNamed(row.permission)
	};
}

export async function callerOfPersonalKey(
	credentials: ControlPlaneCredentials,
	presented: string,
	fetchDocument: FetchDocument = fetchThroughTheRuntime
): Promise<Caller | null> {
	if (!isPersonalKey(presented)) return null;
	const query = new URLSearchParams({
		kind: `eq.${personalKeyKind}`,
		external_id: `eq.${await hashOf(presented)}`,
		select: `${permissionColumn},member(email,company_id)`
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

export const keyCacheSeconds = 60;

const keysKeptInMemory = 4096;

export class PersonalKeyCache {
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
