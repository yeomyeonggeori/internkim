import { describe, expect, test } from 'bun:test';
import { browserSupabaseCallSites, dataCallsIn, isBrowserReachable, recordedCallSites } from './browser-supabase-call-sites';

describe('direct Supabase calls in browser-reachable modules', () => {
	test('a table read is a call site', () => {
		expect(dataCallsIn(`supabase().from('task').select('id')`)).toEqual([`from('task')`]);
	});

	test('a chain broken across lines is a call site', () => {
		expect(dataCallsIn(`await supabase()\n\t\t.from('leave')\n\t\t.insert(row);`)).toEqual([`from('leave')`]);
	});

	test('a stored procedure keeps only its name', () => {
		expect(dataCallsIn(`supabase().rpc('task_parent_set', { target_task_id: id })`)).toEqual([
			`rpc('task_parent_set')`
		]);
	});

	test('a table named by a variable is a call site', () => {
		expect(dataCallsIn(`client.storage.from(assetBucket).upload(path, file)`)).toEqual(['from(assetBucket)']);
	});

	test('a call with a type argument or a space is a call site', () => {
		expect(dataCallsIn(`client.from<Row>('task').select()`)).toEqual([`from('task')`]);
		expect(dataCallsIn(`client.rpc ('task_parent_set')`)).toEqual([`rpc('task_parent_set')`]);
	});

	test('an array constructor is not a call site; any other capitalised receiver is', () => {
		expect(dataCallsIn(`Array.from({ length: 7 }, (_, index) => index)`)).toEqual([]);
		expect(dataCallsIn(`Uint8Array.from(binary, (character) => character.charCodeAt(0))`)).toEqual([]);
		expect(dataCallsIn(`DB.from('member')`)).toEqual([`from('member')`]);
	});

	test('what SvelteKit keeps on the server is not browser-reachable', () => {
		expect(isBrowserReachable('src/lib/server/tell.ts')).toBe(false);
		expect(isBrowserReachable('src/routes/task/+page.server.ts')).toBe(false);
		expect(isBrowserReachable('src/routes/api/v1/[...path]/+server.ts')).toBe(false);
		expect(isBrowserReachable('src/hooks.server.ts')).toBe(false);
		expect(isBrowserReachable('src/lib/task/supabase-task.ts')).toBe(true);
		expect(isBrowserReachable('src/routes/task/+page.svelte')).toBe(true);
	});

	test('the inventory holds every call site and nothing new appears', () => {
		expect(browserSupabaseCallSites()).toEqual(recordedCallSites());
	});
});
