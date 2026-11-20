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

const { POST: disconnect } = await import('../../src/routes/api/box/disconnect/+server');

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
		{ name: `Disconnect route ${label}`, slug: `disconnect-route-${label}-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`disconnect-route-${label}-${stamp}-admin@example.test`
	);
	companyIDs.push(provisioned.companyID);
	const issued = await issueAgentKey(client, provisioned.companyID, companyComputerName);
	return { companyID: provisioned.companyID, connectionKey: issued.apiKey };
}

async function aBoxKeptBy(companyID: string): Promise<Box> {
	const kept = await aBox();
	const { error } = await client.rpc('keep_claimed_box', {
		claiming_company: companyID,
		box_key: kept.publicKey,
		box_encryption_key: kept.encryptionKey,
		box_host_name: null,
		is_replacing: false
	});
	if (error) throw new Error(error.message);
	return kept;
}

async function boxesOf(companyID: string): Promise<string[]> {
	const { data, error } = await client.from('box').select('public_key').eq('company_id', companyID);
	if (error) throw new Error(error.message);
	return data.map((box) => box.public_key);
}

async function statusOfDisconnection(assertion: string, body: Record<string, unknown>): Promise<number> {
	const request = new Request('https://intern.example.test/api/box/disconnect', {
		method: 'POST',
		headers: { Authorization: `Bearer ${assertion}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	try {
		const response = await disconnect({ request, url: new URL(request.url), platform: undefined } as unknown as Parameters<typeof disconnect>[0]);
		return response.status;
	} catch (thrown) {
		const refusal = thrown as { status?: number };
		if (refusal.status === undefined) throw thrown;
		return refusal.status;
	}
}

describe('a computer reset by whoever holds it', () => {
	test('disconnects its own box, and the company keeps its other boxes', async () => {
		const company = await aCompanyOnAConnectionFile('box');
		const reset = await aBoxKeptBy(company.companyID);
		const staying = await aBoxKeptBy(company.companyID);

		expect(await statusOfDisconnection(await reset.sign(), {})).toBe(204);

		expect(await boxesOf(company.companyID)).toEqual([staying.publicKey]);
	});

	test('on its connection file gives back the file key, and the box beside it stays', async () => {
		const company = await aCompanyOnAConnectionFile('file');
		const beside = await aBoxKeptBy(company.companyID);

		expect(await statusOfDisconnection(await (await aBox()).sign(), { connectionKey: company.connectionKey })).toBe(204);

		await expect(sessionForHost(credentials, company.connectionKey)).rejects.toThrow('that key belongs to no agent');
		expect(await boxesOf(company.companyID)).toEqual([beside.publicKey]);
	});

	test('is answered again when the plane already let it go', async () => {
		const company = await aCompanyOnAConnectionFile('again');
		const computer = await aBox();

		expect(await statusOfDisconnection(await computer.sign(), { connectionKey: company.connectionKey })).toBe(204);
		expect(await statusOfDisconnection(await computer.sign(), { connectionKey: company.connectionKey })).toBe(204);
	});

	test('is refused without a signature from its box key', async () => {
		const company = await aCompanyOnAConnectionFile('unsigned');

		expect(await statusOfDisconnection('not-an-assertion', { connectionKey: company.connectionKey })).toBe(401);

		expect((await sessionForHost(credentials, company.connectionKey)).companyID).toBe(company.companyID);
	});
});
