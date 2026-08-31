import { describe, expect, test } from 'bun:test';
import { JSONWebKeyCache, TokenRefused } from '../../connection-gateway/src/identity';
import { callerOfSessionToken, issuerOf, jwksURLOf } from './session-caller';

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

const environment = { SUPABASE_URL: 'https://plane.supabase.co/', SUPABASE_SECRET_KEY: 'service-role' };
const claims = { sub: 'account-1', iss: 'https://plane.supabase.co/auth/v1', exp: 4102444800 };
const now = 1786000000;

function keyCacheServing(document: string): JSONWebKeyCache {
	return new JSONWebKeyCache(jwksURLOf(environment.SUPABASE_URL), () =>
		Promise.resolve(new Response(document, { status: 200 }))
	);
}

describe('the issuer addresses', () => {
	test('are built from the project url with trailing slashes dropped', () => {
		expect(issuerOf('https://plane.supabase.co/')).toBe('https://plane.supabase.co/auth/v1');
		expect(jwksURLOf('https://plane.supabase.co')).toBe(
			'https://plane.supabase.co/auth/v1/.well-known/jwks.json'
		);
	});
});

describe('callerOfSessionToken', () => {
	test('turns a signed session into the member with full permission', async () => {
		const { token, jwks } = await signedToken(claims);
		const caller = await callerOfSessionToken(environment, keyCacheServing(jwks), token, now, () =>
			Promise.resolve(Response.json([{ id: 'm1', company_id: 'c1', email: 'Someone@Example.com' }]))
		);
		expect(caller).toEqual({
			email: 'someone@example.com',
			companyID: 'c1',
			memberID: 'm1',
			tokenName: '',
			permission: 'delete'
		});
	});

	test('offers the member record the session token, never the secret, as the bearer', async () => {
		const { token, jwks } = await signedToken(claims);
		let seenAuthorization = '';
		await callerOfSessionToken(environment, keyCacheServing(jwks), token, now, (_url, options) => {
			seenAuthorization = options?.headers?.Authorization ?? '';
			return Promise.resolve(Response.json([{ id: 'm1', company_id: 'c1', email: 'someone@example.com' }]));
		});
		expect(seenAuthorization).toBe(`Bearer ${token}`);
	});

	test('refuses a member row that carries no email', async () => {
		const { token, jwks } = await signedToken(claims);
		expect(
			callerOfSessionToken(environment, keyCacheServing(jwks), token, now, () =>
				Promise.resolve(Response.json([{ id: 'm1', company_id: 'c1', email: null }]))
			)
		).rejects.toThrow('no email');
	});

	test('refuses a token that does not verify before touching the record', async () => {
		let recordAsked = false;
		expect(
			callerOfSessionToken(environment, keyCacheServing('{"keys":[]}'), 'not-a-jwt', now, () => {
				recordAsked = true;
				return Promise.resolve(Response.json([]));
			})
		).rejects.toThrow(TokenRefused);
		expect(recordAsked).toBe(false);
	});
});
