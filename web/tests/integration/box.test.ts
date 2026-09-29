import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { SignJWT, decodeJwt, exportJWK, generateKeyPair } from 'jose';
import { z } from 'zod';
import { x25519 } from '@noble/curves/ed25519.js';
import { base64URLOf } from '../../src/lib/company/seal-model-key';
import { sealedModelKeySchema } from '../../src/lib/company/box';
import { companyComputerName } from '../../src/lib/company/host-setup';
import { callingAgent } from '../../src/lib/server/agent-request';
import {
	announceBox,
	boxAssertionAudience,
	boxKeyOfAssertion,
	BoxRefused,
	boxSessionFor,
	claimBox,
	claimBoxWithConnectionFile,
	connectedBoxOf,
	emptyBoxesAt,
	keepSealedModelKey
} from '../../src/lib/server/box';
import {
	companyOfHostSession,
	controlPlane,
	issueAgentKey,
	provisionCompany,
	sessionForHost,
	sessionForMember,
	type RefreshableHostSession
} from '../../src/lib/server/control-plane';
import { recordTokenFor } from '../../src/lib/server/record-token';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

const networkHookTimeout = 60_000;
const credentials = { projectURL, serviceRoleKey, signingKey };
const client = controlPlane(credentials);
const stamp = Date.now();
const officeAddress = `198.51.100.${stamp % 200}`;
const neighbourAddress = `203.0.113.${stamp % 200}`;
const environment = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey,
	GATEWAY_URL: 'https://gateway.example.test'
};
const appURL = 'https://intern.example.test';
const companyIDs: string[] = [];
let companyID = '';
let adminMemberID = '';

type Box = { publicKey: string; encryptionKey: string; sign: (claims?: Partial<AssertionClaims>) => Promise<string> };
type AssertionClaims = { issuer: string; audience: string; issuedAt: number; expiresAt: number };

async function aBox(): Promise<Box> {
	const { privateKey, publicKey } = await generateKeyPair('EdDSA', { crv: 'Ed25519' });
	const { x } = await exportJWK(publicKey);
	if (!x) throw new Error('an Ed25519 key exports its public half as x');
	const encryptionKey = base64URLOf(x25519.getPublicKey(x25519.utils.randomSecretKey()));
	const sign = (claims: Partial<AssertionClaims> = {}) => {
		const now = Math.floor(Date.now() / 1000);
		return new SignJWT({})
			.setProtectedHeader({ alg: 'EdDSA', typ: 'JWT' })
			.setIssuer(claims.issuer ?? x)
			.setAudience(claims.audience ?? boxAssertionAudience)
			.setIssuedAt(claims.issuedAt ?? now)
			.setExpirationTime(claims.expiresAt ?? now + 60)
			.sign(privateKey);
	};
	return { publicKey: x, encryptionKey, sign };
}

async function aCompany(label: string) {
	const provisioned = await provisionCompany(
		client,
		{ name: `Box ${label}`, slug: `box-${label}-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`box-${label}-${stamp}-admin@example.test`
	);
	companyIDs.push(provisioned.companyID);
	return provisioned;
}

async function refreshableSessionOf(box: Box): Promise<RefreshableHostSession> {
	const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);
	const session = answered?.session;
	if (!session || !('refreshToken' in session)) throw new Error('a claimed box that asks for a refreshable session gets one');
	return session;
}

const refreshedSchema = z.object({ access_token: z.string(), refresh_token: z.string() });

async function refreshWith(refreshToken: string): Promise<Response> {
	return fetch(`${projectURL}/auth/v1/token?grant_type=refresh_token`, {
		method: 'POST',
		headers: { apikey: publishableKey, 'Content-Type': 'application/json' },
		body: JSON.stringify({ refresh_token: refreshToken })
	});
}

async function refreshedThroughTheProject(refreshToken: string) {
	const answered = await refreshWith(refreshToken);
	expect(answered.status).toBe(200);
	return refreshedSchema.parse(await answered.json());
}

function requestBearing(token: string): Request {
	return new Request('https://intern.example.test/api/agent/member', { headers: { authorization: `Bearer ${token}` } });
}

beforeAll(async () => {
	const provisioned = await aCompany('ours');
	companyID = provisioned.companyID;
	adminMemberID = provisioned.adminMemberID;
	const { data } = await client.from('member').select('email').eq('id', adminMemberID).single();
	const account = await client.auth.admin.createUser({ email: data?.email ?? '', email_confirm: true });
	if (account.error) throw new Error(account.error.message);
	await client.from('member').update({ status: 'active', user_id: account.data.user.id }).eq('id', adminMemberID);
}, networkHookTimeout);

afterAll(async () => {
	await client.from('empty_box').delete().in('public_address', [officeAddress, neighbourAddress]);
	for (const heldCompanyID of companyIDs) {
		const { data: members } = await client.from('member').select('user_id').eq('company_id', heldCompanyID);
		await client.from('company').delete().eq('id', heldCompanyID);
		for (const member of members ?? []) {
			if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
		}
	}
}, networkHookTimeout);

describe('a box proves itself with the key it names', () => {
	test('an assertion signed by the named key answers that key', async () => {
		const box = await aBox();
		expect(await boxKeyOfAssertion(await box.sign())).toBe(box.publicKey);
	});

	test('an assertion naming someone else’s key is refused', async () => {
		const box = await aBox();
		const other = await aBox();
		expect(await boxKeyOfAssertion(await box.sign({ issuer: other.publicKey }))).toBeNull();
	});

	test('an old assertion is refused even before it expires', async () => {
		const box = await aBox();
		const now = Math.floor(Date.now() / 1000);
		expect(await boxKeyOfAssertion(await box.sign({ issuedAt: now - 300, expiresAt: now + 300 }))).toBeNull();
	});

	test('an assertion meant for something else is refused', async () => {
		const box = await aBox();
		expect(await boxKeyOfAssertion(await box.sign({ audience: 'something-else' }))).toBeNull();
	});

	test('something that is not an assertion is refused', async () => {
		expect(await boxKeyOfAssertion('not-a-token')).toBeNull();
		expect(await boxKeyOfAssertion('')).toBeNull();
	});
});

describe('connecting an empty box', () => {
	test('an announced box is listed only at the address it announced from', async () => {
		const box = await aBox();
		expect(await announceBox(client, box.publicKey, box.encryptionKey, officeAddress)).toEqual({ isClaimed: false });

		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).toContain(box.publicKey);
		expect((await emptyBoxesAt(client, neighbourAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
	});

	test('a box that stopped announcing drops out of the list', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		const later = new Date(Date.now() + 11 * 60 * 1000);

		expect((await emptyBoxesAt(client, officeAddress, later)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
	});

	test('a box cannot be claimed from another network', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);

		await expect(claimBox(client, companyID, box.publicKey, neighbourAddress)).rejects.toBeInstanceOf(BoxRefused);
		expect(await connectedBoxOf(client, companyID)).toBeNull();
	});

	test('claiming binds the box to the company and takes it off the list', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);

		await claimBox(client, companyID, box.publicKey, officeAddress);

		expect(await connectedBoxOf(client, companyID)).toMatchObject({
			publicKey: box.publicKey,
			encryptionKey: box.encryptionKey,
			hasModelKey: false
		});
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
		expect(await announceBox(client, box.publicKey, box.encryptionKey, officeAddress)).toEqual({ isClaimed: true });
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
	});

	test('a claimed box gets its company and a host session, and the model key is not in the answer', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		await claimBox(client, companyID, box.publicKey, officeAddress);
		const sealedModelKey = { ephemeralPublicKey: box.encryptionKey, nonce: 'MzMzMzMzMzMzMzMz', ciphertext: 'c2VhbGVk' };
		await keepSealedModelKey(client, companyID, sealedModelKey);

		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);

		expect(answered?.configuration.company.id).toBe(companyID);
		expect(answered?.configuration.appURL).toBe(appURL);
		expect(Object.keys(answered ?? {}).sort()).toEqual(['configuration', 'session']);
		expect(await companyOfHostSession(credentials, answered?.session.accessToken ?? '')).toBe(companyID);
		expect((await connectedBoxOf(client, companyID))?.lastSeenAt).not.toBeNull();
	});

	test('an unclaimed box gets no session', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);

		expect(await boxSessionFor(credentials, box.publicKey, environment, appURL)).toBeNull();
	});

	test('connecting a second box moves the company to it', async () => {
		const first = await aBox();
		const second = await aBox();
		for (const box of [first, second]) await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);

		await claimBox(client, companyID, first.publicKey, officeAddress);
		await claimBox(client, companyID, second.publicKey, officeAddress);

		expect((await connectedBoxOf(client, companyID))?.publicKey).toBe(second.publicKey);
		expect(await boxSessionFor(credentials, first.publicKey, environment, appURL)).toBeNull();
	});

	test('a model key waits for a connected box', async () => {
		const empty = await aCompany('empty');
		const sealedModelKey = { ephemeralPublicKey: 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM', nonce: 'MzMzMzMzMzMzMzMz', ciphertext: 'c2VhbGVk' };

		await expect(keepSealedModelKey(client, empty.companyID, sealedModelKey)).rejects.toBeInstanceOf(BoxRefused);
	});
});

describe('a connection file claims the computer it is installed on', () => {
	test('the file names the company once, and the computer then gets its session', async () => {
		const box = await aBox();
		const issued = await issueAgentKey(client, companyID, companyComputerName, { replaceStanding: true });

		expect(await claimBoxWithConnectionFile(client, box.publicKey, box.encryptionKey, issued.apiKey)).toBe(companyID);
		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);

		expect(answered?.configuration.company.id).toBe(companyID);
	});

	test('a file already used claims nothing more', async () => {
		const first = await aBox();
		const second = await aBox();
		const issued = await issueAgentKey(client, companyID, companyComputerName, { replaceStanding: true });
		await claimBoxWithConnectionFile(client, first.publicKey, first.encryptionKey, issued.apiKey);

		expect(await claimBoxWithConnectionFile(client, second.publicKey, second.encryptionKey, issued.apiKey)).toBeNull();
		expect(await boxSessionFor(credentials, second.publicKey, environment, appURL)).toBeNull();
	});

	test('only a connection file key claims, not another key the company holds', async () => {
		const box = await aBox();
		const other = await issueAgentKey(client, companyID, `another-caller-${stamp}`);

		expect(await claimBoxWithConnectionFile(client, box.publicKey, box.encryptionKey, other.apiKey)).toBeNull();
	});
});

describe('the routes a company computer calls accept its session', () => {
	test('a host session names the company it was issued for', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		await claimBox(client, companyID, box.publicKey, officeAddress);
		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);

		const agent = await callingAgent(requestBearing(answered?.session.accessToken ?? ''), environment);

		expect(agent.companyID).toBe(companyID);
	});

	test('a token naming the company for anyone but its host account is not a company computer', async () => {
		const token = await recordTokenFor(signingKey, projectURL, {
			userID: crypto.randomUUID(),
			email: `box-ours-${stamp}-admin@example.test`,
			appMetadata: { company_id: companyID }
		});

		expect(await companyOfHostSession(credentials, token.accessToken)).toBeNull();
	});

	test('a member’s own session is not a company computer', async () => {
		const memberSession = await sessionForMember(credentials, adminMemberID);

		await expect(callingAgent(requestBearing(memberSession.accessToken), environment)).rejects.toMatchObject({ status: 403 });
	});

	test('asking for a host session with one hands the same one back, unrenewed', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		await claimBox(client, companyID, box.publicKey, officeAddress);
		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);
		const presented = answered?.session;
		if (!presented) throw new Error('a claimed box gets a session');

		expect(await sessionForHost(credentials, presented.accessToken)).toEqual({
			companyID: presented.companyID,
			accessToken: presented.accessToken,
			expiresAt: presented.expiresAt
		});
	});

	test('the first session comes with a refresh token the box renews with by itself', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		await claimBox(client, companyID, box.publicKey, officeAddress);
		const first = await refreshableSessionOf(box);
		expect(first.refreshToken).not.toBe('');

		const renewed = await refreshedThroughTheProject(first.refreshToken);

		expect(renewed.refresh_token).not.toBe(first.refreshToken);
		expect(await companyOfHostSession(credentials, renewed.access_token)).toBe(companyID);
		const agent = await callingAgent(requestBearing(renewed.access_token), environment);
		expect(agent.companyID).toBe(companyID);
	});

	test('a second refreshable bootstrap ends the first one’s refresh token', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		await claimBox(client, companyID, box.publicKey, officeAddress);
		const first = await refreshableSessionOf(box);

		const second = await refreshableSessionOf(box);

		expect((await refreshWith(first.refreshToken)).status).toBe(400);
		expect((await refreshedThroughTheProject(second.refreshToken)).refresh_token).not.toBe('');
	});

	test('a host session an agent key bought has no session to end, and a box bootstrap leaves it valid', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		await claimBox(client, companyID, box.publicKey, officeAddress);
		const issued = await issueAgentKey(client, companyID, `host-session-buyer-${stamp}`);
		const bought = await sessionForHost(credentials, issued.apiKey);
		await refreshableSessionOf(box);

		expect(await companyOfHostSession(credentials, bought.accessToken)).toBe(companyID);
		const agent = await callingAgent(requestBearing(bought.accessToken), environment);
		expect(agent.companyID).toBe(companyID);
	});

	test('a refreshed host session reads the model key its company sealed to the box', async () => {
		const box = await aBox();
		await announceBox(client, box.publicKey, box.encryptionKey, officeAddress);
		await claimBox(client, companyID, box.publicKey, officeAddress);
		const sealedModelKey = { ephemeralPublicKey: box.encryptionKey, nonce: 'MzMzMzMzMzMzMzMz', ciphertext: 'c2VhbGVk' };
		await keepSealedModelKey(client, companyID, sealedModelKey);
		const renewed = await refreshedThroughTheProject((await refreshableSessionOf(box)).refreshToken);

		const answered = await fetch(`${projectURL}/rest/v1/rpc/box_sealed_model_key`, {
			method: 'POST',
			headers: { apikey: publishableKey, Authorization: `Bearer ${renewed.access_token}`, 'Content-Type': 'application/json' },
			body: '{}'
		});

		expect(answered.status).toBe(200);
		expect(sealedModelKeySchema.parse(await answered.json())).toEqual(sealedModelKey);
	});

	test('a refresh token nobody issued renews nothing', async () => {
		const answered = await refreshWith('not-a-refresh-token');

		expect(answered.status).toBe(400);
	});

	test('a member’s own session buys no host session', async () => {
		const memberSession = await sessionForMember(credentials, adminMemberID);

		await expect(sessionForHost(credentials, memberSession.accessToken)).rejects.toThrow('no company computer');
	});
});
