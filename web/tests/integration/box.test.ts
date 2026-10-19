import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { SignJWT, decodeJwt, exportJWK, generateKeyPair } from 'jose';
import { x25519 } from '@noble/curves/ed25519.js';
import { base64URLOf } from '../../src/lib/company/seal-to-box';
import { pairingPageAddressSchema, type BoxRole } from '../../src/lib/company/box';
import { companyComputerName } from '../../src/lib/company/host-setup';
import { callingAgent } from '../../src/lib/server/agent-request';
import {
	announceBox,
	boxAssertionAudience,
	boxKeyOfAssertion,
	boxOfPublicKey,
	BoxRefused,
	aimStandbyBoxesAt,
	boxSessionFor,
	type ClaimIntent,
	claimBox,
	claimBoxWithConnectionFile,
	verifyBoxCode,
	companyBoxesOf,
	keepSealedCompanyKeys,
	emptyBoxesAt,
	keepSealedModelKey,
	keepTakeOver,
	makeBoxServe,
	placeBox,
	newestKeptBackupAt,
	releaseBox,
	releaseBoxOfKey
} from '../../src/lib/server/box';
import { pendingWifiChangeOf, recordNearbyNetworks, reportWifiOutcome, requestWifiChange, wifiChangeStatusFor } from '../../src/lib/server/box-wifi';
import { adminPasswordStatusFor, noteAdminAccount, pendingAdminPasswordOf, reportAdminPasswordOutcome, requestAdminPassword } from '../../src/lib/server/box-admin-password';
import {
	asMember,
	companyOfHostSession,
	controlPlane,
	hostAddressOf,
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

async function claimWithCode(
	claimingCompanyID: string,
	publicKey: string,
	code: string,
	intent: ClaimIntent = 'replace'
): Promise<BoxRole> {
	const { ticket } = await verifyBoxCode(client, claimingCompanyID, publicKey, code);
	return claimBox(client, claimingCompanyID, publicKey, ticket, intent);
}

async function connectedBox(holdingCompanyID: string, publicKey: string) {
	return (await companyBoxesOf(client, holdingCompanyID)).boxes.find((box) => box.publicKey === publicKey) ?? null;
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
		expect(await connectedBox(companyID, box.publicKey)).toBeNull();
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).toContain(box.publicKey);
		const { count } = await client
			.from('box_pairing_refusal')
			.select('*', { count: 'exact', head: true })
			.eq('company_id', companyID);
		expect(count).toBe(0);

		await expect(claimBox(client, companyID, box.publicKey, code, 'replace')).rejects.toBeInstanceOf(BoxRefused);
		await expect(verifyBoxCode(client, companyID, box.publicKey, code)).rejects.toBeInstanceOf(BoxRefused);
		await claimBox(client, companyID, box.publicKey, verified.ticket, 'replace');
		expect((await connectedBox(companyID, box.publicKey))?.publicKey).toBe(box.publicKey);
		await expect(claimBox(client, companyID, box.publicKey, verified.ticket, 'replace')).rejects.toBeInstanceOf(BoxRefused);
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
		expect(await connectedBox(companyID, box.publicKey)).toBeNull();
		await clearRefusalsOf(companyID);
	});

	test('the code is claimed from any network, typed in any case and spacing', async () => {
		const outsider = await aCompany('remote');
		const box = await aBox();
		const code = await announcedCode(box, neighbourAddress);

		await claimWithCode(outsider.companyID, box.publicKey, ` ${code.toLowerCase().replace('-', ' ')} `);

		expect((await connectedBox(outsider.companyID, box.publicKey))?.publicKey).toBe(box.publicKey);
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
		expect((await connectedBox(companyID, target.publicKey))?.publicKey).toBe(target.publicKey);
	});

	test('claiming binds the box to the company and takes it off the list', async () => {
		const box = await aBox();
		expect(await claimWithCode(companyID, box.publicKey, await announcedCode(box))).toBe('service');

		expect(await connectedBox(companyID, box.publicKey)).toMatchObject({
			companyID,
			publicKey: box.publicKey,
			encryptionKey: box.encryptionKey,
			role: 'service',
			holdsCompanyKey: true
		});
		expect((await companyBoxesOf(client, companyID)).sealingKey).toBe(box.encryptionKey);
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
		expect(await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: true })).toEqual({ isClaimed: true });
		expect((await emptyBoxesAt(client, officeAddress)).map((listed) => listed.publicKey)).not.toContain(box.publicKey);
	});

	test('a claimed box gets its company, a host session, its role and the model key sealed to the company', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const sealedModelKey = { version: 1 as const, recipient: box.encryptionKey, enc: box.encryptionKey, ciphertext: 'c2VhbGVk' };
		await keepSealedModelKey(client, companyID, sealedModelKey);

		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);

		expect(answered?.configuration.company.id).toBe(companyID);
		expect(answered?.configuration.appURL).toBe(appURL);
		expect(answered?.role).toBe('service');
		expect(answered?.sealingKey).toBe(box.encryptionKey);
		expect(answered?.sealedModelKey).toEqual(sealedModelKey);
		expect(await companyOfHostSession(credentials, answered?.session.accessToken ?? '')).toBe(companyID);
		expect((await connectedBox(companyID, box.publicKey))?.lastSeenAt).not.toBeNull();
	});

	test('a model key sealed to a key the company no longer seals to is refused', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const stranger = (await aBox()).encryptionKey;
		const sealedModelKey = { version: 1 as const, recipient: stranger, enc: stranger, ciphertext: 'c2VhbGVk' };

		await expect(keepSealedModelKey(client, companyID, sealedModelKey)).rejects.toBeInstanceOf(BoxRefused);
	});

	test('a model key sealed before HPKE still reaches its box until it is given again', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const sealedBeforeHPKE = { ephemeralPublicKey: box.encryptionKey, nonce: 'MzMzMzMzMzMzMzMz', ciphertext: 'c2VhbGVk' };
		const { error } = await client
			.from('box_keyring')
			.update({ sealed_model_key: sealedBeforeHPKE })
			.eq('company_id', companyID);
		expect(error).toBeNull();

		const answered = await boxSessionFor(credentials, box.publicKey, environment, appURL);

		expect(answered?.sealedModelKey).toEqual(sealedBeforeHPKE);
	});

	test('an unclaimed box gets no session', async () => {
		const box = await aBox();
		await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false });

		expect(await boxSessionFor(credentials, box.publicKey, environment, appURL)).toBeNull();
	});

	test('replacing with a second box moves the company to it and forgets what only the first could open', async () => {
		const first = await aBox();
		const second = await aBox();
		await claimWithCode(companyID, first.publicKey, await announcedCode(first));
		await keepSealedModelKey(client, companyID, { version: 1, recipient: first.encryptionKey, enc: first.encryptionKey, ciphertext: 'c2VhbGVk' });

		await claimWithCode(companyID, second.publicKey, await announcedCode(second), 'replace');

		const held = await companyBoxesOf(client, companyID);
		expect(held.boxes.map((box) => box.publicKey)).toEqual([second.publicKey]);
		expect(held.sealingKey).toBe(second.encryptionKey);
		expect(held.hasModelKey).toBe(false);
		expect(await boxSessionFor(credentials, first.publicKey, environment, appURL)).toBeNull();
	});

	test('adding a second box keeps the first serving, and the second waits for the company key from it', async () => {
		const adding = await aCompany('adding');
		const first = await aBox();
		const second = await aBox();
		await claimWithCode(adding.companyID, first.publicKey, await announcedCode(first));
		const sealedModelKey = { version: 1 as const, recipient: first.encryptionKey, enc: first.encryptionKey, ciphertext: 'c2VhbGVk' };
		await keepSealedModelKey(client, adding.companyID, sealedModelKey);

		expect(await claimWithCode(adding.companyID, second.publicKey, await announcedCode(second), 'add')).toBe('standby');

		const serving = await boxSessionFor(credentials, first.publicKey, environment, appURL);
		const waiting = await boxSessionFor(credentials, second.publicKey, environment, appURL);
		expect(serving?.role).toBe('service');
		expect(serving?.boxesAwaitingCompanyKey).toEqual([{ publicKey: second.publicKey, encryptionKey: second.encryptionKey }]);
		expect(waiting?.role).toBe('standby');
		expect(waiting?.sealingKey).toBe(first.encryptionKey);
		expect(waiting?.sealedCompanyKey).toBeNull();
		expect(waiting?.sealedModelKey).toBeNull();
		expect(await companyOfHostSession(credentials, waiting?.session.accessToken ?? '')).toBe(adding.companyID);
		expect(await companyOfHostSession(credentials, serving?.session.accessToken ?? '')).toBe(adding.companyID);
	});

	test('a box that holds the company key hands it on, and the receiving box then gets the model key', async () => {
		const handing = await aCompany('handing');
		const first = await aBox();
		const second = await aBox();
		await claimWithCode(handing.companyID, first.publicKey, await announcedCode(first));
		await claimWithCode(handing.companyID, second.publicKey, await announcedCode(second), 'add');
		const sealedModelKey = { version: 1 as const, recipient: first.encryptionKey, enc: first.encryptionKey, ciphertext: 'c2VhbGVk' };
		await keepSealedModelKey(client, handing.companyID, sealedModelKey);
		const holder = await boxOfPublicKey(client, first.publicKey);
		const waiting = await boxOfPublicKey(client, second.publicKey);
		if (!holder || !waiting) throw new Error('both claimed boxes are found by their public keys');
		const sealedCompanyKey = { version: 1 as const, recipient: second.encryptionKey, enc: second.encryptionKey, ciphertext: 'a2V5' };

		await expect(keepSealedCompanyKeys(client, waiting, [{ publicKey: second.publicKey, sealed: sealedCompanyKey }])).rejects.toBeInstanceOf(BoxRefused);
		await expect(keepSealedCompanyKeys(client, holder, [{ publicKey: second.publicKey, sealed: { ...sealedCompanyKey, recipient: first.encryptionKey } }])).rejects.toBeInstanceOf(BoxRefused);
		await keepSealedCompanyKeys(client, holder, [{ publicKey: second.publicKey, sealed: sealedCompanyKey }]);

		const received = await boxSessionFor(credentials, second.publicKey, environment, appURL);
		expect(received?.sealedCompanyKey).toEqual(sealedCompanyKey);
		expect(received?.sealedModelKey).toEqual(sealedModelKey);
		expect((await boxSessionFor(credentials, first.publicKey, environment, appURL))?.boxesAwaitingCompanyKey).toEqual([]);
		expect((await connectedBox(handing.companyID, second.publicKey))?.holdsCompanyKey).toBe(true);
		await expect(keepSealedCompanyKeys(client, holder, [{ publicKey: second.publicKey, sealed: sealedCompanyKey }])).rejects.toBeInstanceOf(BoxRefused);
	});

	test('a box of another company is never handed this company\'s key', async () => {
		const ours = await aCompany('key-ours');
		const theirs = await aCompany('key-theirs');
		const holderBox = await aBox();
		const strangerBox = await aBox();
		await claimWithCode(ours.companyID, holderBox.publicKey, await announcedCode(holderBox));
		await claimWithCode(theirs.companyID, strangerBox.publicKey, await announcedCode(strangerBox));
		const holder = await boxOfPublicKey(client, holderBox.publicKey);
		if (!holder) throw new Error('a claimed box is found by its public key');
		const sealed = { version: 1 as const, recipient: strangerBox.encryptionKey, enc: strangerBox.encryptionKey, ciphertext: 'a2V5' };

		await expect(keepSealedCompanyKeys(client, holder, [{ publicKey: strangerBox.publicKey, sealed }])).rejects.toMatchObject({ status: 404 });
	});

	test('a company runs one service box however many it adds', async () => {
		const counting = await aCompany('counting');
		const roles: BoxRole[] = [];
		for (let index = 0; index < 3; index += 1) {
			const box = await aBox();
			roles.push(await claimWithCode(counting.companyID, box.publicKey, await announcedCode(box), 'add'));
		}
		expect(roles).toEqual(['service', 'standby', 'standby']);
		const held = await companyBoxesOf(client, counting.companyID);
		expect(held.boxes.filter((box) => box.role === 'service')).toHaveLength(1);
	});

	test('an administrator makes a standby box serve, and the box that served stands by', async () => {
		const switching = await aCompany('switching');
		const first = await aBox();
		const second = await aBox();
		const stranger = await aBox();
		await claimWithCode(switching.companyID, first.publicKey, await announcedCode(first));
		await claimWithCode(switching.companyID, second.publicKey, await announcedCode(second), 'add');

		expect(await makeBoxServe(client, switching.companyID, stranger.publicKey)).toBe('not_its_box');
		expect(await makeBoxServe(client, switching.companyID, second.publicKey)).toBe('no_backup');
		expect(await boxSessionFor(credentials, second.publicKey, environment, appURL)).toMatchObject({
			role: 'standby',
			keepsBackups: false
		});

		const keptManifest = `${switching.companyID}/snapshots/internkim-snapshot-20261020T000000Z.json`;
		await client.storage.from('box_backup').upload(keptManifest, new Blob(['{}'], { type: 'application/json' }));
		expect(await makeBoxServe(client, switching.companyID, second.publicKey)).toBe('serving');

		expect(await boxSessionFor(credentials, second.publicKey, environment, appURL)).toMatchObject({
			role: 'service',
			keepsBackups: true
		});
		expect((await boxSessionFor(credentials, first.publicKey, environment, appURL))?.role).toBe('standby');
		await client.storage.from('box_backup').remove([keptManifest]);
	});

	test('a standby box that holds the company key takes over a service box the company has not seen, once the company lets it and keeps a backup', async () => {
		const lapsing = await aCompany('lapsing');
		const first = await aBox();
		const second = await aBox();
		await claimWithCode(lapsing.companyID, first.publicKey, await announcedCode(first));
		await claimWithCode(lapsing.companyID, second.publicKey, await announcedCode(second), 'add');
		const holder = await boxOfPublicKey(client, first.publicKey);
		if (!holder) throw new Error('a claimed box is found by its public key');
		const sealed = { version: 1 as const, recipient: second.encryptionKey, enc: second.encryptionKey, ciphertext: 'a2V5' };
		await keepSealedCompanyKeys(client, holder, [{ publicKey: second.publicKey, sealed }]);
		expect(await placeBox(client, lapsing.companyID, first.publicKey, { site: 'Head office', weight: 1 })).toBe(true);
		expect(await placeBox(client, lapsing.companyID, second.publicKey, { site: 'Annex', weight: 2 })).toBe(true);
		await expect(placeBox(client, lapsing.companyID, second.publicKey, { site: 'Annex', weight: 0 })).rejects.toMatchObject({ status: 400 });
		await client.from('box').update({ last_seen_at: new Date(Date.now() - 3_600_000).toISOString() }).eq('public_key', first.publicKey);

		expect((await boxSessionFor(credentials, second.publicKey, environment, appURL))?.role).toBe('standby');
		await expect(keepTakeOver(client, lapsing.companyID, { isAutomatic: true, lapseSeconds: 10 })).rejects.toMatchObject({ status: 400 });
		await keepTakeOver(client, lapsing.companyID, { isAutomatic: true, lapseSeconds: 300 });
		expect((await boxSessionFor(credentials, second.publicKey, environment, appURL))?.role).toBe('standby');

		const keptManifest = `${lapsing.companyID}/snapshots/internkim-snapshot-20261020T000000Z.json`;
		await client.storage.from('box_backup').upload(keptManifest, new Blob(['{}'], { type: 'application/json' }));

		expect(await boxSessionFor(credentials, second.publicKey, environment, appURL)).toMatchObject({
			role: 'service',
			takeOver: { isAutomatic: true, lapseSeconds: 300 }
		});
		expect((await boxSessionFor(credentials, first.publicKey, environment, appURL))?.role).toBe('standby');
		expect((await companyBoxesOf(client, lapsing.companyID)).takeOver).toEqual({ isAutomatic: true, lapseSeconds: 300 });
		await client.storage.from('box_backup').remove([keptManifest]);
	});

	test('a standby box is aimed at the release the service box is about to take, and the service box waits for it', async () => {
		const updating = await aCompany('updating');
		const first = await aBox();
		const second = await aBox();
		await claimWithCode(updating.companyID, first.publicKey, await announcedCode(first));
		await claimWithCode(updating.companyID, second.publicKey, await announcedCode(second), 'add');
		await boxSessionFor(credentials, first.publicKey, environment, appURL, 'v0.0.1');
		expect((await boxSessionFor(credentials, second.publicKey, environment, appURL, 'v0.0.1'))?.targetVersion).toBeNull();

		await expect(aimStandbyBoxesAt(client, second.publicKey, 'v0.0.2')).rejects.toMatchObject({ status: 409 });
		expect((await aimStandbyBoxesAt(client, first.publicKey, 'v0.0.2')).map((short) => short.publicKey)).toEqual([second.publicKey]);
		expect((await boxSessionFor(credentials, second.publicKey, environment, appURL, 'v0.0.1'))?.targetVersion).toBe('v0.0.2');
		expect((await boxSessionFor(credentials, first.publicKey, environment, appURL, 'v0.0.1'))?.targetVersion).toBeNull();

		await boxSessionFor(credentials, second.publicKey, environment, appURL, 'v0.0.2');
		expect(await aimStandbyBoxesAt(client, first.publicKey, 'v0.0.2')).toEqual([]);
	});

	test('the newest snapshot the boxes share is the newest readable one sealed to the company key', async () => {
		const backingUp = await aCompany('backing-up');
		const first = await aBox();
		await claimWithCode(backingUp.companyID, first.publicKey, await announcedCode(first));
		const bucket = client.storage.from('box_backup');
		const snapshots = `${backingUp.companyID}/snapshots`;
		const manifestOf = (name: string, sealingKey: string, createdAt: string) =>
			new Blob([JSON.stringify({ version: 2, name, sealingKey, createdAt, archive: {}, chunks: {} })], { type: 'application/json' });
		expect(await newestKeptBackupAt(client, backingUp.companyID, first.encryptionKey)).toBeNull();

		await bucket.upload(`${snapshots}/internkim-snapshot-20261017T030000Z.json`, manifestOf('internkim-snapshot-20261017T030000Z.json', first.encryptionKey, '2026-10-17T03:00:00Z'));
		await bucket.upload(`${snapshots}/internkim-snapshot-20261018T030000Z.json`, manifestOf('internkim-snapshot-20261018T030000Z.json', 'o'.repeat(43), '2026-10-18T03:00:00Z'));
		await bucket.upload(`${snapshots}/internkim-snapshot-20261019T030000Z.json`, new Blob(['half written']));

		expect(await newestKeptBackupAt(client, backingUp.companyID, first.encryptionKey)).toBe('2026-10-17T03:00:00Z');
		await bucket.remove([
			`${snapshots}/internkim-snapshot-20261017T030000Z.json`,
			`${snapshots}/internkim-snapshot-20261018T030000Z.json`,
			`${snapshots}/internkim-snapshot-20261019T030000Z.json`
		]);
	});


	test('an administrator disconnects one box, which then announces as empty, and the others stay', async () => {
		const releasing = await aCompany('releasing');
		const box = await aBox();
		const staying = await aBox();
		await claimWithCode(releasing.companyID, box.publicKey, await announcedCode(box));
		await claimWithCode(releasing.companyID, staying.publicKey, await announcedCode(staying), 'add');

		expect(await releaseBox(client, releasing.companyID, box.publicKey)).toBe(true);

		expect(await connectedBox(releasing.companyID, box.publicKey)).toBeNull();
		expect(await connectedBox(releasing.companyID, staying.publicKey)).not.toBeNull();
		expect(await boxSessionFor(credentials, box.publicKey, environment, appURL)).toBeNull();
		const announced = await announceBox(client, {
			publicKey: box.publicKey,
			encryptionKey: box.encryptionKey,
			publicAddress: officeAddress,
			wantsPairingCode: true
		});
		expect(announced).toMatchObject({ isClaimed: false, pairingCode: expect.any(String) });
		expect(await releaseBox(client, releasing.companyID, box.publicKey)).toBe(false);
	});

	test('an administrator cannot disconnect another company\'s box', async () => {
		const ours = await aCompany('release-ours');
		const theirs = await aCompany('release-theirs');
		const box = await aBox();
		await claimWithCode(theirs.companyID, box.publicKey, await announcedCode(box));

		expect(await releaseBox(client, ours.companyID, box.publicKey)).toBe(false);
		expect(await connectedBox(theirs.companyID, box.publicKey)).not.toBeNull();
	});

	test('a box reset by whoever holds it releases itself from the company that claimed it', async () => {
		const claiming = await aCompany('claiming-by-mistake');
		const box = await aBox();
		await claimWithCode(claiming.companyID, box.publicKey, await announcedCode(box));

		expect(await releaseBoxOfKey(client, box.publicKey)).toBe(true);

		expect(await connectedBox(claiming.companyID, box.publicKey)).toBeNull();
		expect(await boxSessionFor(credentials, box.publicKey, environment, appURL)).toBeNull();
		const announced = await announceBox(client, {
			publicKey: box.publicKey,
			encryptionKey: box.encryptionKey,
			publicAddress: officeAddress,
			wantsPairingCode: true
		});
		expect(announced).toMatchObject({ isClaimed: false, pairingCode: expect.any(String) });
		expect(await releaseBoxOfKey(client, box.publicKey)).toBe(false);
	});

	test('a model key waits for a connected box', async () => {
		const empty = await aCompany('empty');
		const boxKey = 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM';
		const sealedModelKey = { version: 1 as const, recipient: boxKey, enc: boxKey, ciphertext: 'c2VhbGVk' };

		await expect(keepSealedModelKey(client, empty.companyID, sealedModelKey)).rejects.toBeInstanceOf(BoxRefused);
	});
});

describe('changing the Wi-Fi network of a connected box', () => {
	test('an administrator requests a change, the box fetches it, and reports back', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const requestID = crypto.randomUUID();
		const sealed = { version: 1 as const, recipient: box.encryptionKey, enc: box.encryptionKey, ciphertext: 'c2VhbGVk' };

		expect(await requestWifiChange(client, companyID, box.publicKey, requestID, sealed)).toEqual({ pendingRequestID: requestID, outcome: null, nearbyNetworks: [], scannedAt: null });

		const fetched = await boxOfPublicKey(client, box.publicKey);
		if (!fetched) throw new Error('a claimed box is found by its public key');
		expect(pendingWifiChangeOf(fetched)).toEqual({ requestID, sealed });

		await reportWifiOutcome(client, fetched, requestID, 'joined');

		expect(await wifiChangeStatusFor(client, companyID, box.publicKey)).toMatchObject({
			pendingRequestID: null,
			outcome: { requestID, result: 'joined' }
		});
		const cleared = await boxOfPublicKey(client, box.publicKey);
		if (!cleared) throw new Error('a claimed box is found by its public key');
		expect(pendingWifiChangeOf(cleared)).toBeNull();
	});

	test('an outcome naming a request that is not pending is refused, and clears nothing', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const requestID = crypto.randomUUID();
		const sealed = { version: 1 as const, recipient: box.encryptionKey, enc: box.encryptionKey, ciphertext: 'c2VhbGVk' };
		await requestWifiChange(client, companyID, box.publicKey, requestID, sealed);
		const current = await boxOfPublicKey(client, box.publicKey);
		if (!current) throw new Error('a claimed box is found by its public key');

		await expect(reportWifiOutcome(client, current, crypto.randomUUID(), 'failed')).rejects.toBeInstanceOf(BoxRefused);

		expect(await wifiChangeStatusFor(client, companyID, box.publicKey)).toMatchObject({ pendingRequestID: requestID, outcome: null });
	});

	test('the nearby networks a box reports are read back by an administrator', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const fetched = await boxOfPublicKey(client, box.publicKey);
		if (!fetched) throw new Error('a claimed box is found by its public key');
		const networks = [{ ssid: 'Sample Office', signalPercent: 80, isSecured: true }];

		await recordNearbyNetworks(client, fetched, networks);

		expect(await wifiChangeStatusFor(client, companyID, box.publicKey)).toMatchObject({ nearbyNetworks: networks });
	});

	test('a fetch without nearby networks keeps the previous list', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const fetched = await boxOfPublicKey(client, box.publicKey);
		if (!fetched) throw new Error('a claimed box is found by its public key');
		const networks = [{ ssid: 'Sample Office', signalPercent: 80, isSecured: false }];
		await recordNearbyNetworks(client, fetched, networks);

		const again = await boxOfPublicKey(client, box.publicKey);
		if (!again) throw new Error('a claimed box is found by its public key');
		expect(pendingWifiChangeOf(again)).toBeNull();

		expect(await wifiChangeStatusFor(client, companyID, box.publicKey)).toMatchObject({ nearbyNetworks: networks });
	});

	test('a Wi-Fi change waits for a connected box', async () => {
		const empty = await aCompany('wifi-empty');
		const boxKey = 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM';
		const sealed = { version: 1 as const, recipient: boxKey, enc: boxKey, ciphertext: 'c2VhbGVk' };

		await expect(requestWifiChange(client, empty.companyID, boxKey, crypto.randomUUID(), sealed)).rejects.toBeInstanceOf(BoxRefused);
	});

	test('a box that belongs to no company has no Wi-Fi change to fetch', async () => {
		const box = await aBox();
		await announceBox(client, { publicKey: box.publicKey, encryptionKey: box.encryptionKey, publicAddress: officeAddress, wantsPairingCode: false });

		expect(await boxOfPublicKey(client, box.publicKey)).toBeNull();
	});
});

describe('setting the admin password of a connected box', () => {
	async function aClaimedBoxWithAnAdminAccount() {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const claimed = await boxOfPublicKey(client, box.publicKey);
		if (!claimed) throw new Error('a claimed box is found by its public key');
		await noteAdminAccount(client, claimed);
		return box;
	}

	test('an administrator sets a password, the box fetches it, and reports it applied', async () => {
		const box = await aClaimedBoxWithAnAdminAccount();
		const settingID = crypto.randomUUID();
		const sealed = { version: 1 as const, recipient: box.encryptionKey, enc: box.encryptionKey, ciphertext: 'c2VhbGVk' };

		expect(await requestAdminPassword(client, companyID, box.publicKey, settingID, sealed)).toEqual({ hasAdminAccount: true, pendingSettingID: settingID, outcome: null });
		expect((await connectedBox(companyID, box.publicKey))?.hasAdminAccount).toBe(true);

		const fetched = await boxOfPublicKey(client, box.publicKey);
		if (!fetched) throw new Error('a claimed box is found by its public key');
		expect(pendingAdminPasswordOf(fetched)).toEqual({ settingID, sealed });

		await reportAdminPasswordOutcome(client, fetched, settingID, 'applied');

		expect(await adminPasswordStatusFor(client, companyID, box.publicKey)).toMatchObject({
			hasAdminAccount: true,
			pendingSettingID: null,
			outcome: { settingID, result: 'applied' }
		});
		const cleared = await boxOfPublicKey(client, box.publicKey);
		if (!cleared) throw new Error('a claimed box is found by its public key');
		expect(pendingAdminPasswordOf(cleared)).toBeNull();
	});

	test('an outcome naming a setting that is not pending is refused, and clears nothing', async () => {
		const box = await aClaimedBoxWithAnAdminAccount();
		const settingID = crypto.randomUUID();
		const sealed = { version: 1 as const, recipient: box.encryptionKey, enc: box.encryptionKey, ciphertext: 'c2VhbGVk' };
		await requestAdminPassword(client, companyID, box.publicKey, settingID, sealed);
		const current = await boxOfPublicKey(client, box.publicKey);
		if (!current) throw new Error('a claimed box is found by its public key');

		await expect(reportAdminPasswordOutcome(client, current, crypto.randomUUID(), 'failed')).rejects.toBeInstanceOf(BoxRefused);

		expect(await adminPasswordStatusFor(client, companyID, box.publicKey)).toMatchObject({ pendingSettingID: settingID, outcome: null });
	});

	test('a box that never asked for an admin password has no admin account to set one for', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const sealed = { version: 1 as const, recipient: box.encryptionKey, enc: box.encryptionKey, ciphertext: 'c2VhbGVk' };

		expect((await connectedBox(companyID, box.publicKey))?.hasAdminAccount).toBe(false);
		await expect(requestAdminPassword(client, companyID, box.publicKey, crypto.randomUUID(), sealed)).rejects.toBeInstanceOf(BoxRefused);
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

	test('a computer the company replaced can neither call its routes nor touch its record, though its session has not expired', async () => {
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

	test('a session issued since the record began asking which computer holds it must name one', async () => {
		const box = await aBox();
		await claimWithCode(companyID, box.publicKey, await announcedCode(box));
		const named = (await boxSessionFor(credentials, box.publicKey, environment, appURL))?.session.accessToken ?? '';
		const unnamed = await recordTokenFor(signingKey, projectURL, {
			userID: decodeJwt(named).sub ?? '',
			email: hostAddressOf(companyID),
			appMetadata: { company_id: companyID }
		});

		expect(await companyOfHostSession(credentials, named)).toBe(companyID);
		expect(await companyOfHostSession(credentials, unnamed.accessToken)).toBeNull();
		await expect(callingAgent(requestBearing(unnamed.accessToken), environment)).rejects.toMatchObject({ status: 403 });
		const planted = await asMember({ projectURL, publishableKey }, unnamed.accessToken)
			.from('contact')
			.insert({ company_id: companyID, name: `Planted by a session naming no computer ${stamp}` });
		expect(planted.error?.code).toBe('42501');
	});

	test('a computer the administrator disconnected is refused at once', async () => {
		const disconnecting = await aCompany('disconnecting');
		const box = await aBox();
		await claimWithCode(disconnecting.companyID, box.publicKey, await announcedCode(box));
		const held = (await boxSessionFor(credentials, box.publicKey, environment, appURL))?.session.accessToken ?? '';
		expect(await companyOfHostSession(credentials, held)).toBe(disconnecting.companyID);

		await releaseBox(client, disconnecting.companyID, box.publicKey);

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
