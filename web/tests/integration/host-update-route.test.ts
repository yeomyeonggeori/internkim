import { afterAll, afterEach, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, controlPlane, issuePersonalAccessToken, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { resultOrRefusal, ToolRefused } from '../../src/lib/tool-answer';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { createMockFetch } from '../unit/test-fetch';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { fallback: reachTheAPI } = await import('../../src/routes/api/v1/[...path]/+server');

const networkHookTimeout = 60_000;
const gatewayURL = 'https://gateway.example.test';
const gatewayCredential = 'gateway-test-credential';
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `host-update-${Date.now()}`;
const administratorEmail = `${slug}-admin@example.test`;
const memberEmail = `${slug}-member@example.test`;
const originalFetch = globalThis.fetch;

let companyID = '';
let administratorSession = '';
let memberSession = '';
let readingCredential = '';

type CarriedCall = { url: string; authorization: string; capability: string; body: Record<string, unknown> };
type GatewayAnswer = { status: number; body: unknown };

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Host Update Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		administratorEmail
	);
	companyID = provisioned.companyID;
	const { data: administrator } = await client.auth.admin.createUser({ email: administratorEmail, email_confirm: true });
	await client.from('member').update({ user_id: administrator.user?.id, status: 'active' }).eq('id', provisioned.adminMemberID);

	const memberID = await addMember(client, companyID, memberEmail);
	const { data: member } = await client.auth.admin.createUser({ email: memberEmail, email_confirm: true });
	await client.from('member').update({ user_id: member.user?.id, status: 'active' }).eq('id', memberID);

	administratorSession = (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, provisioned.adminMemberID)).accessToken;
	memberSession = (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID)).accessToken;
	readingCredential = await issuePersonalAccessToken(client, provisioned.adminMemberID, 'reader', 'read');
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

afterEach(() => {
	globalThis.fetch = originalFetch;
});

function gatewayAnswering(answer: GatewayAnswer): CarriedCall[] {
	const carried: CarriedCall[] = [];
	globalThis.fetch = createMockFetch(async (input, options) => {
		const url = input instanceof Request ? input.url : String(input);
		if (!url.startsWith(gatewayURL)) return originalFetch(input, options);
		const call = JSON.parse(String(options?.body)) as { requestID: string; capability: string; body: Record<string, unknown> };
		const headers = new Headers(options?.headers);
		carried.push({ url, authorization: headers.get('Authorization') ?? '', capability: call.capability, body: call.body });
		return Response.json({ requestID: call.requestID, ...answer });
	});
	return carried;
}

async function invoke(name: string, credential: string, input: Record<string, unknown>): Promise<GatewayAnswer> {
	const request = new Request(`https://space.example.test/api/v1/tools/${name}/invoke`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${credential}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
	try {
		const response = await reachTheAPI({
			request,
			url: new URL(request.url),
			params: { path: `tools/${name}/invoke` },
			platform: { env: { GATEWAY_URL: gatewayURL, GATEWAY_ADMIN_TOKEN: gatewayCredential } }
		} as unknown as Parameters<typeof reachTheAPI>[0]);
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: unknown };
		if (typeof refusal.status !== 'number') throw thrown;
		return { status: refusal.status, body: refusal.body };
	}
}

function refusalIn(answered: GatewayAnswer, name: string): ToolRefused {
	try {
		resultOrRefusal(name, answered.status, answered.body);
	} catch (refusal) {
		if (refusal instanceof ToolRefused) return refusal;
		throw refusal;
	}
	throw new Error(`${name} was not refused`);
}

const startedAnswer = {
	toolName: 'host_update',
	outcome: 'succeeded',
	result: { status: 'started', fromVersion: 'v2026.10.01.000000', toVersion: 'v2026.10.02.090000', startedAt: '2026-10-04T05:00:00Z', expectedDowntimeSeconds: 60 }
};

describe('a host update asked for from a signed-in session', () => {
	test('is carried to the company machine as the member who asked, for admind to decide', async () => {
		const carried = gatewayAnswering({ status: 200, body: startedAnswer });
		const answered = await invoke('host_update', administratorSession, { targetVersion: 'v2026.10.02.090000' });

		expect(answered).toEqual({ status: 200, body: startedAnswer });
		expect(carried).toEqual([
			{
				url: `${gatewayURL}/company/${companyID}/call`,
				authorization: `Bearer ${gatewayCredential}`,
				capability: 'person.api.request',
				body: {
					method: 'POST',
					path: '/tools/host_update/invoke',
					query: '',
					permission: 'delete',
					requester: administratorEmail,
					payload: { input: { targetVersion: 'v2026.10.02.090000' } }
				}
			}
		]);
	});

	test('from a member who is not an administrator is still carried, and comes back refused in admind’s words', async () => {
		const carried = gatewayAnswering({
			status: 200,
			body: { toolName: 'host_update', outcome: 'failed', errorCode: 'access_denied', failureStage: 'authorization', message: 'only an administrator of this company can update its host', retryable: false, result: {} }
		});
		const answered = await invoke('host_update', memberSession, {});

		expect(carried.map((call) => call.body.requester)).toEqual([memberEmail]);
		const refusal = refusalIn(answered, 'host_update');
		expect([refusal.errorCode, refusal.message]).toEqual(['access_denied', 'only an administrator of this company can update its host']);
	});

	test('comes back as each refusal admind makes, with its code', async () => {
		for (const errorCode of ['update_method_unsupported', 'channel_not_stable', 'already_installed', 'update_in_progress', 'no_stable_release']) {
			gatewayAnswering({ status: 200, body: { toolName: 'host_update', outcome: 'failed', errorCode, failureStage: 'precondition', message: `refused: ${errorCode}`, retryable: false, result: {} } });
			expect(refusalIn(await invoke('host_update', administratorSession, {}), 'host_update').errorCode).toBe(errorCode);
		}
	});

	test('is refused here, before anything is carried, when the version is not a release tag', async () => {
		const carried = gatewayAnswering({ status: 200, body: startedAnswer });
		const answered = await invoke('host_update', administratorSession, { targetVersion: 'latest' });

		expect(answered.status).toBe(400);
		expect(carried).toEqual([]);
	});

	test('is refused here for a credential that may only read', async () => {
		const carried = gatewayAnswering({ status: 200, body: startedAnswer });
		const answered = await invoke('host_update', readingCredential, {});

		expect(answered.status).toBe(403);
		expect(carried).toEqual([]);
	});
});

describe('reading the host version', () => {
	test('is carried for any member with only the right to read', async () => {
		const carried = gatewayAnswering({ status: 200, body: { toolName: 'host_version_get', outcome: 'succeeded', result: { installedVersion: 'v2026.10.01.000000' } } });
		const answered = await invoke('host_version_get', memberSession, {});

		expect(answered.status).toBe(200);
		expect(carried.map((call) => [call.body.path, call.body.requester])).toEqual([['/tools/host_version_get/invoke', memberEmail]]);
	});

	test('while the host is away comes back as the gateway’s offline answer', async () => {
		gatewayAnswering({ status: 503, body: { error: 'server_offline' } });
		const answered = await invoke('host_version_get', memberSession, {});

		expect(answered).toEqual({ status: 503, body: { error: 'server_offline' } });
		expect(refusalIn(answered, 'host_version_get').status).toBe(503);
	});
});
