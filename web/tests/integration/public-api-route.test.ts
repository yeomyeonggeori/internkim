import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import {
	addMember,
	controlPlane,
	issuePersonalAccessToken,
	provisionCompany,
	sessionForMember,
} from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';
import { createOpenApiDocument } from '../../../docs/web/app/lib/openapi';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { GET: listTokens } = await import('../../src/routes/api/v1/tokens/+server');
const { POST: mintToken, DELETE: revokeToken } = await import('../../src/routes/api/v1/token/+server');
const { fallback: reachTheAPI } = await import('../../src/routes/api/v1/[...path]/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `public-api-${Date.now()}`;

let companyID = '';
let memberID = '';
let holdersToken = '';
let readersToken = '';
let sessionToken = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Public API Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	memberID = await addMember(client, companyID, `${slug}-holder@example.test`);

	const { data: account } = await client.auth.admin.createUser({
		email: `${slug}-holder@example.test`,
		email_confirm: true
	});
	await client.from('member').update({ user_id: account.user!.id }).eq('id', memberID);

	holdersToken = await issuePersonalAccessToken(client, memberID, 'holder', 'delete');
	readersToken = await issuePersonalAccessToken(client, memberID, 'reader', 'read');
	sessionToken = (await sessionForMember({ projectURL, serviceRoleKey }, memberID)).accessToken;
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

function asking(path: string, token: string | null, options: RequestInit = {}): Request {
	const headers = token ? { Authorization: `Bearer ${token}`, ...options.headers } : options.headers;
	return new Request(`https://space.intern.kim/api/v1${path}`, { ...options, headers });
}

type RouteAnswer = { status: number; body: unknown };

async function answerOf(handler: () => Promise<Response>): Promise<RouteAnswer> {
	try {
		const response = await handler();
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: unknown };
		if (typeof refusal.status !== 'number') throw thrown;
		return { status: refusal.status, body: refusal.body };
	}
}

function reach(path: string, token: string | null, options: RequestInit = {}): Promise<RouteAnswer> {
	const request = asking(path, token, options);
	const url = new URL(request.url);
	return answerOf(() =>
		Promise.resolve(
			reachTheAPI({
				request,
				url,
				params: { path: path.replace(/^\//, '').split('?')[0] },
				platform: undefined
			} as unknown as Parameters<typeof reachTheAPI>[0])
		)
	);
}

function tokens(token: string): Promise<RouteAnswer> {
	const request = asking('/tokens', token);
	return answerOf(() =>
		Promise.resolve(listTokens({ request, platform: undefined } as unknown as Parameters<typeof listTokens>[0]))
	);
}

function mint(token: string, body: unknown): Promise<RouteAnswer> {
	const request = asking('/token', token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	return answerOf(() =>
		Promise.resolve(mintToken({ request, platform: undefined } as unknown as Parameters<typeof mintToken>[0]))
	);
}

function revoke(token: string, name: string): Promise<RouteAnswer> {
	const request = asking(`/token?name=${encodeURIComponent(name)}`, token, { method: 'DELETE' });
	return answerOf(() =>
		Promise.resolve(
			revokeToken({
				request,
				url: new URL(request.url),
				platform: undefined
			} as unknown as Parameters<typeof revokeToken>[0])
		)
	);
}

describe('a call that names nobody', () => {
	test('is refused without a bearer, and with one nobody holds', async () => {
		expect((await reach('/tools', null)).status).toBe(401);
		expect((await reach('/tools', 'ik_nobody')).status).toBe(401);
	});
});

describe('the catalog', () => {
	test('is answered here, and a reading token sees fewer tools than a deleting one', async () => {
		const held = await reach('/tools', holdersToken);
		const read = await reach('/tools', readersToken);

		const heldTools = (held.body as { tools: unknown[] }).tools;
		const readTools = (read.body as { tools: unknown[] }).tools;
		expect(held.status).toBe(200);
		expect(heldTools.length > 0).toBe(true);
		expect(readTools.length < heldTools.length).toBe(true);
	});

	test('answers one tool by name, and refuses a name it does not carry', async () => {
		expect((await reach('/tools/task_list', holdersToken)).status).toBe(200);
		expect((await reach('/tools/no_such_tool', holdersToken)).status).toBe(404);
	});
});

describe('a signed-in session', () => {
	test('reaches the same catalog its owner reaches, with no key issued', async () => {
		const answered = await reach('/tools', sessionToken);
		expect(answered.status).toBe(200);
		expect((answered.body as { tools: unknown[] }).tools.length > 0).toBe(true);
	});

	test('makes and revokes a token without holding one', async () => {
		const made = await mint(sessionToken, { name: 'from-a-session', permission: 'read' });
		expect(made.status).toBe(200);
		expect((made.body as { token: string }).token.startsWith('ik_')).toBe(true);
		expect((await revoke(sessionToken, 'from-a-session')).status).toBe(200);
	});
});

describe('the tokens a member holds', () => {
	test('are made, listed and revoked, and a token never outranks its maker', async () => {
		const made = (await mint(holdersToken, { name: 'made', permission: 'read' })).body as {
			name: string;
			permission: string;
			token: string;
		};
		expect(made.permission).toBe('read');

		const unnamed = (await mint(holdersToken, {})).body as { name: string };
		expect(unnamed.name).toBe('pat-1');

		const listed = (await tokens(holdersToken)).body as { tokens: { name: string }[] };
		const names = listed.tokens.map((token) => token.name);
		expect(names).toContain('made');
		expect(names).toContain('pat-1');

		expect((await mint(readersToken, { name: 'escalation', permission: 'delete' })).status).toBe(403);
		expect((await mint(holdersToken, { name: 'holder' })).status).toBe(409);

		expect((await revoke(holdersToken, 'holder')).status).toBe(409);
		expect((await revoke(holdersToken, 'never-existed')).status).toBe(404);
		expect((await revoke(holdersToken, made.name)).status).toBe(200);
		expect((await reach('/tools', made.token)).status).toBe(401);
	});
});

describe('a tool whose rows live in the record', () => {
	test('is answered here, without a gateway to any company machine', async () => {
		const answered = await reach('/tools/person_list/invoke', holdersToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: {} })
		});
		expect(answered.status).toBe(200);
		expect((answered.body as { tool: string }).tool).toBe('person_list');
	});

	test('is refused by rung before it runs', async () => {
		const answered = await reach('/tools/task_delete/invoke', readersToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: { taskHint: 'anything' } })
		});
		expect(answered.status).toBe(403);
	});
});

describe('a tool the company machine runs', () => {
	test('is refused with no gateway configured rather than answered here', async () => {
		const answered = await reach('/tools/message_send/invoke', holdersToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: { conversationID: 'c1', message: 'hello' } })
		});
		expect(answered.status).toBe(503);
	});
});

type Operation = { path: string; method: string };
type PathsOfDocument = Record<string, Record<string, unknown> | undefined>;

function documentedOperations(): Operation[] {
	const paths = (createOpenApiDocument('en') as { paths: PathsOfDocument }).paths;
	const operations: Operation[] = [];
	for (const [path, methods] of Object.entries(paths)) {
		for (const method of Object.keys(methods ?? {})) operations.push({ path, method });
	}
	return operations;
}

// A documented path carries {name} where a real call carries a tool. The routes
// only have to recognise the shape, so any name they can resolve will do.
function reachDocumented(operation: Operation, revocableName: string): Promise<RouteAnswer> {
	const method = operation.method.toUpperCase();
	const carriesBody = method === 'POST' || method === 'PUT' || method === 'PATCH';
	const path = operation.path.replace('{name}', 'task_list');
	if (path === '/tokens') return tokens(holdersToken);
	if (path === '/token' && method === 'POST') return mint(holdersToken, {});
	if (path === '/token' && method === 'DELETE') return revoke(holdersToken, revocableName);
	return reach(path, holdersToken, {
		method,
		headers: { 'Content-Type': 'application/json' },
		...(carriesBody ? { body: '{}' } : {})
	});
}

describe('the documented endpoints', () => {
	test('are more than a handful, so an empty document cannot pass this', () => {
		expect(documentedOperations().length > 30).toBe(true);
	});

	test('are all reachable, and none answers as if it carried no token', async () => {
		const wrong: string[] = [];
		for (const [ordinal, operation] of documentedOperations().entries()) {
			const revocableName = `conformance-${ordinal}`;
			await mint(holdersToken, { name: revocableName, permission: 'read' });
			const answered = await reachDocumented(operation, revocableName);
			if ([401, 404, 405].includes(answered.status)) {
				wrong.push(`${operation.method.toUpperCase()} ${operation.path} -> ${answered.status}`);
			}
		}
		expect(wrong).toEqual([]);
	}, networkHookTimeout);

	test('cover every path these routes answer', () => {
		const documented = new Set(
			documentedOperations().map((operation) => `${operation.method} ${operation.path}`)
		);
		const served = [
			'get /tools',
			'get /tools/{name}',
			'post /tools/{name}/invoke',
			'get /tokens',
			'post /token',
			'delete /token',
			'post /files',
			'post /agent/messages',
			'get /agent/replies'
		];
		expect(served.filter((operation) => !documented.has(operation))).toEqual([]);
	});
});
