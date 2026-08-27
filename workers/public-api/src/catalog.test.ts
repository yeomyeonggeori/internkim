import { describe, expect, test } from 'bun:test';
import { baseCatalogAnswer, permissionForTool, toolReachableBy, toolsReachableBy } from './catalog';

function namesReachableBy(permission: 'read' | 'write' | 'delete'): string[] {
	return toolsReachableBy(permission).map((descriptor) => descriptor.name);
}

describe('which permission a tool needs', () => {
	test('is read for a tool that only reads', () => {
		expect(permissionForTool({ name: 'task_list', sideEffectClass: 'read' })).toBe('read');
	});

	test('is write for every class that changes something short of destroying it', () => {
		expect(permissionForTool({ name: 'task_add', sideEffectClass: 'workspace_write' })).toBe('write');
		expect(permissionForTool({ name: 'message_send', sideEffectClass: 'external_send' })).toBe('write');
		expect(permissionForTool({ name: 'site_serve', sideEffectClass: 'site_publish' })).toBe('write');
		expect(permissionForTool({ name: 'browser_open', sideEffectClass: 'connect' })).toBe('write');
	});

	test('is delete for a destructive class and for a class nobody mapped', () => {
		expect(permissionForTool({ name: 'task_delete', sideEffectClass: 'destructive' })).toBe('delete');
		expect(permissionForTool({ name: 'invented', sideEffectClass: 'something_new' })).toBe('delete');
	});
});

describe('the catalog a key reaches', () => {
	test('shows a read key only the tools that read', () => {
		const names = namesReachableBy('read');
		expect(names).toContain('task_list');
		expect(names).not.toContain('task_add');
		expect(names).not.toContain('task_delete');
	});

	test('shows a write key the reads and the writes, and no deletion', () => {
		const names = namesReachableBy('write');
		expect(names).toContain('task_list');
		expect(names).toContain('task_add');
		expect(names).toContain('message_send');
		expect(names).not.toContain('task_delete');
		expect(names).not.toContain('message_delete');
	});

	test('shows a delete key everything the other two see and the deletions too', () => {
		const names = namesReachableBy('delete');
		expect(names).toContain('task_delete');
		expect(names).toContain('site_unserve');
		for (const reachable of namesReachableBy('write')) expect(names).toContain(reachable);
	});

	test('is a ladder, so each rung holds every rung below it', () => {
		const read = namesReachableBy('read');
		const write = namesReachableBy('write');
		expect(write.length).toBeGreaterThan(read.length);
		expect(namesReachableBy('delete').length).toBeGreaterThan(write.length);
		for (const reachable of read) expect(write).toContain(reachable);
	});
});

describe('reading one descriptor', () => {
	test('answers the descriptor the key reaches', () => {
		expect(toolReachableBy('task_list', 'read')?.name).toBe('task_list');
	});

	test('answers nothing for a tool above the key, as if it did not exist', () => {
		expect(toolReachableBy('task_delete', 'write')).toBeUndefined();
	});

	test('answers nothing for a name no tool has', () => {
		expect(toolReachableBy('task_invent', 'delete')).toBeUndefined();
	});
});

describe('what the discovery answer says about itself', () => {
	test('names itself the base catalog and where the live set is', () => {
		const answer = baseCatalogAnswer('delete');
		expect(answer.catalog.source).toBe('base');
		expect(answer.catalog.protocolVersion).toMatch(/^\d+\.\d+\.\d+$/);
		expect(answer.catalog.live).toBe('/v1/tools?live=true');
		expect(answer.catalog.explanation).toContain('companion');
		expect(answer.catalog.explanation).toContain('your company machine');
	});

	test('carries the tools the key reaches under the key every client already reads', () => {
		expect(baseCatalogAnswer('read').tools.map((descriptor) => descriptor.name)).toEqual(
			namesReachableBy('read')
		);
	});
});
