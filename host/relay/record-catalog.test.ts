import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { recordCatalogName, RecordCatalogs, ticketOf, type MemberSession } from './record-catalog';

type PluginServerDeclaration = { mcpServers: Record<string, { type: string; url: string } | undefined> };

const anHourFromNow = () => Math.floor(Date.now() / 1000) + 3600;

function aCatalogWith(
	mintFor: (requesterEmail: string) => Promise<MemberSession>,
	appURL = 'http://app.test'
) {
	return new RecordCatalogs({ appURL, loopbackURL: 'http://127.0.0.1:18091', mintFor });
}

function aCall(url: string) {
	return new Request(url, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list', params: {} })
	});
}

describe('the address a session is given', () => {
	test('names this relay, not the company app', () => {
		const catalogs = aCatalogWith(async () => ({ accessToken: 'a', expiresAt: anHourFromNow() }));

		const [server] = catalogs.serversFor('sample@example.test', 'conversation-1');

		expect(server.type).toBe('http');
		expect(server.url.startsWith('http://127.0.0.1:18091/mcp/')).toBe(true);
		expect(server.headers).toEqual([]);
	});

	test('is the same ticket while the conversation is the same', () => {
		const catalogs = aCatalogWith(async () => ({ accessToken: 'a', expiresAt: anHourFromNow() }));

		const first = catalogs.serversFor('sample@example.test', 'conversation-1')[0].url;
		const second = catalogs.serversFor('sample@example.test', 'conversation-1')[0].url;
		const other = catalogs.serversFor('sample@example.test', 'conversation-2')[0].url;

		expect(second).toBe(first);
		expect(other).not.toBe(first);
	});

	test('is nothing at all for a requester with no address', () => {
		const catalogs = aCatalogWith(async () => ({ accessToken: 'a', expiresAt: anHourFromNow() }));

		expect(catalogs.serversFor('  ', 'conversation-1')).toEqual([]);
	});
});

describe('a call against that address', () => {
	test('is forwarded to the company catalog as the person the ticket names', async () => {
		let sentTo = '';
		let sentAuthorization = '';
		const originalFetch = globalThis.fetch;
		globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
			sentTo = String(input);
			sentAuthorization = String((init?.headers as Record<string, string>).Authorization);
			return new Response('{"result":{}}', {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			});
		}) as typeof fetch;
		try {
			const catalogs = aCatalogWith(async () => ({
				accessToken: 'a-member-token',
				expiresAt: anHourFromNow()
			}));
			const url = catalogs.serversFor('sample@example.test', 'conversation-1')[0].url;

			const answered = await catalogs.serve(aCall(url), ticketOf(new URL(url).pathname));

			expect(answered.status).toBe(200);
			expect(sentTo).toBe('http://app.test/api/v1/mcp');
			expect(sentAuthorization).toBe('Bearer a-member-token');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('mints once while the session is fresh and again once it is nearly spent', async () => {
		let mintCount = 0;
		const originalFetch = globalThis.fetch;
		globalThis.fetch = (async () =>
			new Response('{}', {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})) as unknown as typeof fetch;
		try {
			const catalogs = aCatalogWith(async () => {
				mintCount += 1;
				return { accessToken: `token-${mintCount}`, expiresAt: Math.floor(Date.now() / 1000) + 60 };
			});
			const url = catalogs.serversFor('sample@example.test', 'conversation-1')[0].url;
			const ticket = ticketOf(new URL(url).pathname);

			await catalogs.serve(aCall(url), ticket);
			await catalogs.serve(aCall(url), ticket);

			expect(mintCount).toBe(2);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('is refused when nobody opened a catalog under that ticket', async () => {
		const catalogs = aCatalogWith(async () => ({ accessToken: 'a', expiresAt: anHourFromNow() }));

		const answered = await catalogs.serve(
			aCall('http://127.0.0.1:18091/mcp/nobody'),
			'nobody'
		);

		expect(answered.status).toBe(404);
	});

	test('says so rather than answering when the record refuses to sign the person in', async () => {
		const catalogs = aCatalogWith(async () => {
			throw new Error('no member here has email identity');
		});
		const url = catalogs.serversFor('sample@example.test', 'conversation-1')[0].url;

		const answered = await catalogs.serve(aCall(url), ticketOf(new URL(url).pathname));

		expect(answered.status).toBe(502);
	});
});

describe('the tool server the plugin declares', () => {
	test('is the one this relay hands the agent, under the same name and at the same route', () => {
		const declarationPath = join(import.meta.dir, '..', '..', '.dependency', 'internkim-plugin', 'mcp.json');
		const declaration: PluginServerDeclaration = JSON.parse(readFileSync(declarationPath, 'utf8'));
		const server = declaration.mcpServers[recordCatalogName];

		expect(server?.type).toBe('streamable-http');
		expect(new URL(server?.url ?? 'https://unnamed.invalid').pathname).toBe('/v1/mcp');
	});
});
