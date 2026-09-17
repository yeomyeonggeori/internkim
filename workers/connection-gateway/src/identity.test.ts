import { describe, expect, test } from 'bun:test';
import { JSONWebKeyCache, TokenRefused, resolveMember, verifyToken } from './identity';

const keyID = 'test-key';

function base64URL(bytes: Uint8Array): string {
	return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function encodeSegment(document: unknown): string {
	return base64URL(new TextEncoder().encode(JSON.stringify(document)));
}

async function signedToken(claims: Record<string, unknown>): Promise<{ token: string; jwks: string }> {
	const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, ['sign', 'verify']);
	const publicKey = await crypto.subtle.exportKey('jwk', pair.publicKey);
	const signed = `${encodeSegment({ alg: 'ES256', kid: keyID })}.${encodeSegment(claims)}`;
	const signature = await crypto.subtle.sign(
		{ name: 'ECDSA', hash: 'SHA-256' },
		pair.privateKey,
		new TextEncoder().encode(signed)
	);
	return {
		token: `${signed}.${base64URL(new Uint8Array(signature))}`,
		jwks: JSON.stringify({ keys: [{ ...publicKey, kid: keyID }] })
	};
}

function keyCacheServing(document: string, status = 200): JSONWebKeyCache {
	return new JSONWebKeyCache('https://issuer.test/jwks', () =>
		Promise.resolve(new Response(document, { status }))
	);
}

const issuer = 'https://issuer.test/auth/v1';
const validClaims = { sub: 'account-1', iss: issuer, exp: 4102444800 };

describe('verifyToken', () => {
	test('accepts a token the issuer signed', async () => {
		const { token, jwks } = await signedToken(validClaims);
		const claims = await verifyToken(token, keyCacheServing(jwks), issuer, 1786000000);
		expect(claims.sub).toBe('account-1');
	});

	test('exposes a host company only from signed app metadata', async () => {
		const { token, jwks } = await signedToken({
			...validClaims,
			app_metadata: { company_id: 'company-1' },
			user_metadata: { company_id: 'company-attacker' }
		});
		const claims = await verifyToken(token, keyCacheServing(jwks), issuer, 1786000000);
		expect(claims.hostCompanyID).toBe('company-1');
	});

	test('keeps ordinary tokens free of a host identity', async () => {
		const { token, jwks } = await signedToken({ ...validClaims, user_metadata: { company_id: 'company-1' } });
		const claims = await verifyToken(token, keyCacheServing(jwks), issuer, 1786000000);
		expect(claims).toEqual(validClaims);
	});

	test('refuses a token whose payload was edited after signing', async () => {
		const { token, jwks } = await signedToken(validClaims);
		const [header, , signature] = token.split('.');
		const forged = `${header}.${encodeSegment({ ...validClaims, sub: 'account-2' })}.${signature}`;
		expect(verifyToken(forged, keyCacheServing(jwks), issuer, 1786000000)).rejects.toThrow(TokenRefused);
	});

	test('refuses an expired token', async () => {
		const { token, jwks } = await signedToken({ ...validClaims, exp: 1000 });
		expect(verifyToken(token, keyCacheServing(jwks), issuer, 1786000000)).rejects.toThrow('expired');
	});

	test('refuses an unsigned token claiming none as its algorithm', async () => {
		const unsigned = `${encodeSegment({ alg: 'none' })}.${encodeSegment(validClaims)}.`;
		expect(verifyToken(unsigned, keyCacheServing('{"keys":[]}'), issuer, 1786000000)).rejects.toThrow(TokenRefused);
	});

	test('refuses when the issuer publishes no matching key', async () => {
		const { token } = await signedToken(validClaims);
		expect(verifyToken(token, keyCacheServing('{"keys":[]}'), issuer, 1786000000)).rejects.toThrow('no key with that id');
	});

	test('refuses a token another project signed', async () => {
		const { token, jwks } = await signedToken({ ...validClaims, iss: 'https://elsewhere.test/auth/v1' });
		expect(verifyToken(token, keyCacheServing(jwks), issuer, 1786000000)).rejects.toThrow('issued by');
	});
});

describe('resolveMember', () => {
	test('reads the member row with the token the caller offered', async () => {
		let seenAuthorization = '';
		const identity = await resolveMember(
			'https://record.test/',
			'publishable',
			'the-token',
			'account-1',
			(_url, options) => {
				seenAuthorization = options?.headers?.Authorization ?? '';
				return Promise.resolve(Response.json([{ id: 'member-1', company_id: 'company-1', email: 'Sample@Example.com ' }]));
			}
		);
		expect(identity).toEqual({
			accountID: 'account-1',
			memberID: 'member-1',
			companyID: 'company-1',
			email: 'sample@example.com'
		});
		expect(seenAuthorization).toBe('Bearer the-token');
	});

	test('refuses an account row level security hides', async () => {
		expect(
			resolveMember('https://record.test', 'publishable', 'the-token', 'account-1', () =>
				Promise.resolve(Response.json([]))
			)
		).rejects.toThrow('belongs to no company');
	});
});
