import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';

let originalState: unknown;
let personPicture: {
	rememberExternals(externalIDs: string[]): Promise<void>;
	pictureOfExternal(externalID: string): string;
	pictureOf(person: { memberID?: string; email?: string; externalID?: string }): string;
};

const projectURL = 'https://company.supabase.co';
const answers = new Map<string, () => Promise<{ address: string } | null>>();
const askedWith: { externalID: string; avatarURL: string }[] = [];
const avatarURLOfExternal = new Map<string, string | undefined>();
let signingFails = false;

const centralPlane = { ...(await import('$lib/supabase')) };
const messengerAPI = { ...(await import('$lib/messenger/messenger-api')) };

function addressOf(name: string): string {
	return `${projectURL}/storage/v1/object/asset/company-1/shared/person-picture/${name}`;
}

function signedOf(name: string): string {
	return `${projectURL}/storage/v1/object/sign/asset/company-1/shared/person-picture/${name}?token=t`;
}

function timesAsked(externalID: string): number {
	return askedWith.filter((asked) => asked.externalID === externalID).length;
}

mock.module('$lib/supabase', () => ({
	...centralPlane,
	isSupabaseConfigured: () => true,
	projectURL: () => projectURL,
	supabase: () => ({
		storage: {
			from: () => ({
				createSignedUrls: async (paths: string[]) => {
					if (signingFails) return { data: null, error: { message: 'no session' } };
					return {
						data: paths.map((path) => ({ path, signedUrl: `${projectURL}/storage/v1/object/sign/asset/${path}?token=t` })),
						error: null
					};
				}
			})
		}
	})
}));

mock.module('$lib/messenger/messenger-api', () => ({
	...messengerAPI,
	fetchPeople: () =>
		Promise.resolve(
			[...avatarURLOfExternal].map(([externalID, avatarURL]) => ({ externalID, name: externalID, avatarURL }))
		),
	keepPersonPictureForReading: (person: { externalID: string; avatarURL: string }) => {
		askedWith.push(person);
		const answer = answers.get(person.externalID);
		if (!answer) return Promise.resolve({ address: addressOf(`${person.externalID}.png`) });
		return answer();
	}
}));

beforeAll(async () => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	avatarURLOfExternal.set('drawn-account', 'https://relay.example.com/media/aaa.png');
	avatarURLOfExternal.set('bare-account', '');
	({ personPicture } = await import('$lib/stores/person-picture.svelte'));
	await personPicture.rememberExternals(['drawn-account', 'bare-account', 'unlisted-account']);
});

afterAll(() => {
	mock.module('$lib/supabase', () => centralPlane);
	mock.module('$lib/messenger/messenger-api', () => messengerAPI);
	if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
	else Reflect.set(globalThis, '$state', originalState);
});

describe('a picture the company keeps', () => {
	test('is drawn from the address the reader signed for, asked after by the url the directory names', () => {
		expect(personPicture.pictureOfExternal('drawn-account')).toBe(signedOf('drawn-account.png'));
		expect(askedWith).toContainEqual({
			externalID: 'drawn-account',
			avatarURL: 'https://relay.example.com/media/aaa.png'
		});
	});

	test('is asked after once, however many surfaces want it', async () => {
		await personPicture.rememberExternals(['drawn-account']);
		await personPicture.rememberExternals(['drawn-account']);
		expect(timesAsked('drawn-account')).toBe(1);
	});

	test('an account the people list names with no picture is never asked after', () => {
		expect(timesAsked('bare-account')).toBe(0);
		expect(personPicture.pictureOfExternal('bare-account')).toBe('');
	});

	test('an account the people list does not name is asked after once, without a url', () => {
		expect(askedWith).toContainEqual({ externalID: 'unlisted-account', avatarURL: '' });
		expect(personPicture.pictureOfExternal('unlisted-account')).toBe(signedOf('unlisted-account.png'));
	});

	test('a person placed by their account alone is drawn by it, with no directory to go through', () => {
		expect(personPicture.pictureOf({ externalID: 'drawn-account' })).toBe(signedOf('drawn-account.png'));
		expect(personPicture.pictureOf({ externalID: 'nobody' })).toBe('');
	});
});

describe('a picture that did not come', () => {
	test('a request that failed is asked again next time', async () => {
		answers.set('flaky-account', () => Promise.reject(new Error('the app answered 502')));
		await personPicture.rememberExternals(['flaky-account']);
		expect(personPicture.pictureOfExternal('flaky-account')).toBe('');

		answers.set('flaky-account', () => Promise.resolve({ address: addressOf('flaky-account.png') }));
		await personPicture.rememberExternals(['flaky-account']);
		expect(personPicture.pictureOfExternal('flaky-account')).toBe(signedOf('flaky-account.png'));
		expect(timesAsked('flaky-account')).toBe(2);
	});

	test('an address the reader could not sign for is asked again next time', async () => {
		signingFails = true;
		await personPicture.rememberExternals(['unsigned-account']);
		expect(personPicture.pictureOfExternal('unsigned-account')).toBe('');

		signingFails = false;
		await personPicture.rememberExternals(['unsigned-account']);
		expect(personPicture.pictureOfExternal('unsigned-account')).toBe(signedOf('unsigned-account.png'));
		expect(timesAsked('unsigned-account')).toBe(2);
	});

	test('someone the messenger knows with no picture is not asked again', async () => {
		answers.set('none-account', () => Promise.resolve(null));
		await personPicture.rememberExternals(['none-account']);
		await personPicture.rememberExternals(['none-account']);
		expect(personPicture.pictureOfExternal('none-account')).toBe('');
		expect(timesAsked('none-account')).toBe(1);
	});
});
