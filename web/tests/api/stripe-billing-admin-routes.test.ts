import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { addMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { GET: readInvoices } = await import('../../src/routes/api/stripe/invoices/+server');
const { POST: openPortal } = await import('../../src/routes/api/stripe/portal/+server');

const networkHookTimeout = 60_000;
const plane = {
	SUPABASE_URL: projectURL,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
};
const configured = { ...plane, STRIPE_SECRET_KEY: 'sk_test_unused' };
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `billing-admin-routes-${Date.now()}`;

let companyID = '';
let adminToken = '';
let colleagueToken = '';

type Handler = (event: never) => Response | Promise<Response>;
type RouteAnswer = { status: number; body: unknown };

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
	const provisioned = await provisionCompany(
		client,
		{ name: 'Billing Admin Sample', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
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

function asking(path: string, method: string, authorization: string): Request {
	return new Request(`https://intern.example.test/api/stripe/${path}`, { method, headers: { authorization } });
}

async function answerOf(handler: Handler, request: Request, environment: Record<string, string> = configured): Promise<RouteAnswer> {
	try {
		const response = await handler({ request, platform: { env: environment }, url: new URL(request.url) } as never);
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		if (isHttpError(thrown)) return { status: thrown.status, body: thrown.body };
		throw thrown;
	}
}

describe('the invoices and the payment portal', () => {
	test('answer 503 while Stripe billing is not configured', async () => {
		expect((await answerOf(readInvoices, asking('invoices', 'GET', `Bearer ${adminToken}`), plane)).status).toBe(503);
		expect((await answerOf(openPortal, asking('portal', 'POST', `Bearer ${adminToken}`), plane)).status).toBe(503);
	});

	test('ask for a sign-in', async () => {
		expect((await answerOf(readInvoices, asking('invoices', 'GET', ''))).status).toBe(401);
		expect((await answerOf(openPortal, asking('portal', 'POST', ''))).status).toBe(401);
	});

	test('refuse a member who is not an admin', async () => {
		expect((await answerOf(readInvoices, asking('invoices', 'GET', `Bearer ${colleagueToken}`))).status).toBe(403);
		expect((await answerOf(openPortal, asking('portal', 'POST', `Bearer ${colleagueToken}`))).status).toBe(403);
	});

	test('show an admin of a company that never paid no invoices and no portal', async () => {
		const invoices = await answerOf(readInvoices, asking('invoices', 'GET', `Bearer ${adminToken}`));
		expect(invoices).toEqual({ status: 200, body: { invoices: [] } });
		expect((await answerOf(openPortal, asking('portal', 'POST', `Bearer ${adminToken}`))).status).toBe(409);
	});
});
