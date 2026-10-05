import { describe, expect, test } from 'bun:test';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import { annotationsOf, answerMCP, descriptorMetaKey, toolsOfferedTo } from '$lib/server/public-api/mcp';
import { isSeenByAModel, toolsAModelReachesWith, toolsReachableBy } from '$lib/server/public-api/catalog';
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

function aToolOnlyTheAPIReaches(): string {
	const hidden = toolsReachableBy('delete').find((descriptor) => !isSeenByAModel(descriptor.name));
	if (!hidden) throw new Error('the catalog hides no reachable tool from a model, so this case has nothing to hold');
	return hidden.name;
}

describe('the tools offered over MCP', () => {
	test('are the catalog tools a model sees that the permission reaches, carrying their own descriptor', () => {
		const offered = toolsOfferedTo(aMemberWhoMay('delete'));
		const reachable = toolsAModelReachesWith('delete');

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

	test('leave out a tool the catalog hides from a model, though the API still answers it', () => {
		const offered = toolsOfferedTo(aMemberWhoMay('delete')).map((tool) => tool.name);
		expect(offered).not.toContain(aToolOnlyTheAPIReaches());
	});

	test('include the data room tools a skill reads the record through', () => {
		const offered = toolsOfferedTo(aMemberWhoMay('delete')).map((tool) => tool.name);
		expect(offered).toContain('company_document_search');
		expect(offered).toContain('company_document_list');
	});
});

describe('the annotations a harness decides from', () => {
	const offered = toolsOfferedTo(aMemberWhoMay('delete'));
	const descriptors = toolsAModelReachesWith('delete');

	test('mark every tool that requires approval destructive, and no other', () => {
		expect(descriptors.some((descriptor) => descriptor.requiresApproval)).toBe(true);
		for (const [ordinal, descriptor] of descriptors.entries()) {
			expect(offered[ordinal].annotations?.destructiveHint).toBe(descriptor.requiresApproval ? true : undefined);
		}
	});

	test('mark every tool that only reads read-only, and no other', () => {
		for (const [ordinal, descriptor] of descriptors.entries()) {
			const readsOnly = descriptor.sideEffectClass === 'read' || descriptor.sideEffectClass === 'computation';
			expect(offered[ordinal].annotations?.readOnlyHint).toBe(readsOnly ? true : undefined);
		}
	});

	test('carry neither hint on a tool that writes without approval', () => {
		const writing = descriptors.find(
			(descriptor) => descriptor.sideEffectClass === 'workspace_write' && !descriptor.requiresApproval
		);
		if (!writing) throw new Error('the catalog has no unapproved workspace write to hold');
		expect(annotationsOf(writing)).toEqual({});
	});

	test('state no fact a descriptor does not', () => {
		for (const tool of offered) {
			for (const key of Object.keys(tool.annotations ?? {})) {
				expect(['readOnlyHint', 'destructiveHint']).toContain(key);
			}
		}
	});

	test('reach a client over the wire', async () => {
		const connected = await anMCPClient('delete');
		try {
			const listed = (await connected.listTools()).tools;
			expect(listed.map((tool) => tool.annotations)).toEqual(offered.map((tool) => tool.annotations));
		} finally {
			await connected.close();
		}
	});
});

describe('the MCP server on a fetch handler', () => {
	test('completes the handshake and lists its tools over Streamable HTTP', async () => {
		const connected = await anMCPClient('delete');
		try {
			const listed = (await connected.listTools()).tools;
			expect(listed.map((tool) => tool.name)).toEqual(
				toolsAModelReachesWith('delete').map((descriptor) => descriptor.name)
			);
		} finally {
			await connected.close();
		}
	});

	test('refuses a call to a tool the catalog hides from a model', async () => {
		const connected = await anMCPClient('delete');
		try {
			const answered = await connected.callTool({ name: aToolOnlyTheAPIReaches(), arguments: {} });
			expect(answered.isError).toBe(true);
		} finally {
			await connected.close();
		}
	});
});
