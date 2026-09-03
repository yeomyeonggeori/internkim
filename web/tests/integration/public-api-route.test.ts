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
let departedToken = '';
let sessionToken = '';
let administratorsToken = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Public API Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;

	const { data: administrator } = await client.auth.admin.createUser({
		email: `${slug}-admin@example.test`,
		email_confirm: true
	});
	await client
		.from('member')
		.update({ user_id: administrator.user!.id, status: 'active' })
		.eq('id', provisioned.adminMemberID);
	administratorsToken = await issuePersonalAccessToken(client, provisioned.adminMemberID, 'administrator', 'delete');

	memberID = await addMember(client, companyID, `${slug}-holder@example.test`);

	const { data: account } = await client.auth.admin.createUser({
		email: `${slug}-holder@example.test`,
		email_confirm: true
	});
	await client.from('member').update({ user_id: account.user!.id }).eq('id', memberID);

	holdersToken = await issuePersonalAccessToken(client, memberID, 'holder', 'delete');
	readersToken = await issuePersonalAccessToken(client, memberID, 'reader', 'read');

	const departedID = await addMember(client, companyID, `${slug}-departed@example.test`);
	departedToken = await issuePersonalAccessToken(client, departedID, 'departed', 'delete');
	await client.from('member').update({ status: 'departed' }).eq('id', departedID);

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
	return new Request(`https://space.example.test/api/v1${path}`, { ...options, headers });
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

function invoke(name: string, token: string, input: unknown): Promise<RouteAnswer> {
	return reach(`/tools/${name}/invoke`, token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
}

function preview(name: string, token: string, input: unknown): Promise<RouteAnswer> {
	return reach(`/tools/${name}/target`, token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
}

function messageOf(answered: RouteAnswer): string {
	return (answered.body as { message?: string; error?: string }).message ?? '';
}

describe('a call that names nobody', () => {
	test('is refused without a bearer, and with one nobody holds', async () => {
		expect((await reach('/tools', null)).status).toBe(401);
		expect((await reach('/tools', 'ik_nobody')).status).toBe(401);
	});
});

describe("a token whose owner has left", () => {
	test('is refused in the words the company machine refuses it with', async () => {
		const answered = await reach('/tools/person_list/invoke', departedToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: {} })
		});
		expect(answered.status).toBe(403);
		expect((answered.body as { message: string }).message).toBe('token owner is not active member');
	});
});

describe('a tool a runtime beside the agent answers', () => {
	test('is refused rather than carried', async () => {
		const answered = await reach('/tools/browser_open/invoke', holdersToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: { url: 'https://example.test/' } })
		});

		expect(answered.status).toBe(400);
		expect((answered.body as { error: string }).error).toContain('browser_open');
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
		const answered = await invoke('message_send', holdersToken, {
			targetType: 'channel',
			channelName: 'town-square',
			message: 'hello'
		});
		expect(answered.status).toBe(503);
	});

	test('is refused here when its input does not fit, before anything is carried', async () => {
		const answered = await invoke('message_send', holdersToken, {
			targetType: 'channel',
			conversationID: 'c1',
			message: 'hello'
		});
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.conversationID');
	});
});

describe('looking at what a destructive call would touch', () => {
	test('is refused for a tool that destroys nothing', async () => {
		const answered = await preview('task_list', holdersToken, {});
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('destroys nothing');
	});

	test('is refused by rung before it resolves anything', async () => {
		const answered = await preview('task_delete', readersToken, { taskHint: 'anything' });
		expect(answered.status).toBe(403);
	});

	test('reads the input against the same schema the call is held to', async () => {
		const answered = await preview('task_delete', holdersToken, { colour: 'red' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.colour');
	});
});

describe('input the catalog does not publish', () => {
	test('is refused for a value outside the enum, naming the field', async () => {
		const answered = await invoke('task_add', holdersToken, { title: 'a task', size: 'huge' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.size');
	});

	test('is refused for a field the strict object does not carry', async () => {
		const answered = await invoke('task_list', holdersToken, { colour: 'red' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.colour');
	});

	test('is refused for a required field left out', async () => {
		const answered = await invoke('task_add', holdersToken, { size: 'M' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.title');
	});

	test('lets an input the catalog does publish through to the record', async () => {
		const answered = await invoke('task_list', holdersToken, { scope: 'self' });
		expect(answered.status).toBe(200);
	});
});

function pictureCall(token: string, picture?: File): Promise<RouteAnswer> {
	const carried = new FormData();
	if (picture) carried.set('file', picture);
	return reach('/company/profile-image', token, {
		method: 'POST',
		...(picture ? { body: carried } : { headers: { 'Content-Type': 'application/json' }, body: '{}' })
	});
}

describe("the company's picture", () => {
	const png = () => new File([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])], 'logo.png', { type: 'image/png' });

	test('is refused to a token that may only read', async () => {
		const answered = await pictureCall(readersToken, png());

		expect(answered.status).toBe(403);
	});

	test('is refused to somebody who does not administer the company', async () => {
		const answered = await pictureCall(holdersToken, png());

		expect(answered.status).toBe(403);
		expect(messageOf(answered)).toContain('administrator');
	});

	test('is refused when the call carries no picture, and when it carries something else', async () => {
		expect((await pictureCall(administratorsToken)).status).toBe(400);
		const document = new File([new Uint8Array([37, 80, 68, 70])], 'terms.pdf', { type: 'application/pdf' });
		expect((await pictureCall(administratorsToken, document)).status).toBe(400);
	});

	// The bucket is private, so what comes back is an address somebody signed
	// rather than the path it was stored at, and company_settings_get answers
	// the same picture from the same row.
	test('is kept for an administrator, and answered as an address a browser can show', async () => {
		const answered = await pictureCall(administratorsToken, png());

		expect(answered.status).toBe(200);
		const kept = (answered.body as { profileImageURL: string }).profileImageURL;
		expect(kept).toContain('/storage/v1/');

		const settings = await invoke('company_settings_get', administratorsToken, {});
		expect((settings.body as { result: { profileImageURL: string } }).result.profileImageURL).toContain('/storage/v1/');
	});

	test('is taken down by an administrator, and nothing is answered in its place', async () => {
		const answered = await reach('/company/profile-image', administratorsToken, { method: 'DELETE' });

		expect(answered.status).toBe(200);
		expect((answered.body as { profileImageURL: string | null }).profileImageURL).toBeNull();

		const settings = await invoke('company_settings_get', administratorsToken, {});
		expect((settings.body as { result: { profileImageURL: string | null } }).result.profileImageURL).toBeNull();
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
			// Only the revoking operation spends a token, so only it needs one minted.
			const revocableName = `conformance-${ordinal}`;
			if (operation.path === '/token' && operation.method.toUpperCase() === 'DELETE') {
				await mint(holdersToken, { name: revocableName, permission: 'read' });
			}
			const answered = await reachDocumented(operation, revocableName);
			if ([401, 404, 405].includes(answered.status)) {
				wrong.push(`${operation.method.toUpperCase()} ${operation.path} -> ${answered.status}`);
			}
		}
		expect(wrong).toEqual([]);
	}, 60_000);

	test('cover every path these routes answer', () => {
		const documented = new Set(
			documentedOperations().map((operation) => `${operation.method} ${operation.path}`)
		);
		const served = [
			'get /tools',
			'get /tools/{name}',
			'post /tools/{name}/invoke',
			'post /tools/{name}/target',
			'get /tokens',
			'post /token',
			'delete /token',
			'post /files',
			'post /company/profile-image',
			'delete /company/profile-image',
			'post /agent/messages',
			'get /agent/replies'
		];
		expect(served.filter((operation) => !documented.has(operation))).toEqual([]);
	});
});
