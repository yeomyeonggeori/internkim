import type { SupabaseClient } from '@supabase/supabase-js';
import { describe, expect, test } from 'bun:test';

type Row = { id: string; profile_image: string | null };

const uploaded: string[] = [];
const removed: string[][] = [];
let company: Row = { id: 'company-1', profile_image: null };
let updateAnswer: { data: unknown[]; error: null } = { data: [{ id: 'company-1' }], error: null };

const storage = {
	from: () => ({
		upload: async (path: string) => {
			uploaded.push(path);
			return { error: null };
		},
		remove: async (paths: string[]) => {
			removed.push(paths);
			return { error: null };
		},
		createSignedUrl: async (path: string) => ({ data: { signedUrl: `https://signed/${path}` }, error: null })
	})
};

// mock.module replaces a module for every suite in the run, and $lib/supabase is
// imported by most of them. The client is passed in instead.
const client = {
	storage,
	from: () => ({
		select: () => ({ limit: () => ({ single: async () => ({ data: company, error: null }) }) }),
		update: () => ({ eq: () => ({ select: async () => updateAnswer }) })
	})
} as unknown as SupabaseClient;

const { saveCompanyProfileImage } = await import('../../src/lib/company/profile-image');

describe('a company sets its own picture', () => {
	test('keeps it where the row will accept it: its own company, readable by everyone in it', async () => {
		uploaded.length = 0;
		await saveCompanyProfileImage(new File(['x'], 'logo.PNG', { type: 'image/png' }), client);

		expect(uploaded).toHaveLength(1);
		expect(uploaded[0]).toMatch(/^company-1\/shared\/company\/[0-9a-f-]+\.png$/);
	});

	// The row lets an administrator through and answers a refusal with no rows,
	// which would otherwise leave the picture in the bucket belonging to nothing.
	test('takes the picture back out when the row refuses it', async () => {
		uploaded.length = 0;
		removed.length = 0;
		updateAnswer = { data: [], error: null };

		await expect(saveCompanyProfileImage(new File(['x'], 'logo.png', { type: 'image/png' }), client)).rejects.toThrow(
			'only an administrator'
		);
		expect(removed).toEqual([[uploaded[0]]]);
		updateAnswer = { data: [{ id: 'company-1' }], error: null };
	});

	// The bucket is private, so a path is a picture only once somebody signs it.
	test('hands back something a browser can show, not the path it stored', async () => {
		const picture = await saveCompanyProfileImage(new File(['x'], 'logo.png', { type: 'image/png' }), client);

		expect(picture.readableURL).toBe(`https://signed/${picture.path}`);
	});
});
