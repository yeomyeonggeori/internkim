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
const { isSeenByAModel } = await import('../../src/lib/server/public-api/catalog');
const { assetBucket } = await import('../../src/lib/server/public-api/asset-address');
const { capabilityAnsweredFilesMetaKey } = await import('../../src/lib/server/public-api/catalog/protocol');

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
	test('are over MCP the ones on the discovery endpoint a model sees, with the same schemas', async () => {
		const connected = await anMCPClient(holdersToken);
		try {
			const overMCP = (await connected.listTools()).tools;
			const overHTTP = await toolsOverHTTP(holdersToken);

			expect(overMCP.length).toBeGreaterThan(30);
			expect(overMCP.map((tool) => tool.name).sort()).toEqual(
				overHTTP.map((tool) => tool.name).filter(isSeenByAModel).sort()
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
			const overHTTP = (await toolsOverHTTP(readersToken)).map((tool) => tool.name).filter(isSeenByAModel).sort();
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

type EmbeddedFile = { uri: string; mimeType?: string; text?: string; blob?: string };

function filesCarriedBy(answer: Awaited<ReturnType<Client['callTool']>>): Map<string, EmbeddedFile> {
	const content = answer.content as Array<{ type: string; resource?: EmbeddedFile }>;
	const files = content.flatMap((item) => (item.type === 'resource' && item.resource ? [item.resource] : []));
	return new Map(files.map((file) => [decodeURIComponent(file.uri.replace('internkim://files/', '')), file]));
}

async function aKeptServiceFile(name: string, categoryCode: string, date: string, extension: string, bytes: string): Promise<string> {
	const documentID = crypto.randomUUID();
	const storagePath = `${companyID}/dataroom/${categoryCode[0]}/${categoryCode}/${name}.${date}.${documentID}.${extension}`;
	await client.storage.from(assetBucket).upload(storagePath, new TextEncoder().encode(bytes), { contentType: `image/${extension}` });
	const { error } = await client.from('company_document').insert({
		id: documentID, company_id: companyID, kind: 'internal', document_type: name, title: name,
		category_code: categoryCode, document_date: date, storage_path: storagePath
	});
	if (error) throw new Error(error.message);
	return storagePath;
}

describe('the company profile read over MCP', () => {
	test('carries the profile as a file, with the newest seal and logo beside it, to a member no circle lets read their categories', async () => {
		const older = await aKeptServiceFile('seal', 'CR', '2026-03-01', 'png', 'an older seal');
		const sameDayFirst = await aKeptServiceFile('seal', 'CR', '2026-09-01', 'png', 'a seal replaced the same day');
		const newest = await aKeptServiceFile('seal', 'CR', '2026-09-01', 'jpg', 'the newest seal');
		const logo = await aKeptServiceFile('logo', 'SM', '2026-09-01', 'png', 'a logo');
		await client.from('company').update({ profile: { name: { ko: '주식회사 예시' } } }).eq('id', companyID);

		const connected = await anMCPClient(holdersToken);
		try {
			const answered = await connected.callTool({
				name: 'company_info_get',
				arguments: { language: 'ko' },
				_meta: { [capabilityAnsweredFilesMetaKey]: 'kept' }
			});
			const files = filesCarriedBy(answered);
			const profile = JSON.parse(String(files.get('company-profile.json')?.text));

			expect(answered.isError ?? false).toBe(false);
			expect(profile.name).toBe('주식회사 예시');
			expect(profile.sealImage).toBe('seal.jpg');
			expect(profile.logoImage).toBe('logo.png');
			expect(atob(String(files.get('seal.jpg')?.blob))).toBe('the newest seal');
			expect(atob(String(files.get('logo.png')?.blob))).toBe('a logo');
			expect([...files.keys()].sort()).toEqual(['company-profile.json', 'logo.png', 'seal.jpg']);
		} finally {
			await connected.close();
			await client.storage.from(assetBucket).remove([older, sameDayFirst, newest, logo]);
		}
	}, networkHookTimeout);

	test('leaves the rest of the category hidden from that member', async () => {
		const articlesID = crypto.randomUUID();
		await client.from('company_document').insert({
			id: articlesID, company_id: companyID, document_type: 'articles', title: 'Articles', category_code: 'CR',
			storage_path: `${companyID}/dataroom/C/CR/articles.${articlesID}.pdf`
		});

		const seal = await aKeptServiceFile('seal', 'CR', '2026-10-04', 'png', 'a seal');

		const listed = await invoke(holdersToken, 'company_document_list', { categoryCode: 'CR' });
		const documents = (listed.body as { result: { documents: { storagePath: string | null }[] } }).result.documents;
		await client.storage.from(assetBucket).remove([seal]);

		expect(listed.status).toBe(200);
		expect(documents.map((document) => document.storagePath)).toContain(seal);
		expect(documents.some((document) => document.storagePath?.includes(articlesID))).toBe(false);
	}, networkHookTimeout);

	test('carries no file to a caller that does not keep them, nor for a tool that answers none', async () => {
		const connected = await anMCPClient(holdersToken);
		try {
			const undeclared = await connected.callTool({ name: 'company_info_get', arguments: { language: 'ko' } });
			const fileless = await connected.callTool({
				name: 'task_list',
				arguments: {},
				_meta: { [capabilityAnsweredFilesMetaKey]: 'kept' }
			});

			expect(filesCarriedBy(undeclared).size).toBe(0);
			expect(undeclared.structuredContent).toEqual((await invoke(holdersToken, 'company_info_get', { language: 'ko' })).body as Record<string, unknown>);
			expect(filesCarriedBy(fileless).size).toBe(0);
		} finally {
			await connected.close();
		}
	}, networkHookTimeout);
});

test('a client whose standalone stream is refused still finished every call above', () => {
	expect(refusedNonPostCalls).toBeGreaterThan(0);
});
