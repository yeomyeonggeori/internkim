import { describe, expect, test } from 'bun:test';
import { exportJWK, generateKeyPair, jwtVerify } from 'jose';
import { recordTokenFor, recordTokenLifetimeSeconds } from '../../../src/lib/server/record-token';

const projectURL = 'https://example.supabase.co';
const now = new Date('2026-09-17T09:00:00Z');
const nowInSeconds = Math.floor(now.getTime() / 1000);

describe('a record token', () => {
	test('names the account and the role, comes from the plane as issuer, and is signed with the shared secret', async () => {
		const secret = new TextEncoder().encode('a-local-secret-of-at-least-thirty-two-characters');
		const signingKey = JSON.stringify({ kty: 'oct', k: Buffer.from(secret).toString('base64url') });

		const token = await recordTokenFor(signingKey, projectURL, { userID: 'user-1', email: 'a@b.c' }, now);
		const { payload, protectedHeader } = await jwtVerify(token.accessToken, secret, {
			issuer: `${projectURL}/auth/v1`,
			audience: 'authenticated',
			currentDate: now
		});

		expect(protectedHeader.alg).toBe('HS256');
		expect(payload.sub).toBe('user-1');
		expect(payload.role).toBe('authenticated');
		expect(payload.email).toBe('a@b.c');
		expect(payload.exp).toBe(nowInSeconds + recordTokenLifetimeSeconds);
		expect(token.expiresAt).toBe(nowInSeconds + recordTokenLifetimeSeconds);
	});

	test('is verified by the public half of an elliptic curve key and carries the kid that finds it', async () => {
		const { privateKey, publicKey } = await generateKeyPair('ES256', { extractable: true });
		const signingKey = JSON.stringify({ ...(await exportJWK(privateKey)), kid: 'key-1' });

		const token = await recordTokenFor(
			signingKey,
			projectURL,
			{ userID: 'user-2', email: null, appMetadata: { company_id: 'company-1' } },
			now
		);
		const { payload, protectedHeader } = await jwtVerify(token.accessToken, publicKey, { currentDate: now });

		expect(protectedHeader).toMatchObject({ alg: 'ES256', kid: 'key-1' });
		expect(payload.app_metadata).toEqual({ company_id: 'company-1' });
		expect(payload.email).toBeUndefined();
	});

	test('signs with a key registered the way Supabase asks, operations and all', async () => {
		const { privateKey, publicKey } = await generateKeyPair('ES256', { extractable: true });
		const signingKey = JSON.stringify({
			...(await exportJWK(privateKey)),
			kid: 'key-2',
			use: 'sig',
			alg: 'ES256',
			key_ops: ['sign', 'verify'],
			ext: true
		});

		const token = await recordTokenFor(signingKey, projectURL, { userID: 'user-3', email: null }, now);
		const { payload } = await jwtVerify(token.accessToken, publicKey, { currentDate: now });

		expect(payload.sub).toBe('user-3');
	});

	test('refuses a key that is not a JWK', async () => {
		await expect(recordTokenFor('"just a string"', projectURL, { userID: 'user', email: null })).rejects.toThrow(
			'not a JWK'
		);
	});
});
