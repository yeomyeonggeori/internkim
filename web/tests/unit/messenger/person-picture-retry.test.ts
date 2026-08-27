import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';

let originalState: unknown;
let personPicture: {
	rememberExternals(externalIDs: string[]): Promise<void>;
	pictureOfExternal(externalID: string): string;
};

const answers = new Map<string, () => Promise<{ dataURL: string } | null>>();
const askedCounts = new Map<string, number>();

mock.module('$lib/messenger/messenger-api', () => ({
	fetchProfilePicture: (externalID: string) => {
		askedCounts.set(externalID, (askedCounts.get(externalID) ?? 0) + 1);
		const answer = answers.get(externalID);
		if (!answer) return Promise.resolve(null);
		return answer();
	}
}));

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	({ personPicture } = await import('$lib/stores/person-picture.svelte'));
});

afterAll(() => {
	if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
	else Reflect.set(globalThis, '$state', originalState);
});

describe('personPicture.rememberExternals', () => {
	test('a request that failed is asked again next time', async () => {
		answers.set('flaky-account', () => Promise.reject(new Error('the app answered 502')));
		await personPicture.rememberExternals(['flaky-account']);
		expect(personPicture.pictureOfExternal('flaky-account')).toBe('');

		answers.set('flaky-account', () => Promise.resolve({ dataURL: 'data:image/png;base64,drawn' }));
		await personPicture.rememberExternals(['flaky-account']);
		expect(personPicture.pictureOfExternal('flaky-account')).toBe('data:image/png;base64,drawn');
		expect(askedCounts.get('flaky-account')).toBe(2);
	});

	test('a messenger that answered with no picture is not asked again', async () => {
		answers.set('bare-account', () => Promise.resolve(null));
		await personPicture.rememberExternals(['bare-account']);
		await personPicture.rememberExternals(['bare-account']);
		expect(personPicture.pictureOfExternal('bare-account')).toBe('');
		expect(askedCounts.get('bare-account')).toBe(1);
	});

	test('a picture that answered is kept and not asked again', async () => {
		answers.set('drawn-account', () => Promise.resolve({ dataURL: 'data:image/png;base64,kept' }));
		await personPicture.rememberExternals(['drawn-account']);
		await personPicture.rememberExternals(['drawn-account']);
		expect(personPicture.pictureOfExternal('drawn-account')).toBe('data:image/png;base64,kept');
		expect(askedCounts.get('drawn-account')).toBe(1);
	});
});
