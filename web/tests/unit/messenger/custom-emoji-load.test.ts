import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';

let originalState: unknown;
let customEmoji: { load(): Promise<void>; nameToURL: Map<string, string> };
let timesAsked = 0;

const centralPlane = { ...(await import('$lib/supabase')) };
const messengerAPI = { ...(await import('$lib/messenger/messenger-api')) };

mock.module('$lib/supabase', () => ({
	...centralPlane,
	isSupabaseConfigured: () => true
}));

mock.module('$lib/messenger/messenger-api', () => ({
	...messengerAPI,
	fetchCustomEmojiNames: () => {
		timesAsked += 1;
		return Promise.resolve([]);
	},
	fetchCustomEmojiImage: () => Promise.resolve(null)
}));

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	({ customEmoji } = await import('$lib/stores/custom-emoji.svelte'));
});

afterAll(() => {
	mock.module('$lib/supabase', () => centralPlane);
	mock.module('$lib/messenger/messenger-api', () => messengerAPI);
	if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
	else Reflect.set(globalThis, '$state', originalState);
});

describe('customEmoji.load on the plane', () => {
	test(
'takes an empty list as the answer and asks once for the page', async () => {
		const startedAt = Date.now();

		await customEmoji.load();
		await customEmoji.load();

		expect(timesAsked).toBe(1);
		expect(customEmoji.nameToURL.size).toBe(0);
		expect(Date.now() - startedAt).toBeLessThan(1000);
	});
});
