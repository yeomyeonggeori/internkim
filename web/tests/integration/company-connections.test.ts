import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const connectionsRoute = await import('../../src/routes/api/company/connections/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `company-connections-${Date.now()}`;

let companyID = '';
let adminToken = '';
let adminUserID = '';

async function statusOf(body: unknown): Promise<number> {
	const request = new Request('https://space.example.test/api/company/connections', {
		method: 'PUT',
		headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	try {
		const response = await connectionsRoute.PUT({ request, platform: undefined } as unknown as Parameters<typeof connectionsRoute.PUT>[0]);
		return response.status;
	} catch (thrown) {
		if (isHttpError(thrown)) return thrown.status;
		throw thrown;
	}
}

async function keptSettings(): Promise<unknown> {
	const { data } = await client.from('credential').select('settings').eq('company_id', companyID).eq('kind', 'buzz').maybeSingle();
	return data?.settings;
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Company Connections Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	const { data: account } = await client.auth.admin.createUser({ email: `${slug}-admin@example.test`, email_confirm: true });
	adminUserID = account.user?.id ?? '';
	await client.from('member').update({ user_id: adminUserID, status: 'active' }).eq('id', provisioned.adminMemberID);
	adminToken = (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, provisioned.adminMemberID)).accessToken;
}, networkHookTimeout);

afterAll(async () => {
	if (companyID) await client.from('company').delete().eq('id', companyID);
	if (adminUserID) await client.auth.admin.deleteUser(adminUserID);
}, networkHookTimeout);

describe('the settings of a company connection', () => {
	test('keep a port and nothing else', async () => {
		expect(await statusOf({ kind: 'buzz', host: 'buzz.example.test', settings: { port: 8443 } })).toBe(200);
		expect(await keptSettings()).toEqual({ port: 8443 });
	});

	test('refuse a field the connection does not use', async () => {
		expect(await statusOf({ kind: 'buzz', host: 'buzz.example.test', settings: { port: 9000, token: 'x' } })).toBe(400);
		expect(await keptSettings()).toEqual({ port: 8443 });
	});

	test('refuse a port that is not a port', async () => {
		expect(await statusOf({ kind: 'buzz', host: 'buzz.example.test', settings: { port: '8443' } })).toBe(400);
		expect(await statusOf({ kind: 'buzz', host: 'buzz.example.test', settings: { port: 70000 } })).toBe(400);
	});

	test('may be left empty', async () => {
		expect(await statusOf({ kind: 'buzz', host: 'buzz.example.test' })).toBe(200);
		expect(await keptSettings()).toEqual({});
	});
});
