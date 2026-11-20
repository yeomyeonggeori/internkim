import { afterAll, describe, expect, mock, test } from 'bun:test';
import { SignJWT, exportJWK, generateKeyPair } from 'jose';
import { x25519 } from '@noble/curves/ed25519.js';
import { base64URLOf } from '../../src/lib/company/seal-to-box';
import { companyComputerName } from '../../src/lib/company/host-setup';
import { boxAssertionAudience } from '../../src/lib/server/box';
import { controlPlane, issueAgentKey, provisionCompany, sessionForHost } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: {
		SUPABASE_URL: projectURL,
		SUPABASE_SECRET_KEY: serviceRoleKey,
		SUPABASE_PUBLISHABLE_KEY: publishableKey,
		SUPABASE_JWT_SIGNING_KEY: signingKey
	}
}));

const { POST: claim } = await import('../../src/routes/api/box/claim/+server');

const networkHookTimeout = 60_000;
const credentials = { projectURL, publishableKey, serviceRoleKey, signingKey };
const client = controlPlane(credentials);
const stamp = Date.now();
const companyIDs: string[] = [];

type Box = { publicKey: string; encryptionKey: string; sign: () => Promise<string> };

afterAll(async () => {
	if (companyIDs.length > 0) await client.from('company').delete().in('id', companyIDs);
}, networkHookTimeout);

async function aBox(): Promise<Box> {
	const { privateKey, publicKey } = await generateKeyPair('EdDSA', { crv: 'Ed25519' });
	const { x } = await exportJWK(publicKey);
	if (!x) throw new Error('an Ed25519 key exports its public half as x');
	const encryptionKey = base64URLOf(x25519.getPublicKey(x25519.utils.randomSecretKey()));
	const sign = () => {
		const now = Math.floor(Date.now() / 1000);
		return new SignJWT({}).setProtectedHeader({ alg: 'EdDSA', typ: 'JWT' }).setIssuer(x).setAudience(boxAssertionAudience).setIssuedAt(now).setExpirationTime(now + 60).sign(privateKey);
	};
	return { publicKey: x, encryptionKey, sign };
}

async function aCompanyOnAConnectionFile(label: string) {
	const provisioned = await provisionCompany(
		client,
		{ name: `Claim route ${label}`, slug: `claim-route-${label}-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`claim-route-${label}-${stamp}-admin@example.test`
	);
	companyIDs.push(provisioned.companyID);
	const issued = await issueAgentKey(client, provisioned.companyID, companyComputerName);
	return { companyID: provisioned.companyID, connectionKey: issued.apiKey };
}

async function aStandbyBoxOf(companyID: string): Promise<Box> {
	const standby = await aBox();
	const { error } = await client.rpc('keep_claimed_box', {
		claiming_company: companyID,
		box_key: standby.publicKey,
		box_encryption_key: standby.encryptionKey,
		box_host_name: null,
		is_replacing: false
	});
	if (error) throw new Error(error.message);
	return standby;
}

async function boxesOf(companyID: string): Promise<string[]> {
	const { data, error } = await client.from('box').select('public_key, role').eq('company_id', companyID);
	if (error) throw new Error(error.message);
	return data.map((box) => `${box.public_key}:${box.role}`);
}

async function statusOfClaim(box: Box, body: Record<string, unknown>): Promise<number> {
	const request = new Request('https://intern.example.test/api/box/claim', {
		method: 'POST',
		headers: { Authorization: `Bearer ${await box.sign()}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ encryptionKey: box.encryptionKey, ...body })
	});
	try {
		const response = await claim({ request, url: new URL(request.url), platform: undefined } as unknown as Parameters<typeof claim>[0]);
		return response.status;
	} catch (thrown) {
		const refusal = thrown as { status?: number };
		if (refusal.status === undefined) throw thrown;
		return refusal.status;
	}
}

describe('a computer claiming itself with its connection file', () => {
	test('is refused beside a box that stands by when it does not replace, and nothing changes', async () => {
		const served = await aCompanyOnAConnectionFile('beside');
		const standby = await aStandbyBoxOf(served.companyID);

		expect(await statusOfClaim(await aBox(), { connectionKey: served.connectionKey, replacesOtherComputers: false })).toBe(409);

		expect((await sessionForHost(credentials, served.connectionKey)).companyID).toBe(served.companyID);
		expect(await boxesOf(served.companyID)).toEqual([`${standby.publicKey}:standby`]);
	});

	test('claims without replacing when it is the company\'s only computer', async () => {
		const served = await aCompanyOnAConnectionFile('alone');
		const box = await aBox();

		expect(await statusOfClaim(box, { connectionKey: served.connectionKey, replacesOtherComputers: false })).toBe(200);

		expect(await boxesOf(served.companyID)).toEqual([`${box.publicKey}:service`]);
	});

	test('replaces the other boxes when the claim does not say, as installers already in the field ask', async () => {
		const served = await aCompanyOnAConnectionFile('install');
		await aStandbyBoxOf(served.companyID);
		const box = await aBox();

		expect(await statusOfClaim(box, { connectionKey: served.connectionKey })).toBe(200);

		expect(await boxesOf(served.companyID)).toEqual([`${box.publicKey}:service`]);
	});
});
