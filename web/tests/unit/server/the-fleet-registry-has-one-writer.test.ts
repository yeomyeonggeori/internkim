import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

const routesRoot = new URL('../../../src/routes', import.meta.url).pathname;
const registryWrites = /\bkv\.(putDevice|deleteDevice)\(|\bKV\.(put|delete)\(/;

function routeModulesUnder(directory: string): string[] {
	return readdirSync(directory).flatMap((entry) => {
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) return routeModulesUnder(path);
		return entry === '+server.ts' ? [path] : [];
	});
}

describe('the device fleet registry', () => {
	test('is written by the registration route alone, which proves the device or the register secret', () => {
		const writers = routeModulesUnder(routesRoot)
			.filter((path) => registryWrites.test(readFileSync(path, 'utf8')))
			.map((path) => relative(routesRoot, path));
		expect(writers).toEqual(['api/register/+server.ts']);
	});
});
