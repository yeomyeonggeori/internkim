import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { SQL } from 'bun';
import { isHttpError } from '@sveltejs/kit';
import { localDatabaseURL } from '../../../supabase/scripts/local-database-url';
import { addMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { POST: chargeTopUp } = await import('../../src/routes/api/stripe/top-up/+server');
const settingsRoute = await import('../../src/routes/api/stripe/top-up/settings/+server');

const networkHookTimeout = 60_000;
const plane = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};
const configured = { ...plane, STRIPE_SECRET_KEY: 'sk_test_unused' };
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `top-up-route-${Date.now()}`;

let companyID = '';
let adminToken = '';
let colleagueToken = '';
let jobSecret = '';

type RouteAnswer = { status: number; body: unknown };
type Handler = (event: never) => Response | Promise<Response>;

async function seatedMember(email: string, isAdmin: boolean): Promise<string> {
	const memberID = await addMember(client, companyID, email, { isAdmin });
	const { data: account, error } = await client.auth.admin.createUser({ email, email_confirm: true });
	if (error) throw new Error(`the test could not create ${email}: ${error.message}`);
	await client.from('member').update({ user_id: account.user.id, status: 'active' }).eq('id', memberID);
	return memberID;
}

async function signedInAs(memberID: string): Promise<string> {
	return (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID)).accessToken;
}

beforeAll(async () => {
	const database = new SQL(localDatabaseURL);
	const [row] = await database`select decrypted_secret from vault.decrypted_secrets where name = 'scheduled_job_secret'`;
	await database.close();
	jobSecret = row.decrypted_secret;

	const provisioned = await provisionCompany(
		client,
		{ name: 'Top-up Route Sample', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-founder@example.test`
	);
	companyID = provisioned.companyID;
	adminToken = await signedInAs(await seatedMember(`${slug}-admin@example.test`, true));
	colleagueToken = await signedInAs(await seatedMember(`${slug}-colleague@example.test`, false));
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

async function answerOf(handler: Handler, request: Request, environment: Record<string, string> = configured): Promise<RouteAnswer> {
	try {
		const response = await handler({ request, platform: { env: environment }, url: new URL(request.url) } as never);
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		if (isHttpError(thrown)) return { status: thrown.status, body: thrown.body };
		throw thrown;
	}
}

function asking(path: string, method: string, authorization: string, body?: unknown): Request {
	return new Request(`https://intern.example.test/api/stripe/${path}`, {
		method,
		headers: { authorization, 'content-type': 'application/json' },
		body: body === undefined ? undefined : JSON.stringify(body)
	});
}

async function pendingTopUpOfTheCompany(): Promise<string> {
	const database = new SQL(localDatabaseURL);
	const [row] = await database`insert into public.company_credit_top_up (company_id) values (${companyID}) returning id`;
	await database.close();
	return row.id;
}

describe('the credit top-up setting', () => {
	test('is set only by a signed-in administrator', async () => {
		const setting = { enabled: true, thresholdCredits: 1500, amount: 10000 };
		expect((await answerOf(settingsRoute.PUT, asking('top-up/settings', 'PUT', '', setting))).status).toBe(401);
		expect((await answerOf(settingsRoute.PUT, asking('top-up/settings', 'PUT', `Bearer ${colleagueToken}`, setting))).status).toBe(403);
		expect((await answerOf(settingsRoute.GET, asking('top-up/settings', 'GET', `Bearer ${colleagueToken}`))).status).toBe(403);
	});

	test('refuses a setting without a positive threshold and amount', async () => {
		const answer = await answerOf(settingsRoute.PUT, asking('top-up/settings', 'PUT', `Bearer ${adminToken}`, {
			enabled: true, thresholdCredits: 0, amount: 10000
		}));
		expect(answer.status).toBe(400);
	});

	test('reads back what the administrator set', async () => {
		const setting = { enabled: true, thresholdCredits: 1500, amount: 30000 };
		expect((await answerOf(settingsRoute.PUT, asking('top-up/settings', 'PUT', `Bearer ${adminToken}`, setting))).status).toBe(200);
		expect(await answerOf(settingsRoute.GET, asking('top-up/settings', 'GET', `Bearer ${adminToken}`))).toEqual({
			status: 200,
			body: { setting, latestTopUp: null }
		});
	});
});

describe('the credit top-up charge', () => {
	test('answers 503 while Stripe billing is not configured', async () => {
		expect((await answerOf(chargeTopUp, asking('top-up', 'POST', `Bearer ${jobSecret}`, { top_up_id: crypto.randomUUID() }), plane)).status).toBe(503);
	});

	test('is asked for only by the scheduled job secret, naming a top-up', async () => {
		expect((await answerOf(chargeTopUp, asking('top-up', 'POST', '', { top_up_id: crypto.randomUUID() }))).status).toBe(401);
		expect((await answerOf(chargeTopUp, asking('top-up', 'POST', 'Bearer not-the-secret', { top_up_id: crypto.randomUUID() }))).status).toBe(403);
		expect((await answerOf(chargeTopUp, asking('top-up', 'POST', `Bearer ${jobSecret}`, { top_up_id: 'nothing' }))).status).toBe(400);
	});

	test('fails a company without a subscription before reaching Stripe, and turns top-ups off', async () => {
		const topUpID = await pendingTopUpOfTheCompany();
		expect(await answerOf(chargeTopUp, asking('top-up', 'POST', `Bearer ${jobSecret}`, { top_up_id: topUpID }))).toEqual({
			status: 200,
			body: { outcome: 'failed' }
		});
		const settings = await answerOf(settingsRoute.GET, asking('top-up/settings', 'GET', `Bearer ${adminToken}`));
		expect(settings.body).toMatchObject({
			setting: { enabled: false },
			latestTopUp: { status: 'failed', failure: 'the company has no Stripe subscription' }
		});
	});

	test('leaves a settled top-up alone', async () => {
		const { data } = await client.from('company_credit_top_up').select('id').eq('company_id', companyID).single<{ id: string }>();
		expect(await answerOf(chargeTopUp, asking('top-up', 'POST', `Bearer ${jobSecret}`, { top_up_id: data?.id }))).toEqual({
			status: 200,
			body: { outcome: 'settled' }
		});
	});
});
