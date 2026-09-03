import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

// Attendance has one store, the company record. A module that fetches
// /attendance/api/* is asking a device for it, which answers nothing on a
// company and silently returns an empty screen - which is how the clock
// disappeared from the rail and the command palette with no error anywhere.
const sourceRoot = join(import.meta.dir, '..', '..', '..', 'src');

function everySourceFile(directory: string): string[] {
	return readdirSync(directory).flatMap((entry) => {
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) return everySourceFile(path);
		return /\.(ts|svelte)$/.test(entry) ? [path] : [];
	});
}

function reachesTheDevicePath(source: string): boolean {
	return /fetch\(\s*[`'"]\/attendance\/api\//.test(source);
}

describe('the attendance screens', () => {
	test('ask no device for attendance', () => {
		const asking = everySourceFile(sourceRoot)
			.map((path) => ({ path, source: readFileSync(path, 'utf8') }))
			.filter(({ source }) => reachesTheDevicePath(source))
			.map(({ path }) => path.slice(path.indexOf('src')));

		expect(asking).toEqual([]);
	});

	test('is a search that reads the modules it is meant to check', () => {
		const scanned = everySourceFile(sourceRoot);
		expect(scanned.some((path) => path.endsWith('attendance-api.ts'))).toBe(true);
	});
});
