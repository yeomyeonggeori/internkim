export type CallerIdentity = {
	accountID: string;
	memberID: string;
	companyID: string;
	email: string;
};

export type TokenClaims = {
	sub: string;
	iss: string;
	exp: number;
	hostCompanyID?: string;
};

export const expectedAudience = 'authenticated';

export class TokenRefused extends Error {
	constructor(reason: string) {
		super(reason);
		this.name = 'TokenRefused';
	}
}

function decodeBase64URL(segment: string): Uint8Array<ArrayBuffer> {
	const padded = segment.replace(/-/g, '+').replace(/_/g, '/');
	const binary = atob(padded.padEnd(padded.length + ((4 - (padded.length % 4)) % 4), '='));
	return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

function decodeSegment(segment: string): unknown {
	try {
		return JSON.parse(new TextDecoder().decode(decodeBase64URL(segment)));
	} catch {
		throw new TokenRefused('the token is malformed');
	}
}

function isClaimRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

function isAudience(audience: unknown): boolean {
	if (Array.isArray(audience)) return audience.includes(expectedAudience);
	return audience === expectedAudience;
}

function claimsOf(payload: unknown, expectedIssuer: string, nowSeconds: number): TokenClaims {
	if (!isClaimRecord(payload)) throw new TokenRefused('the token carries no claims');
	const claims = payload;
	const { sub, iss, exp } = claims;
	if (typeof sub !== 'string' || sub.trim() === '') throw new TokenRefused('the token names no account');
	if (iss !== expectedIssuer) throw new TokenRefused(`the token was issued by ${String(iss)}`);
	if (!isAudience(claims.aud)) throw new TokenRefused('the token was not issued for signed-in accounts');
	if (typeof exp !== 'number') throw new TokenRefused('the token carries no expiry');
	if (exp <= nowSeconds) throw new TokenRefused('the token has expired');
	const appMetadata = claims.app_metadata;
	if (isClaimRecord(appMetadata)) {
		const companyID = appMetadata.company_id;
		if (typeof companyID === 'string' && companyID.trim() !== '') {
			return { sub, iss, exp, hostCompanyID: companyID };
		}
	}
	return { sub, iss, exp };
}

const algorithms: Record<string, { name: string; namedCurve?: string; hash: string }> = {
	ES256: { name: 'ECDSA', namedCurve: 'P-256', hash: 'SHA-256' },
	RS256: { name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' }
};

export type JSONWebKeySet = { keys: JsonWebKey[] };

// Narrower than the global fetch on purpose: the only dependency here is
// "something that reads a document", which a test can stand in for.
export type FetchDocument = (
	url: string,
	options?: { headers?: Record<string, string> }
) => Promise<{ ok: boolean; status: number; json: () => Promise<unknown> }>;

// The runtime's fetch refuses to be called with any other `this`, which is what
// holding it in a field or a default parameter does to it.
const fetchThroughTheRuntime: FetchDocument = (url, options) => fetch(url, options);

export class JSONWebKeyCache {
	private keys: Promise<JSONWebKeySet> | undefined;

	constructor(
		private readonly jwksURL: string,
		private readonly fetchDocument: FetchDocument = fetchThroughTheRuntime
	) {}

	async keySet(): Promise<JSONWebKeySet> {
		this.keys ??= this.readKeySet();
		return this.keys;
	}

	forget(): void {
		this.keys = undefined;
	}

	private async readKeySet(): Promise<JSONWebKeySet> {
		const response = await this.fetchDocument(this.jwksURL);
		if (!response.ok) {
			this.keys = undefined;
			throw new TokenRefused(`the issuer's key set answered ${response.status}`);
		}
		return (await response.json()) as JSONWebKeySet;
	}
}

export async function verifyToken(
	token: string,
	keyCache: JSONWebKeyCache,
	expectedIssuer: string,
	nowSeconds: number
): Promise<TokenClaims> {
	const [headerSegment, payloadSegment, signatureSegment] = token.split('.');
	if (!headerSegment || !payloadSegment || !signatureSegment) throw new TokenRefused('the token is not a jwt');

	const header = decodeSegment(headerSegment) as { alg?: string; kid?: string };
	const algorithm = header.alg ? algorithms[header.alg] : undefined;
	if (!algorithm) throw new TokenRefused(`the token is signed with ${header.alg ?? 'nothing'}`);

	const { keys } = await keyCache.keySet();
	const jsonWebKey = keys.find((candidate) => (candidate as { kid?: string }).kid === header.kid);
	if (!jsonWebKey) {
		keyCache.forget();
		throw new TokenRefused('the issuer published no key with that id');
	}

	const key = await crypto.subtle.importKey('jwk', jsonWebKey, algorithm, false, ['verify']);
	const isSigned = await crypto.subtle.verify(
		algorithm.name === 'ECDSA' ? { name: 'ECDSA', hash: algorithm.hash } : algorithm.name,
		key,
		decodeBase64URL(signatureSegment),
		new TextEncoder().encode(`${headerSegment}.${payloadSegment}`)
	);
	if (!isSigned) throw new TokenRefused('the token signature does not verify');

	return claimsOf(decodeSegment(payloadSegment), expectedIssuer, nowSeconds);
}

type MemberRow = { id: string; company_id: string; email: string | null };

// The member row is read with the caller's own token, so row level security is
// what decides the answer; the gateway holds no key that could read another
// company's rows.
export async function resolveMember(
	supabaseURL: string,
	apiKey: string,
	token: string,
	accountID: string,
	fetchDocument: FetchDocument = fetchThroughTheRuntime
): Promise<CallerIdentity> {
	const url = `${supabaseURL.replace(/\/+$/, '')}/rest/v1/member?user_id=eq.${encodeURIComponent(accountID)}&select=id,company_id,email`;
	const response = await fetchDocument(url, {
		headers: { apikey: apiKey, Authorization: `Bearer ${token}` }
	});
	if (!response.ok) throw new TokenRefused(`the record answered ${response.status} for this account`);
	const rows = (await response.json()) as MemberRow[];
	if (!Array.isArray(rows) || rows.length !== 1) throw new TokenRefused('this account belongs to no company');
	return {
		accountID,
		memberID: rows[0].id,
		companyID: rows[0].company_id,
		email: (rows[0].email ?? '').trim().toLowerCase()
	};
}

export async function companyOfHost(
	supabaseURL: string,
	apiKey: string,
	token: string,
	fetchDocument: FetchDocument = fetchThroughTheRuntime
): Promise<string | null> {
	const url = `${supabaseURL.replace(/\/+$/, '')}/rest/v1/rpc/my_app_company`;
	const response = await fetchDocument(url, {
		headers: { apikey: apiKey, Authorization: `Bearer ${token}` }
	});
	if (!response.ok) throw new TokenRefused(`the record answered ${response.status} for this computer`);
	const company = await response.json();
	return typeof company === 'string' ? company : null;
}
