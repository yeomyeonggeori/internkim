import { describe, expect, test } from 'bun:test';
import catalogDocument from '../../../../../pkg/capabilityprotocol/generated/capability-tools.json';
import {
	baseCatalogAnswer,
	parseToolCatalog,
	permissionForTool,
	toolReachableBy,
	toolsReachableBy
} from '$lib/server/public-api/catalog';

describe('the permission a tool asks for', () => {
	test('comes from its side effect class, and an unknown class asks for the most', () => {
		expect(permissionForTool({ sideEffectClass: 'read' })).toBe('read');
		expect(permissionForTool({ sideEffectClass: 'workspace_write' })).toBe('write');
		expect(permissionForTool({ sideEffectClass: 'destructive' })).toBe('delete');
		expect(permissionForTool({ sideEffectClass: 'a class nobody wrote yet' })).toBe('delete');
	});
});

describe('the tools a permission reaches', () => {
	test('grow with the rung, and a reading token sees only reading tools', () => {
		const reading = toolsReachableBy('read');
		const writing = toolsReachableBy('write');
		const deleting = toolsReachableBy('delete');

		expect(reading.every((descriptor) => descriptor.sideEffectClass === 'read')).toBe(true);
		expect(writing.length > reading.length).toBe(true);
		expect(deleting.length > writing.length).toBe(true);
		expect(reading.map((descriptor) => descriptor.name)).toContain('task_list');
		expect(reading.map((descriptor) => descriptor.name)).not.toContain('task_add');
	});

	test('name one tool only when the permission reaches it', () => {
		expect(toolReachableBy('task_list', 'read')?.name).toBe('task_list');
		expect(toolReachableBy('task_delete', 'write')).toBeUndefined();
		expect(toolReachableBy('task_delete', 'delete')?.name).toBe('task_delete');
		expect(toolReachableBy('no_such_tool', 'delete')).toBeUndefined();
	});
});

describe('the base catalog answer', () => {
	test('says where the live set is asked for, on this app rather than elsewhere', () => {
		const answered = baseCatalogAnswer('read');
		expect(answered.catalog.source).toBe('base');
		expect(answered.catalog.live).toBe('/api/v1/tools?live=true');
		expect(answered.catalog.protocolVersion).toMatch(/^\d+\.\d+\.\d+$/);
		expect(answered.tools).toEqual(toolsReachableBy('read'));
	});
});

describe('the canonical tool descriptor boundary', () => {
	test('round-trips the complete generated catalog without pruning or rewriting metadata', () => {
		const generatedDocument: unknown = catalogDocument;
		const parsedDocument: unknown = parseToolCatalog(generatedDocument);
		expect(parsedDocument).toEqual(generatedDocument);
	});

	test('preserves approval, idempotency, and effect metadata', () => {
		const descriptor = toolReachableBy('message_send', 'delete');
		expect(descriptor).toBeDefined();
		if (!descriptor) return;

		const parsed = parseToolCatalog({ protocolVersion: 'test', tools: [descriptor] });
		expect(parsed.tools[0]).toEqual(descriptor);
		expect(parsed.tools[0]?.requiresApproval).toBe(true);
		expect(parsed.tools[0]?.idempotency).toEqual(descriptor.idempotency);
		expect(parsed.tools[0]?.resultContract).toEqual(descriptor.resultContract);
	});

	test('refuses missing required metadata instead of filling it', () => {
		const descriptor = toolReachableBy('message_send', 'delete');
		expect(descriptor).toBeDefined();
		if (!descriptor) return;

		const { idempotency: _missingIdempotency, ...incompleteDescriptor } = descriptor;
		expect(() =>
			parseToolCatalog({ protocolVersion: 'test', tools: [incompleteDescriptor] })
		).toThrow();
	});

	test('refuses malformed approval metadata', () => {
		const descriptor = toolReachableBy('message_send', 'delete');
		expect(descriptor).toBeDefined();
		if (!descriptor) return;

		expect(() =>
			parseToolCatalog({
				protocolVersion: 'test',
				tools: [{ ...descriptor, requiresApproval: 'yes' }]
			})
		).toThrow();
	});

	test('refuses incomplete effect metadata', () => {
		const descriptor = toolReachableBy('message_send', 'delete');
		expect(descriptor).toBeDefined();
		expect(descriptor?.resultContract).toBeDefined();
		if (!descriptor || !descriptor.resultContract) return;

		expect(() =>
			parseToolCatalog({
				protocolVersion: 'test',
				tools: [
					{
						...descriptor,
						resultContract: {
							...descriptor.resultContract,
							effects: [{ objectType: 'message', effect: 'sent' }]
						}
					}
				]
			})
		).toThrow();
	});

	test('refuses descriptor fields outside the canonical schema', () => {
		const descriptor = toolReachableBy('message_send', 'delete');
		expect(descriptor).toBeDefined();
		if (!descriptor) return;

		expect(() =>
			parseToolCatalog({
				protocolVersion: 'test',
				tools: [{ ...descriptor, inventedApprovalPolicy: 'always' }]
			})
		).toThrow();
	});
});
