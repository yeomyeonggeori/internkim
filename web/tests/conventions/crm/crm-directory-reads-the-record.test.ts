import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

// No server answers /organization/api/ on the plane: web/src/hooks.ts rewrites
// only /v1, so the request resolves through the dev proxy alone.
const crmScreen = join(import.meta.dir, '../../../src/routes/crm');
const recordDirectory = join(import.meta.dir, '../../../src/lib/organization/supabase-directory.ts');

function filesUnder(directory: string): string[] {
	return readdirSync(directory).flatMap((entry) => {
		const path = join(directory, entry);
		return statSync(path).isDirectory() ? filesUnder(path) : [path];
	});
}

function namingAnAdmindOrganizationPath(paths: string[]): string[] {
	return paths.filter((path) => readFileSync(path, 'utf8').includes('/organization/api/'));
}

describe('the CRM screen reads its directory from the record', () => {
	test('no file on the screen names an admind organization path', () => {
		expect(namingAnAdmindOrganizationPath(filesUnder(crmScreen))).toEqual([]);
	});

	test('the record branch names none either', () => {
		expect(namingAnAdmindOrganizationPath([recordDirectory])).toEqual([]);
	});

	test('the scan reads the screen rather than an empty directory', () => {
		expect(filesUnder(crmScreen).length).toBeGreaterThan(20);
	});
});
