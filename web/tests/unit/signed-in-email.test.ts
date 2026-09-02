import { afterAll, afterEach, describe, expect, mock, test } from 'bun:test';

let configured = false;
let heldEmail: string | undefined;
let sessionLookups = 0;

const centralPlane = await import('../../src/lib/supabase');

mock.module('$lib/supabase', () => ({
	...centralPlane,
	isSupabaseConfigured: () => configured,
	supabase: () => ({
		auth: {
			getSession: async () => {
				sessionLookups += 1;
				return { data: { session: heldEmail === undefined ? null : { user: { email: heldEmail } } } };
			}
		}
	})
}));

const { signedInEmail } = await import('../../src/lib/signed-in-email');

const realFetch = globalThis.fetch;

afterEach(() => {
	globalThis.fetch = realFetch;
	configured = false;
	heldEmail = undefined;
	sessionLookups = 0;
});

afterAll(() => {
	mock.module('$lib/supabase', () => centralPlane);
});

describe('signedInEmail', () => {
	test('on a device it is the /auth/session email', async () => {
		const asked: string[] = [];
		globalThis.fetch = (async (input: RequestInfo | URL) => {
			asked.push(String(input));
			return new Response(JSON.stringify({ email: ' Member1@Example.com ' }));
		}) as typeof fetch;

		expect(await signedInEmail()).toBe('member1@example.com');
		expect(asked).toEqual(['/auth/session']);
		expect(sessionLookups).toBe(0);
	});

	test('on the central plane it is the Supabase session email, lowercased', async () => {
		configured = true;
		heldEmail = ' Member1@Example.com ';
		globalThis.fetch = (async () => {
			throw new Error('the device session must not be asked on the central plane');
		}) as unknown as typeof fetch;

		expect(await signedInEmail()).toBe('member1@example.com');
		expect(sessionLookups).toBe(1);
	});

	test('on the central plane with nobody signed in it is empty', async () => {
		configured = true;
		heldEmail = undefined;

		expect(await signedInEmail()).toBe('');
		expect(sessionLookups).toBe(1);
	});
});
