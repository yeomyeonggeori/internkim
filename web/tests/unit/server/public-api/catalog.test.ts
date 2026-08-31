import { describe, expect, test } from 'bun:test';
import {
	baseCatalogAnswer,
	permissionForTool,
	toolReachableBy,
	toolsReachableBy
} from '$lib/server/public-api/catalog';

describe('the permission a tool asks for', () => {
	test('comes from its side effect class, and an unknown class asks for the most', () => {
		expect(permissionForTool({ name: 'task_list', sideEffectClass: 'read' })).toBe('read');
		expect(permissionForTool({ name: 'task_add', sideEffectClass: 'workspace_write' })).toBe('write');
		expect(permissionForTool({ name: 'task_delete', sideEffectClass: 'destructive' })).toBe('delete');
		expect(permissionForTool({ name: 'someday', sideEffectClass: 'a class nobody wrote yet' })).toBe('delete');
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
