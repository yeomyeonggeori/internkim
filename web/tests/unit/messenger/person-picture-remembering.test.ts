import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import type { CachedPicture } from '$lib/person-picture-cache';

let originalState: unknown;
let personPicture: {
	rememberExternals(externalIDs: string[]): Promise<void>;
	pictureOfExternal(externalID: string): string;
	pictureOf(person: { memberID?: string; email?: string; externalID?: string }): string;
};

const answers = new Map<string, () => Promise<{ dataURL: string } | null>>();
const askedCounts = new Map<string, number>();
const avatarURLOfExternal = new Map<string, string | undefined>();
const kept: CachedPicture[] = [
	{
		externalID: 'unchanged-account',
		avatarURL: 'https://relay.example.com/media/aaa.png',
		dataURL: 'data:image/png;base64,old-aaa'
	},
	{
		externalID: 'repictured-account',
		avatarURL: 'https://relay.example.com/media/bbb.png',
		dataURL: 'data:image/png;base64,old-bbb'
	},
	{
		externalID: 'bared-account',
		avatarURL: 'https://relay.example.com/media/ccc.png',
		dataURL: 'data:image/png;base64,old-ccc'
	},
	{
		externalID: 'emptied-account',
		avatarURL: 'https://relay.example.com/media/ddd.png',
		dataURL: ''
	}
];
const written: CachedPicture[] = [];
const forgotten: string[] = [];

mock.module('$lib/messenger/messenger-api', () => ({
	fetchPeople: () =>
		Promise.resolve(
			[...avatarURLOfExternal].map(([externalID, avatarURL]) => ({ externalID, name: externalID, avatarURL }))
		),
	fetchProfilePicture: (externalID: string) => {
		askedCounts.set(externalID, (askedCounts.get(externalID) ?? 0) + 1);
		const answer = answers.get(externalID);
		if (!answer) return Promise.resolve({ dataURL: `data:image/png;base64,fresh-${externalID}` });
		return answer();
	}
}));

mock.module('$lib/person-picture-cache', () => ({
	readCachedPictures: () => Promise.resolve(kept),
	writeCachedPicture: (picture: CachedPicture) => {
		written.push(picture);
		return Promise.resolve();
	},
	forgetCachedPicture: (externalID: string) => {
		forgotten.push(externalID);
		return Promise.resolve();
	}
}));

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	avatarURLOfExternal.set('unchanged-account', 'https://relay.example.com/media/aaa.png');
	avatarURLOfExternal.set('repictured-account', 'https://relay.example.com/media/zzz.png');
	avatarURLOfExternal.set('bared-account', undefined);
	avatarURLOfExternal.set('emptied-account', 'https://relay.example.com/media/ddd.png');
	({ personPicture } = await import('$lib/stores/person-picture.svelte'));
	await personPicture.rememberExternals([
		'unchanged-account',
		'repictured-account',
		'bared-account',
		'emptied-account'
	]);
});

afterAll(() => {
	if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
	else Reflect.set(globalThis, '$state', originalState);
});

describe('a kept picture and the avatar URL it was kept from', () => {
	test('an account still carrying the same avatar URL is drawn from the kept copy without being asked after', () => {
		expect(personPicture.pictureOfExternal('unchanged-account')).toBe('data:image/png;base64,old-aaa');
		expect(askedCounts.get('unchanged-account')).toBeUndefined();
	});

	test('an account carrying a different avatar URL is asked after again and the kept copy replaced', () => {
		expect(personPicture.pictureOfExternal('repictured-account')).toBe(
			'data:image/png;base64,fresh-repictured-account'
		);
		expect(askedCounts.get('repictured-account')).toBe(1);
		expect(written).toContainEqual({
			externalID: 'repictured-account',
			avatarURL: 'https://relay.example.com/media/zzz.png',
			dataURL: 'data:image/png;base64,fresh-repictured-account'
		});
	});

	test('a person placed by their account alone is drawn by it, with no directory to go through', () => {
		expect(personPicture.pictureOf({ externalID: 'unchanged-account' })).toBe('data:image/png;base64,old-aaa');
		expect(personPicture.pictureOf({ externalID: 'nobody' })).toBe('');
	});

	test('an empty kept copy is dropped and the account asked after again', () => {
		expect(personPicture.pictureOfExternal('emptied-account')).toBe('data:image/png;base64,fresh-emptied-account');
		expect(askedCounts.get('emptied-account')).toBe(1);
		expect(forgotten).toContain('emptied-account');
	});

	test('an account that no longer carries a picture has the kept copy dropped rather than redrawn', () => {
		expect(personPicture.pictureOfExternal('bared-account')).toBe('');
		expect(askedCounts.get('bared-account')).toBeUndefined();
		expect(forgotten).toContain('bared-account');
	});
});

describe('an account the people list does not name', () => {
	test('a request that failed is asked again next time', async () => {
		answers.set('flaky-account', () => Promise.reject(new Error('the app answered 502')));
		await personPicture.rememberExternals(['flaky-account']);
		expect(personPicture.pictureOfExternal('flaky-account')).toBe('');

		answers.set('flaky-account', () => Promise.resolve({ dataURL: 'data:image/png;base64,drawn' }));
		await personPicture.rememberExternals(['flaky-account']);
		expect(personPicture.pictureOfExternal('flaky-account')).toBe('data:image/png;base64,drawn');
		expect(askedCounts.get('flaky-account')).toBe(2);
	});

	test('a messenger that answered with no picture is not remembered and is asked again next time', async () => {
		answers.set('bare-account', () => Promise.resolve(null));
		await personPicture.rememberExternals(['bare-account']);
		expect(personPicture.pictureOfExternal('bare-account')).toBe('');
		expect(written.map((picture) => picture.externalID)).not.toContain('bare-account');

		answers.set('bare-account', () => Promise.resolve({ dataURL: 'data:image/png;base64,late' }));
		await personPicture.rememberExternals(['bare-account']);
		expect(personPicture.pictureOfExternal('bare-account')).toBe('data:image/png;base64,late');
		expect(askedCounts.get('bare-account')).toBe(2);
	});

	test('a picture that answered is kept and not asked again', async () => {
		answers.set('drawn-account', () => Promise.resolve({ dataURL: 'data:image/png;base64,kept' }));
		await personPicture.rememberExternals(['drawn-account']);
		await personPicture.rememberExternals(['drawn-account']);
		expect(personPicture.pictureOfExternal('drawn-account')).toBe('data:image/png;base64,kept');
		expect(askedCounts.get('drawn-account')).toBe(1);
	});
});
