import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

// A device answers /attendance/api/*; a company on the central plane reads the
// tables itself. Every module that reaches those paths has to carry that fork,
// because the device arm alone works on a device and silently returns nothing
// everywhere else - which is how the clock disappeared from the rail and the
// command palette with no error anywhere.
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

describe('every module that reaches the attendance API', () => {
	// Every module now carries both arms. A new one-armed module fails this.
	test('also knows a device from the central plane', () => {
		const oneArmed = everySourceFile(sourceRoot)
			.map((path) => ({ path, source: readFileSync(path, 'utf8') }))
			.filter(({ source }) => reachesTheDevicePath(source))
			.filter(({ source }) => !source.includes('isSupabaseConfigured'))
			.map(({ path }) => path.slice(path.indexOf('src')));

		expect(oneArmed).toEqual([]);
	});

	test('is a search that finds the modules it is meant to check', () => {
		const reaching = everySourceFile(sourceRoot).filter((path) =>
			reachesTheDevicePath(readFileSync(path, 'utf8'))
		);
		expect(reaching.length > 1).toBe(true);
	});
});
