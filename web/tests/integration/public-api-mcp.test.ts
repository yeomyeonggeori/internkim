import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import type { Tool } from '@modelcontextprotocol/sdk/types.js';
import {
	addMember,
	controlPlane,
	issuePersonalAccessToken,
	provisionCompany,
} from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { fallback: reachTheAPI } = await import('../../src/routes/api/v1/[...path]/+server');
const { POST: reachMCP } = await import('../../src/routes/api/v1/mcp/+server');
const { descriptorMetaKey } = await import('../../src/lib/server/public-api/mcp');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `public-api-mcp-${Date.now()}`;
const address = 'https://space.example.test/api/v1';

let companyID = '';
let holdersToken = '';
let readersToken = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Public API MCP Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;

	const memberID = await addMember(client, companyID, `${slug}-holder@example.test`);
	const { data: account } = await client.auth.admin.createUser({
		email: `${slug}-holder@example.test`,
		email_confirm: true
	});
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);

	holdersToken = await issuePersonalAccessToken(client, memberID, 'holder', 'delete');
	readersToken = await issuePersonalAccessToken(client, memberID, 'reader', 'read');
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

// The route exports POST alone, so anything else is answered 405 by the router
// before it reaches a handler. The stand-in answers the same way.
let refusedNonPostCalls = 0;

function fetchingTheRoute(token: string) {
	return async (url: string | URL | Request, options?: RequestInit): Promise<Response> => {
		const request = new Request(url instanceof Request ? url : String(url), options);
		request.headers.set('Authorization', `Bearer ${token}`);
		if (request.method !== 'POST') {
			refusedNonPostCalls += 1;
			return new Response(null, { status: 405 });
		}
		return reachMCP({
			request,
			url: new URL(request.url),
			params: {},
			platform: undefined
		} as unknown as Parameters<typeof reachMCP>[0]);
	};
}

async function anMCPClient(token: string): Promise<Client> {
	const connected = new Client({ name: 'internkim-test', version: '1' });
	await connected.connect(
		new StreamableHTTPClientTransport(new URL(`${address}/mcp`), { fetch: fetchingTheRoute(token) })
	);
	return connected;
}

type RouteAnswer = { status: number; body: unknown };

// A route hands a refusal to SvelteKit by throwing it, and the router that
// turns one into a response is not in the room, so this reads both.
async function reach(path: string, token: string, options: RequestInit = {}): Promise<RouteAnswer> {
	const request = new Request(`${address}${path}`, {
		...options,
		headers: { Authorization: `Bearer ${token}`, ...options.headers }
	});
	try {
		const response = await reachTheAPI({
			request,
			url: new URL(request.url),
			params: { path: path.replace(/^\//, '').split('?')[0] },
			platform: undefined
		} as unknown as Parameters<typeof reachTheAPI>[0]);
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: unknown };
		if (typeof refusal.status !== 'number') throw thrown;
		return { status: refusal.status, body: refusal.body };
	}
}

function invoke(token: string, name: string, input: unknown): Promise<RouteAnswer> {
	return reach(`/tools/${name}/invoke`, token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
}

type ListedTool = { name: string; description: string; inputSchema: unknown; outputSchema: unknown };

async function toolsOverHTTP(token: string): Promise<ListedTool[]> {
	const answered = await reach('/tools', token);
	return (answered.body as { tools: ListedTool[] }).tools;
}

function descriptorCarriedBy(tool: Tool): ListedTool {
	return (tool._meta as Record<string, ListedTool>)[descriptorMetaKey];
}

describe('the tools this token reaches', () => {
	test('are the same over MCP as over the discovery endpoint, with the same schemas', async () => {
		const connected = await anMCPClient(holdersToken);
		try {
			const overMCP = (await connected.listTools()).tools;
			const overHTTP = await toolsOverHTTP(holdersToken);

			expect(overMCP.length).toBeGreaterThan(30);
			expect(overMCP.map((tool) => tool.name).sort()).toEqual(
				overHTTP.map((tool) => tool.name).sort()
			);

			const listedByName = new Map(overHTTP.map((tool) => [tool.name, tool]));
			for (const tool of overMCP) {
				const listed = listedByName.get(tool.name) as ListedTool;
				expect(tool.inputSchema as unknown).toEqual(listed.inputSchema);
				expect(tool.description).toEqual(listed.description);
				expect(descriptorCarriedBy(tool)).toEqual(listed);
			}
		} finally {
			await connected.close();
		}
	}, networkHookTimeout);

	test('shrink to what a reading token reaches, the same way on both paths', async () => {
		const connected = await anMCPClient(readersToken);
		try {
			const overMCP = (await connected.listTools()).tools.map((tool) => tool.name).sort();
			const overHTTP = (await toolsOverHTTP(readersToken)).map((tool) => tool.name).sort();
			expect(overMCP).toEqual(overHTTP);
			expect(overMCP).not.toContain('task_add');
		} finally {
			await connected.close();
		}
	}, networkHookTimeout);
});

describe('a tool called over MCP', () => {
	test('runs against the record and answers what the invoke endpoint answers', async () => {
		const connected = await anMCPClient(holdersToken);
		try {
			const written = await connected.callTool({
				name: 'task_add',
				arguments: { title: 'MCP가 만든 업무', type: 'task' }
			});
			expect(written.isError ?? false).toBe(false);

			const overMCP = await connected.callTool({ name: 'task_list', arguments: {} });
			const overHTTP = await invoke(holdersToken, 'task_list', {});

			expect(overHTTP.status).toBe(200);
			expect(overMCP.structuredContent).toEqual(overHTTP.body as Record<string, unknown>);
			expect(JSON.stringify(overHTTP.body)).toContain('MCP가 만든 업무');
		} finally {
			await connected.close();
		}
	}, networkHookTimeout);

	test('is refused the way the invoke endpoint refuses it', async () => {
		const connected = await anMCPClient(readersToken);
		try {
			const overMCP = await connected.callTool({ name: 'task_add', arguments: { title: 'x', type: 'task' } });
			const overHTTP = await invoke(readersToken, 'task_add', { title: 'x', type: 'task' });

			expect(overHTTP.status).toBe(403);
			expect(overMCP.isError).toBe(true);
			expect(overMCP.structuredContent).toEqual(overHTTP.body as Record<string, unknown>);
		} finally {
			await connected.close();
		}
	}, networkHookTimeout);
});

test('a client whose standalone stream is refused still finished every call above', () => {
	expect(refusedNonPostCalls).toBeGreaterThan(0);
});
