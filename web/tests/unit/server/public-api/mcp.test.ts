import { describe, expect, test } from 'bun:test';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import { answerMCP, descriptorMetaKey, toolsOfferedTo } from '$lib/server/public-api/mcp';
import { toolsReachableBy } from '$lib/server/public-api/catalog';
import type { CallingMember } from '$lib/server/member-request';
import type { PublicAPIPermission } from '$lib/public-api-permission';

function aMemberWhoMay(permission: PublicAPIPermission): CallingMember {
	return { permission } as CallingMember;
}

const address = new URL('https://space.example.test/api/v1/mcp');

async function anMCPClient(permission: PublicAPIPermission): Promise<Client> {
	const connected = new Client({ name: 'unit', version: '1' });
	await connected.connect(
		new StreamableHTTPClientTransport(address, {
			fetch: (url, options) =>
				answerMCP(new Request(url instanceof Request ? url : String(url), options), {}, aMemberWhoMay(permission))
		})
	);
	return connected;
}

describe('the tools offered over MCP', () => {
	test('are the catalog tools the permission reaches, carrying their own descriptor', () => {
		const offered = toolsOfferedTo(aMemberWhoMay('delete'));
		const reachable = toolsReachableBy('delete');

		expect(offered.map((tool) => tool.name)).toEqual(reachable.map((descriptor) => descriptor.name));
		for (const [ordinal, tool] of offered.entries()) {
			expect(tool.inputSchema as unknown).toEqual(reachable[ordinal].inputSchema);
			expect(tool.description).toBe(reachable[ordinal].description);
			expect((tool._meta as Record<string, unknown>)[descriptorMetaKey]).toEqual(reachable[ordinal]);
		}
	});

	test('shrink to what a reading token reaches', () => {
		const reading = toolsOfferedTo(aMemberWhoMay('read')).map((tool) => tool.name);
		expect(reading).toContain('task_list');
		expect(reading).not.toContain('task_add');
	});
});

describe('the MCP server on a fetch handler', () => {
	test('completes the handshake and lists its tools over Streamable HTTP', async () => {
		const connected = await anMCPClient('delete');
		try {
			const listed = (await connected.listTools()).tools;
			expect(listed.map((tool) => tool.name)).toEqual(
				toolsReachableBy('delete').map((descriptor) => descriptor.name)
			);
		} finally {
			await connected.close();
		}
	});
});
