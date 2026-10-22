import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

const accessIdentityHeader = 'cf-access-authenticated-user-email';
const sourceRoot = new URL('../../../src', import.meta.url).pathname;

function serverModulesUnder(directory: string): string[] {
	return readdirSync(directory).flatMap((entry) => {
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) return serverModulesUnder(path);
		return /\.(ts|js)$/.test(entry) ? [path] : [];
	});
}

describe('the web app', () => {
	test('takes nobody at their word through the Access identity header, which any caller can set', () => {
		const trusting = serverModulesUnder(sourceRoot).filter((path) =>
			readFileSync(path, 'utf8').toLowerCase().includes(accessIdentityHeader)
		);
		expect(trusting).toEqual([]);
	});
});
