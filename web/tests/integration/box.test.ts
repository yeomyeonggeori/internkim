import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { SignJWT, exportJWK, generateKeyPair } from 'jose';
import { x25519 } from '@noble/curves/ed25519.js';
import { base64URLOf } from '../../src/lib/company/seal-to-box';
import { pairingPageAddressSchema } from '../../src/lib/company/box';
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
	verifyBoxCode,
	connectedBoxOf,
	emptyBoxesAt,
	keepSealedModelKey,
	releaseBox
} from '../../src/lib/server/box';
import {
	asMember,
	companyOfHostSession,
	controlPlane,
	fleetCredentialKind,
	issueAgentKey,
	provisionCompany,
	sessionForHost,
	sessionForMember
} from '../../src/lib/server/control-plane';
import { recordTokenFor } from '../../src/lib/server/record-token';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

const networkHookTimeout = 60_000;
const credentials = { projectURL, publishableKey, serviceRoleKey, signingKey };
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

async function claimWithCode(claimingCompanyID: string, publicKey: string, code: string): Promise<void> {
	const { ticket } = await verifyBoxCode(client, claimingCompanyID, publicKey, code);
	await claimBox(client, claimingCompanyID, publicKey, ticket);
}

async function announcedCode(box: Box, address = officeAddress): Promise<string> {
	const announced = await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: address, wantsPairingCode: true });
	if (announced.isClaimed || !announced.pairingCode) throw new Error('an empty box that asks for a code is given one');
	return announced.pairingCode;
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

async function clearRefusalsOf(refusedCompanyID: string): Promise<void> {
	await client.from('box_pairing_refusal').delete().eq('company_id', refusedCompanyID);
}

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
	test('a box says where on its own network it shows its code, and only an address there', async () => {
		const box = await aBox();
		const pairingPageAddresses = ['http://kimmini.local:18088/', 'http://192.168.0.23:18088/'];
		await announceBox(client, {
			publicKey: box.publicKey,
			encryptionKey: box.encryptionKey,
			publicAddress: officeAddress,
			wantsPairingCode: true,
			hostName: 'kimmini',
			pairingPageAddresses
		});

		const listed = (await emptyBoxesAt(client, officeAddress)).find((empty) => empty.publicKey === box.publicKey);
		expect(listed).toMatchObject({ hostName: 'kimmini', pairingPageAddresses });
		expect(pairingPageAddressSchema.safeParse('https://phishing.example.test/').success).toBe(false);
		expect(pairingPageAddressSchema.safeParse('http://8.8.8.8:18088/').success).toBe(false);
	});

	test('an announced box is listed only at the address it announced from', async () => {
		const box = await aBox();
		expect(await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false })).toMatchObject({ isClaimed: false });

		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).toContain(box.publicKey);
		expect((await emptyBoxesAt(client, neighbourAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
	});

	test('a box that stopped announcing drops out of the list', async () => {
		const box = await aBox();
		await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false });
		const later = new Date(Date.now() + 11 * 60 * 1000);

		expect((await emptyBoxesAt(client, officeAddress, later)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
	});

	test('a right code claims nothing until the administrator confirms, and is not counted as a guess', async () => {
		const box = await aBox();
		const code = await announcedCode(box);

		const verified = await verifyBoxCode(client, companyID, box.publicKey, code);

		expect(verified).toMatchObject({ publicKey: box.publicKey, publicAddress: officeAddress });
		expect(await connectedBoxOf(client, companyID)).toBeNull();
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).toContain(box.publicKey);
		const { count } = await client
			.from('box_pairing_refusal')
			.select('*', { count: 'exact', head: true })
			.eq('company_id', companyID);
		expect(count).toBe(0);

		await expect(claimBox(client, companyID, box.publicKey, code)).rejects.toBeInstanceOf(BoxRefused);
		await expect(verifyBoxCode(client, companyID, box.publicKey, code)).rejects.toBeInstanceOf(BoxRefused);
		await claimBox(client, companyID, box.publicKey, verified.ticket);
		expect((await connectedBoxOf(client, companyID))?.publicKey).toBe(box.publicKey);
		await expect(claimBox(client, companyID, box.publicKey, verified.ticket)).rejects.toBeInstanceOf(BoxRefused);
		await clearRefusalsOf(companyID);
	});

	test('a box is given a short code, keeps it while it lives, and gets a new one when it asks', async () => {
		const box = await aBox();
		const first = await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false });
		if (first.isClaimed || !first.pairingCode || !first.pairingCodeExpiresAt) throw new Error('a new box is given a code');

		expect(first.pairingCode).toMatch(/^[A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4}$/);
		expect(Date.parse(first.pairingCodeExpiresAt) - Date.now()).toBeLessThanOrEqual(15 * 60 * 1000);
		expect(await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false })).toEqual({ isClaimed: false });

		const asked = await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: true });
		if (asked.isClaimed || !asked.pairingCode) throw new Error('a box that asks for a code is given one');
		expect(asked.pairingCode).not.toBe(first.pairingCode);
	});

	test('the record keeps the hash of the code, never the code', async () => {
		const box = await aBox();
		const code = await announcedCode(box);
		const { data } = await client.from('empty_box').select('*').eq('public_key', box.publicKey).single();

		expect(JSON.stringify(data)).not.toContain(code.replace('-', ''));
		expect(JSON.stringify(data)).not.toContain(code);
	});

	test('a wrong code claims nothing, from the box’s own network or any other', async () => {
		const box = await aBox();
		await announcedCode(box);

		await expect(verifyBoxCode(client, companyID, box.publicKey, 'AAAA-AAAA')).rejects.toBeInstanceOf(BoxRefused);
		expect((await connectedBoxOf(client, companyID))?.publicKey).not.toBe(box.publicKey);
		await clearRefusalsOf(companyID);
	});

	test('the code is claimed from any network, typed in any case and spacing', async () => {
		const outsider = await aCompany('remote');
		const box = await aBox();
		const code = await announcedCode(box, neighbourAddress);

		await claimWithCode(outsider.companyID, box.publicKey, ` ${code.toLowerCase().replace('-', ' ')} `);

		expect((await connectedBoxOf(client, outsider.companyID))?.publicKey).toBe(box.publicKey);
	});

	test('five wrong codes spend the code, and the box is given another', async () => {
		const box = await aBox();
		const code = await announcedCode(box);
		for (let attempt = 0; attempt < 5; attempt += 1) {
			await expect(verifyBoxCode(client, companyID, box.publicKey, 'AAAA-AAAA')).rejects.toBeInstanceOf(BoxRefused);
		}

		await expect(verifyBoxCode(client, companyID, box.publicKey, code)).rejects.toBeInstanceOf(BoxRefused);
		expect(await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false })).toMatchObject({
			pairingCode: expect.any(String)
		});
		await clearRefusalsOf(companyID);
	});

	test('an expired code claims nothing', async () => {
		const box = await aBox();
		const code = await announcedCode(box);
		await client
			.from('empty_box')
			.update({ pairing_code_expires_at: new Date(Date.now() - 1000).toISOString() })
			.eq('public_key', box.publicKey);

		await expect(verifyBoxCode(client, companyID, box.publicKey, code)).rejects.toBeInstanceOf(BoxRefused);
		await clearRefusalsOf(companyID);
	});

	test('a company that guesses twenty times in an hour is refused even the right code', async () => {
		const guesser = await aCompany('guesser');
		const boxes = await Promise.all([aBox(), aBox(), aBox(), aBox(), aBox()]);
		for (const box of boxes) await announcedCode(box);
		for (const box of boxes) {
			for (let attempt = 0; attempt < 4; attempt += 1) {
				await expect(verifyBoxCode(client, guesser.companyID, box.publicKey, 'AAAA-AAAA')).rejects.toBeInstanceOf(BoxRefused);
			}
		}
		const target = await aBox();
		const code = await announcedCode(target);

		await expect(verifyBoxCode(client, guesser.companyID, target.publicKey, code)).rejects.toMatchObject({ status: 429 });
		await claimWithCode(companyID, target.publicKey, code);
		expect((await connectedBoxOf(client, companyID))?.publicKey).toBe(target.publicKey);
	});

	test('claiming binds the box to the company and takes it off the list', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));

		expect(await connectedBoxOf(client, companyID)).toMatchObject({
			companyID,
			publicKey: box.publicKey,
			encryptionKey: box.encryptionKey,
			hasModelKey: false
		});
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
		expect(await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: true })).toEqual({ isClaimed: true });
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
	});

	test('a claimed box gets its company, a host session and the model key sealed to it', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const sealedModelKey = { version: 1 as const, recipient: box.encryptionKey, enc: box.encryptionKey, ciphertext: 'c2VhbGVk' };
		await keepSealedModelKey(client, companyID, sealedModelKey);

		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);

		expect(answered?.configuration.company.id).toBe(companyID);
		expect(answered?.configuration.appURL).toBe(appURL);
		expect(answered?.sealedModelKey).toEqual(sealedModelKey);
		expect(await companyOfHostSession(credentials, answered?.session.accessToken ?? '')).toBe(companyID);
		expect((await connectedBoxOf(client, companyID))?.lastSeenAt).not.toBeNull();
	});

	test('a model key sealed before HPKE still reaches its box until it is given again', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const sealedBeforeHPKE = { ephemeralPublicKey: box.encryptionKey, nonce: 'MzMzMzMzMzMzMzMz', ciphertext: 'c2VhbGVk' };
		const { error } = await client
			.from('credential')
			.update({ settings: { encryptionKey: box.encryptionKey, sealedModelKey: sealedBeforeHPKE } })
			.eq('company_id', companyID)
			.eq('kind', fleetCredentialKind);
		expect(error).toBeNull();

		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);

		expect(answered?.sealedModelKey).toEqual(sealedBeforeHPKE);
	});

	test('an unclaimed box gets no session', async () => {
		const box = await aBox();
		await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false });

		expect(await boxSessionFor(credentials, box.publicKey, environment, appURL)).toBeNull();
	});

	test('connecting a second box moves the company to it', async () => {
		const first = await aBox();
		const second = await aBox();
		const firstCode = await announcedCode(first);
		const secondCode = await announcedCode(second);

		await claimWithCode(companyID, first.publicKey, firstCode);
		await claimWithCode(companyID, second.publicKey, secondCode);

		expect((await connectedBoxOf(client, companyID))?.publicKey).toBe(second.publicKey);
		expect(await boxSessionFor(credentials, first.publicKey, environment, appURL)).toBeNull();
	});

	test('an administrator disconnects a box, which then announces as empty and is given a new code', async () => {
		const releasing = await aCompany('releasing');
		const box = await aBox();
		await claimWithCode(releasing.companyID, box.publicKey, await announcedCode(box));

		expect(await releaseBox(client, releasing.companyID)).toBe(true);

		expect(await connectedBoxOf(client, releasing.companyID)).toBeNull();
		expect(await boxSessionFor(credentials, box.publicKey, environment, appURL)).toBeNull();
		const announced = await announceBox(client, {
			publicKey: box.publicKey,
			encryptionKey: box.encryptionKey,
			publicAddress: officeAddress,
			wantsPairingCode: true
		});
		expect(announced).toMatchObject({ isClaimed: false, pairingCode: expect.any(String) });
		expect(await releaseBox(client, releasing.companyID)).toBe(false);
	});

	test('a model key waits for a connected box', async () => {
		const empty = await aCompany('empty');
		const boxKey = 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM';
		const sealedModelKey = { version: 1 as const, recipient: boxKey, enc: boxKey, ciphertext: 'c2VhbGVk' };

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
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
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
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);
		const presented = answered?.session;
		if (!presented) throw new Error('a claimed box gets a session');

		expect(await sessionForHost(credentials, presented.accessToken)).toEqual(presented);
	});

	test('a computer the company moved off can neither call its routes nor touch its record, though its session has not expired', async () => {
		const first = await aBox();
		const second = await aBox();
		await claimWithCode(companyID, first.publicKey, await announcedCode(first));
		const forgotten = (await boxSessionFor(credentials, first.publicKey, environment, appURL))?.session.accessToken ?? '';
		await claimWithCode(companyID, second.publicKey, await announcedCode(second));
		const current = (await boxSessionFor(credentials, second.publicKey, environment, appURL))?.session.accessToken ?? '';

		await expect(callingAgent(requestBearing(forgotten), environment)).rejects.toMatchObject({ status: 403 });
		await expect(sessionForHost(credentials, forgotten)).rejects.toThrow('no company computer');
		const planted = await asMember({ projectURL, publishableKey }, forgotten)
			.from('contact')
			.insert({ company_id: companyID, name: `Planted by a forgotten computer ${stamp}` });
		expect(planted.error?.code).toBe('42501');

		expect((await callingAgent(requestBearing(current), environment)).companyID).toBe(companyID);
		const kept = await asMember({ projectURL, publishableKey }, current)
			.from('contact')
			.insert({ company_id: companyID, name: `Kept by the connected computer ${stamp}` });
		expect(kept.error).toBeNull();
	});

	test('a computer the administrator disconnected is refused at once', async () => {
		const disconnecting = await aCompany('disconnecting');
		const box = await aBox();
		await claimWithCode(disconnecting.companyID, box.publicKey, await announcedCode(box));
		const held = (await boxSessionFor(credentials, box.publicKey, environment, appURL))?.session.accessToken ?? '';
		expect(await companyOfHostSession(credentials, held)).toBe(disconnecting.companyID);

		await releaseBox(client, disconnecting.companyID);

		expect(await companyOfHostSession(credentials, held)).toBeNull();
	});

	test('a session bought with an agent key ends when the company replaces that key', async () => {
		const name = `replaced-key-${stamp}`;
		const first = await issueAgentKey(client, companyID, name);
		const bought = await sessionForHost(credentials, first.apiKey);
		expect(await companyOfHostSession(credentials, bought.accessToken)).toBe(companyID);

		await issueAgentKey(client, companyID, name, { replaceStanding: true });

		expect(await companyOfHostSession(credentials, bought.accessToken)).toBeNull();
	});

	test('a member’s own session buys no host session', async () => {
		const memberSession = await sessionForMember(credentials, adminMemberID);

		await expect(sessionForHost(credentials, memberSession.accessToken)).rejects.toThrow('no company computer');
	});
});
