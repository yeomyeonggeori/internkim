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
const slug = `persona-tools-${Date.now()}`;
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
		{ name: 'Persona Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
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

const profile = { schemaVersion: 1, callMe: '샘플 님', morningBriefing: { enabled: false, time: '08:00' } };
const identity = { schemaVersion: 1, names: ['김인턴', 'Intern Kim'], handle: 'internkim' };

function succeeded(toolName: string, result: Record<string, unknown>) {
	return { toolName, outcome: 'succeeded', result };
}

describe('the requester’s persona profile', () => {
	test('is changed as the member who asked, with the call’s fields and nothing naming a person', async () => {
		const carried = gatewayAnswering({ status: 200, body: succeeded('persona_profile_update', profile) });
		const answered = await invoke('persona_profile_update', memberSession, { morningBriefing: { enabled: false } });

		expect(answered.status).toBe(200);
		expect(carried.map((call) => [call.body.path, call.body.requester, call.body.payload])).toEqual([
			['/tools/persona_profile_update/invoke', memberEmail, { input: { morningBriefing: { enabled: false } } }]
		]);
	});

	test('is refused here, before anything is carried, when the call tries to name somebody else', async () => {
		const carried = gatewayAnswering({ status: 200, body: succeeded('persona_profile_update', profile) });
		const answered = await invoke('persona_profile_update', memberSession, { personID: 'somebody-else', callMe: '샘플' });

		expect(answered.status).toBe(400);
		expect(carried).toEqual([]);
	});

	test('is read with only the right to read', async () => {
		const carried = gatewayAnswering({ status: 200, body: succeeded('persona_profile_get', profile) });
		const answered = await invoke('persona_profile_get', readingCredential, {});

		expect(answered.status).toBe(200);
		expect(carried.map((call) => [call.body.path, call.body.requester])).toEqual([['/tools/persona_profile_get/invoke', administratorEmail]]);
	});
});

describe('the agent’s persona identity', () => {
	test('is read by any member', async () => {
		const carried = gatewayAnswering({ status: 200, body: succeeded('persona_identity_get', identity) });
		const answered = await invoke('persona_identity_get', memberSession, {});

		expect(answered.status).toBe(200);
		expect(carried.map((call) => call.body.requester)).toEqual([memberEmail]);
	});

	test('changed by a member who is not an administrator is carried, and comes back refused in admind’s words', async () => {
		const carried = gatewayAnswering({
			status: 200,
			body: { toolName: 'persona_identity_update', outcome: 'failed', errorCode: 'access_denied', failureStage: 'authorization', message: 'only an administrator changes the agent identity', retryable: false, result: {} }
		});
		const answered = await invoke('persona_identity_update', memberSession, { names: ['이샘플'] });

		expect(carried.map((call) => call.body.requester)).toEqual([memberEmail]);
		const refusal = refusalIn(answered, 'persona_identity_update');
		expect([refusal.errorCode, refusal.message]).toEqual(['access_denied', 'only an administrator changes the agent identity']);
	});

	test('is refused here for a credential that may only read', async () => {
		const carried = gatewayAnswering({ status: 200, body: succeeded('persona_identity_update', identity) });
		const answered = await invoke('persona_identity_update', readingCredential, { names: ['이샘플'] });

		expect(answered.status).toBe(403);
		expect(carried).toEqual([]);
	});
});
