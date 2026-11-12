import { describe, expect, test } from 'bun:test';
import {
	browserSupabaseCallSites,
	clientAccessesIn,
	isBrowserReachable,
	recordedCallSites,
	recordedInventory
} from './browser-supabase-call-sites';

const allowedReasons: Record<string, string> = {
	auth: 'resolves the Supabase auth session itself into who is signed in, not a business record.',
	realtime: 'a Realtime channel name or subscription, not a table read.',
	storage: 'a Storage bucket handle, not a table read.',
	messenger: 'the messenger account-routing map (member.messenger, contact.messenger), which record/people.ts and record/crm.ts deliberately never select.'
};

describe('direct Supabase calls in browser-reachable modules', () => {
	test('a table read is a call site', () => {
		expect(clientAccessesIn(`supabase().from('task').select('id')`)).toEqual([`from('task')`]);
	});

	test('a chain broken across lines is a call site', () => {
		expect(clientAccessesIn(`await supabase()\n\t\t.from('leave')\n\t\t.insert(row);`)).toEqual([`from('leave')`]);
	});

	test('a stored procedure keeps only its name', () => {
		expect(clientAccessesIn(`supabase().rpc('task_parent_set', { target_task_id: id })`)).toEqual([
			`rpc('task_parent_set')`
		]);
	});

	test('a project function the browser invokes itself is a call site', () => {
		expect(clientAccessesIn(`await supabase().functions.invoke('announce-attendance', { body: { what } })`)).toEqual([
			`functions.invoke('announce-attendance')`
		]);
	});

	test('any other member of the client is a call site', () => {
		expect(clientAccessesIn(`supabase().auth.getSession()`)).toEqual(['auth.getSession']);
		expect(clientAccessesIn(`supabase().storage.from(assetBucket).upload(path, file)`)).toEqual(['storage.from(assetBucket)']);
		expect(clientAccessesIn(`void supabase().removeChannel(channel);`)).toEqual(['removeChannel']);
	});

	test('a name the client is kept under is the client', () => {
		expect(clientAccessesIn(`const client = supabase();\nclient.from<Row>('member').select();`)).toEqual([`from('member')`]);
		expect(clientAccessesIn(`function f(client: SupabaseClient = supabase()) { client.auth.signOut(); }`)).toEqual([
			'auth.signOut'
		]);
	});

	test('a from that is not on the client is not a call site', () => {
		expect(clientAccessesIn(`Array.from({ length: 7 }, (_, index) => index)`)).toEqual([]);
		expect(clientAccessesIn(`Fragment.from(schema.text(text))`)).toEqual([]);
	});

	test('what SvelteKit keeps on the server is not browser-reachable', () => {
		expect(isBrowserReachable('src/lib/server/tell.ts')).toBe(false);
		expect(isBrowserReachable('src/routes/task/+page.server.ts')).toBe(false);
		expect(isBrowserReachable('src/routes/api/v1/[...path]/+server.ts')).toBe(false);
		expect(isBrowserReachable('src/hooks.server.ts')).toBe(false);
		expect(isBrowserReachable('src/lib/task/task-record.ts')).toBe(true);
		expect(isBrowserReachable('src/routes/task/+page.svelte')).toBe(true);
	});

	test('the inventory holds every call site and nothing new appears', () => {
		expect(browserSupabaseCallSites()).toEqual(recordedCallSites());
	});

	test('every allowed call site names a declared reason', () => {
		const { allowed } = recordedInventory();
		for (const reason of Object.keys(allowed)) {
			expect(allowedReasons).toHaveProperty(reason);
		}
	});
});
