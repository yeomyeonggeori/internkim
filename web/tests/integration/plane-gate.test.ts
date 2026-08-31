import { describe, expect, mock, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { toolsTheRecordRuns } = await import('../../src/lib/server/public-api/record');

// A tool that runs here is only as safe as the test that calls it. Reading the
// suite for the calls it makes is cruder than a registry of cases would be, and
// it is also the thing that cannot go stale: a test that stops calling a tool
// stops covering it, and this notices.
function toolsTheSuiteCalls(): Set<string> {
	const here = join(import.meta.dir);
	const called = new Set<string>();
	for (const entry of readdirSync(here)) {
		if (!entry.endsWith('.test.ts') || entry === 'plane-gate.test.ts') continue;
		const source = readFileSync(join(here, entry), 'utf8');
		for (const [, name] of source.matchAll(/\b(?:run|asSample|asAdmin)\('([a-z_]+)'/g)) {
			called.add(name);
		}
	}
	return called;
}

describe('the gate over the tools the record runs', () => {
	test('every one of them is called by this suite', () => {
		const called = toolsTheSuiteCalls();
		const uncalled = toolsTheRecordRuns()
			.filter((name) => !called.has(name))
			.sort();

		expect(uncalled).toEqual([]);
	});

	test('the suite calls nothing the record does not run', () => {
		const runs = new Set(toolsTheRecordRuns());
		const invented = [...toolsTheSuiteCalls()].filter((name) => !runs.has(name)).sort();

		expect(invented).toEqual([]);
	});

	// The two halves of the gate are in different runtimes, so the thing that
	// says the whole catalog is covered has to be able to see both counts.
	test('the record half covers thirteen of the catalog', () => {
		expect(toolsTheRecordRuns().length).toBe(13);
	});
});
