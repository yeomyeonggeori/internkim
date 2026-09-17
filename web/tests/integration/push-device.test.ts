import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import {
	addMember,
	controlPlane,
	issuePersonalAccessToken,
	provisionCompany,
	sessionForMember
} from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const pushDeviceRoute = await import('../../src/routes/api/member/push-device/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `push-device-${Date.now()}`;
const subscription = 'https://push.example.test/subscription-1';

let companyID = '';
let sessionToken = '';
let colleaguesSessionToken = '';
let personalAccessToken = '';

type Handler = typeof pushDeviceRoute.GET;

type RouteAnswer = { status: number; body: unknown };

type Reachability = { serverKey: string; isServerKeyVaulted: boolean; hasClaimedDevice: boolean };

async function seatedMember(email: string): Promise<string> {
	const memberID = await addMember(client, companyID, email);
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
		{ name: 'Push Device Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;

	const holderID = await seatedMember(`${slug}-holder@example.test`);
	const colleagueID = await seatedMember(`${slug}-colleague@example.test`);
	sessionToken = await signedInAs(holderID);
	colleaguesSessionToken = await signedInAs(colleagueID);
	personalAccessToken = await issuePersonalAccessToken(client, holderID, 'holder', 'delete');
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

async function answerOf(handler: Handler, request: Request): Promise<RouteAnswer> {
	try {
		const response = await handler({ request, url: new URL(request.url), platform: undefined } as unknown as Parameters<Handler>[0]);
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		if (isHttpError(thrown)) return { status: thrown.status, body: thrown.body };
		throw thrown;
	}
}

function asking(token: string, method: string, query = '', body?: Record<string, unknown>): Request {
	return new Request(`https://space.example.test/api/member/push-device${query}`, {
		method,
		headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
		body: body ? JSON.stringify(body) : undefined
	});
}

function isReachability(body: unknown): body is Reachability {
	return typeof body === 'object' && body !== null && 'serverKey' in body && 'hasClaimedDevice' in body;
}

function reachabilityOf(answer: RouteAnswer): Reachability {
	expect(answer.status).toBe(200);
	if (!isReachability(answer.body)) throw new Error('the push device answered without its reachability');
	return answer.body;
}

function refusalOf(answer: RouteAnswer): string {
	const body = answer.body;
	if (typeof body !== 'object' || body === null || !('message' in body)) return '';
	return typeof body.message === 'string' ? body.message : '';
}

function reachability(token: string): Promise<RouteAnswer> {
	return answerOf(pushDeviceRoute.GET, asking(token, 'GET'));
}

function claim(token: string, body: Record<string, unknown>): Promise<RouteAnswer> {
	return answerOf(pushDeviceRoute.PUT, asking(token, 'PUT', '', body));
}

function release(token: string, query: string): Promise<RouteAnswer> {
	return answerOf(pushDeviceRoute.DELETE, asking(token, 'DELETE', query));
}

describe('the device push is delivered to', () => {
	test('is claimed, read back, and released by the signed-in member', async () => {
		expect(reachabilityOf(await reachability(sessionToken)).hasClaimedDevice).toBe(false);

		const claimed = reachabilityOf(
			await claim(sessionToken, {
				endpoint: subscription,
				publicKey: 'a-public-key',
				authenticationSecret: 'an-authentication-secret'
			})
		);
		expect(claimed.hasClaimedDevice).toBe(true);
		expect(reachabilityOf(await reachability(colleaguesSessionToken)).hasClaimedDevice).toBe(false);

		const released = reachabilityOf(await release(sessionToken, `?endpoint=${encodeURIComponent(subscription)}`));
		expect(released.hasClaimedDevice).toBe(false);
	});

	test('is an app token that carries no keys', async () => {
		const claimed = reachabilityOf(await claim(sessionToken, { endpoint: 'a-device-token', kind: 'apns' }));
		expect(claimed.hasClaimedDevice).toBe(true);

		const released = reachabilityOf(await release(sessionToken, '?endpoint=a-device-token&kind=apns'));
		expect(released.hasClaimedDevice).toBe(false);
	});

	test('is refused when a subscription carries no keys to encrypt to', async () => {
		const refused = await claim(sessionToken, { endpoint: subscription });

		expect(refused.status).toBe(400);
		expect(refusalOf(refused)).toContain('keys');
	});

	test('answers no server key while this company has none in the vault', async () => {
		const answered = reachabilityOf(await reachability(sessionToken));

		expect(answered.serverKey).toBe('');
		expect(answered.isServerKeyVaulted).toBe(false);
	});

	test('is claimed only from a signed-in app, never with a personal access token', async () => {
		const read = await reachability(personalAccessToken);
		const claimed = await claim(personalAccessToken, { endpoint: 'a-device-token', kind: 'apns' });

		expect(read.status).toBe(403);
		expect(claimed.status).toBe(403);
		expect(reachabilityOf(await reachability(sessionToken)).hasClaimedDevice).toBe(false);
	});
});
