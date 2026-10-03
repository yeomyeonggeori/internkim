import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { addMember, controlPlane, issueAgentKey, provisionCompany } from '../../src/lib/server/control-plane';
import { messengerIdentityCredentialKind } from '../../src/lib/server/public-api/catalog/credential';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: keepCredentialAsHost } = await import('../../src/routes/api/agent/messenger-credential/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const stamp = Date.now();
const environment = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};
const keyOfTheReplacedBox = `old-${stamp}`.padEnd(64, '0');
const keyOfTheNewBox = `new-${stamp}`.padEnd(64, '1');

let companyID = '';
let memberID = '';
let agentKey = '';

async function keepAsHost(body: Record<string, unknown>): Promise<Response> {
	const request = new Request('https://intern.example.test/api/agent/messenger-credential', {
		method: 'POST',
		headers: { authorization: `Bearer ${agentKey}`, 'content-type': 'application/json' },
		body: JSON.stringify(body)
	});
	return keepCredentialAsHost({ request, platform: { env: environment } } as unknown as Parameters<
		typeof keepCredentialAsHost
	>[0]);
}

async function messengerAccountsOfMember(): Promise<Record<string, string> | null> {
	const { data, error } = await client
		.from('member')
		.select('messenger')
		.eq('id', memberID)
		.single<{ messenger: Record<string, string> | null }>();
	if (error) throw new Error(error.message);
	return data.messenger;
}

async function issuedExternalIDOfMember(): Promise<string | null> {
	const { data, error } = await client
		.from('credential')
		.select('external_id')
		.eq('member_id', memberID)
		.eq('kind', messengerIdentityCredentialKind)
		.single<{ external_id: string | null }>();
	if (error) throw new Error(error.message);
	return data.external_id;
}

beforeAll(async () => {
	const slug = `messenger-credential-${stamp}`;
	const provisioned = await provisionCompany(
		client,
		{ name: 'Messenger credential', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	memberID = await addMember(client, companyID, `${slug}-member@example.test`);
	agentKey = (await issueAgentKey(client, companyID, 'messenger-credential')).apiKey;
}, networkHookTimeout);

afterAll(async () => {
	await client.from('credential').delete().eq('member_id', memberID);
	await client.from('company').delete().eq('id', companyID);
}, networkHookTimeout);

describe('a box that writes a member messenger key', () => {
	test('moves the member messenger account to the new key along with the credential', async () => {
		const replaced = await keepAsHost({
			memberID,
			kind: messengerIdentityCredentialKind,
			externalID: keyOfTheReplacedBox,
			secret: 'secret-of-the-replaced-box'
		});
		expect(replaced.status).toBe(200);

		const renewed = await keepAsHost({
			memberID,
			kind: messengerIdentityCredentialKind,
			externalID: keyOfTheNewBox,
			secret: 'secret-of-the-new-box'
		});
		expect(renewed.status).toBe(200);

		expect(await issuedExternalIDOfMember()).toBe(keyOfTheNewBox);
		expect((await messengerAccountsOfMember())?.buzz).toBe(keyOfTheNewBox);
	});

	test('refuses a messenger key that names no account', async () => {
		const refusal = await keepAsHost({
			memberID,
			kind: messengerIdentityCredentialKind,
			externalID: '',
			secret: 'secret-without-an-account'
		}).catch((thrown: unknown) => thrown);

		expect(isHttpError(refusal, 400)).toBe(true);
		expect(await issuedExternalIDOfMember()).toBe(keyOfTheNewBox);
	});
});
