import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

const sourceRoot = join(import.meta.dir, '..', '..', '..', 'src');

function everySourceFile(directory: string): string[] {
	return readdirSync(directory).flatMap((entry) => {
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) return everySourceFile(path);
		return /\.(ts|svelte)$/.test(entry) ? [path] : [];
	});
}

function reachesTheDevicePath(source: string): boolean {
	return /fetch\(\s*[`'"]\/(task|flow)\/api\//.test(source);
}

describe('the task screens', () => {
	test('ask no device for a task', () => {
		const asking = everySourceFile(sourceRoot)
			.map((path) => ({ path, source: readFileSync(path, 'utf8') }))
			.filter(({ source }) => reachesTheDevicePath(source))
			.map(({ path }) => path.slice(path.indexOf('src')));

		expect(asking).toEqual([]);
	});

	test('is a search that reads the modules it is meant to check', () => {
		const scanned = everySourceFile(sourceRoot);
		expect(scanned.some((path) => path.endsWith(join('routes', 'task', 'task-api.ts')))).toBe(true);
	});
});
