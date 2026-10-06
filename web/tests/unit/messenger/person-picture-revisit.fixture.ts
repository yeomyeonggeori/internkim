import { mock } from 'bun:test';

import { keptPictureOf } from '$lib/stores/person-picture-cache';

const projectURL = 'https://company.supabase.co';
const scope = 'company-1';
const askedWith: { externalID: string; avatarURL: string }[] = [];

const centralPlane = { ...(await import('$lib/supabase')) };
const messengerAPI = { ...(await import('$lib/messenger/messenger-api')) };
const cacheScope = { ...(await import('$lib/messenger/cache-scope')) };

function addressOf(name: string): string {
	return `${projectURL}/storage/v1/object/asset/company-1/shared/person-picture/${name}`;
}

function signedOf(name: string, token: string): string {
	return `${projectURL}/storage/v1/object/sign/asset/company-1/shared/person-picture/${name}?token=${token}`;
}

function storageWith(entries: Map<string, string>): Storage {
	return {
		get length() {
			return entries.size;
		},
		clear: () => entries.clear(),
		getItem: (key: string) => entries.get(key) ?? null,
		key: (index: number) => [...entries.keys()][index] ?? null,
		removeItem: (key: string) => {
			entries.delete(key);
		},
		setItem: (key: string, value: string) => {
			entries.set(key, value);
		}
	};
}

mock.module('$lib/supabase', () => ({
	...centralPlane,
	isSupabaseConfigured: () => true,
	projectURL: () => projectURL,
	supabase: () => ({
		storage: {
			from: () => ({
				createSignedUrls: async (paths: string[]) => ({
					data: paths.map((path) => ({ path, signedUrl: `${projectURL}/storage/v1/object/sign/asset/${path}?token=fresh` })),
					error: null
				})
			})
		}
	})
}));

mock.module('$lib/messenger/cache-scope', () => ({
	...cacheScope,
	messengerCacheKey: () => scope
}));

mock.module('$lib/messenger/messenger-api', () => ({
	...messengerAPI,
	fetchPeople: () =>
		Promise.resolve([
			{ externalID: 'unchanged-account', name: 'unchanged-account', avatarURL: 'https://relay.example.com/media/same.png' },
			{ externalID: 'changed-account', name: 'changed-account', avatarURL: 'https://relay.example.com/media/new.png' }
		]),
	keepPersonPictureForReading: (person: { externalID: string; avatarURL: string }) => {
		askedWith.push(person);
		return Promise.resolve({ address: addressOf(`${person.externalID}-latest.png`) });
	}
}));

const keptOnEarlierVisit = [
	[
		'unchanged-account',
		keptPictureOf(addressOf('unchanged-account.png'), signedOf('unchanged-account.png', 'kept'), 'https://relay.example.com/media/same.png')
	],
	[
		'changed-account',
		keptPictureOf(addressOf('changed-account.png'), signedOf('changed-account.png', 'kept'), 'https://relay.example.com/media/old.png')
	]
];
Object.defineProperty(globalThis, 'window', {
	value: { localStorage: storageWith(new Map([[`personPicture.signed:${scope}`, JSON.stringify(keptOnEarlierVisit)]])) },
	configurable: true,
	writable: true
});
Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const { personPicture } = await import('$lib/stores/person-picture.svelte');
await personPicture.rememberExternals(['unchanged-account', 'changed-account']);

console.log(JSON.stringify({
	askedWith,
	unchanged: personPicture.pictureOfExternal('unchanged-account'),
	changed: personPicture.pictureOfExternal('changed-account')
}));

export {};
