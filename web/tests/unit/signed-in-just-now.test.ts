import type { SupabaseClient } from '@supabase/supabase-js';
import { describe, expect, test } from 'bun:test';
import { hasSignedInJustNow } from '../../src/lib/supabase-session';

const minute = 60 * 1000;

function clientSignedInAt(lastSignedInAt: string | null): SupabaseClient {
	return {
		auth: {
			getUser: async () => ({ data: { user: lastSignedInAt ? { last_sign_in_at: lastSignedInAt } : null } })
		}
	} as unknown as SupabaseClient;
}

function agoInMinutes(minutes: number): string {
	return new Date(Date.now() - minutes * minute).toISOString();
}

describe('whether somebody has just proved who they are', () => {
	test('counts a sign-in that landed seconds ago, which is what a claim code and a magic link look like', async () => {
		expect(await hasSignedInJustNow(clientSignedInAt(agoInMinutes(0)))).toBe(true);
	});

	test('counts one a few minutes old, so reading the mail does not cost the claim', async () => {
		expect(await hasSignedInJustNow(clientSignedInAt(agoInMinutes(9)))).toBe(true);
	});

	test('refuses a session left open in a browser', async () => {
		expect(await hasSignedInJustNow(clientSignedInAt(agoInMinutes(40)))).toBe(false);
		expect(await hasSignedInJustNow(clientSignedInAt(agoInMinutes(60 * 20)))).toBe(false);
	});

	test('refuses when nobody is signed in', async () => {
		expect(await hasSignedInJustNow(clientSignedInAt(null))).toBe(false);
	});
});
