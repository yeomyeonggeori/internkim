import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'bun:test';

import {
	forgetKeptPictures,
	freshSignedURLOf,
	keptPictureOf,
	readKeptAccounts,
	readKeptPictures,
	writeKeptAccounts,
	writeKeptPictures
} from '$lib/stores/person-picture-cache';

function createMemoryStorage(): Storage {
	const entries = new Map<string, string>();
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

const hour = 60 * 60 * 1000;
const keptAt = Date.parse('2026-07-01T00:00:00Z');
const address = 'https://company.supabase.co/storage/v1/object/asset/company-1/shared/person-picture/a.png';
const signedURL = 'https://company.supabase.co/storage/v1/object/sign/asset/company-1/shared/person-picture/a.png?token=t';
const avatarURL = 'https://relay.example.com/media/aaa.png';

const originalWindowDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'window');

beforeAll(() => {
	Object.defineProperty(globalThis, 'window', {
		value: { localStorage: createMemoryStorage() },
		configurable: true,
		writable: true
	});
});

afterAll(() => {
	if (originalWindowDescriptor) {
		Object.defineProperty(globalThis, 'window', originalWindowDescriptor);
		return;
	}
	delete (globalThis as { window?: Window }).window;
});

describe('kept picture signatures', () => {
	beforeEach(() => {
		window.localStorage.clear();
		writeKeptPictures(new Map([['account-1', keptPictureOf(address, signedURL, avatarURL, keptAt)]]));
	});

	test('are read back on a later visit while the signature has time left', () => {
		const kept = readKeptPictures().get('account-1');

		expect(kept && freshSignedURLOf(kept, keptAt + hour)).toBe(signedURL);
	});

	test('keep the picture address once the signature is within an hour of expiring', () => {
		const kept = readKeptPictures().get('account-1');

		expect(kept?.address).toBe(address);
		expect(kept && freshSignedURLOf(kept, keptAt + 23 * hour + 1)).toBe('');
	});

	test('are gone after being forgotten', () => {
		forgetKeptPictures();

		expect(readKeptPictures().size).toBe(0);
	});
});

describe('kept accounts', () => {
	beforeEach(() => {
		window.localStorage.clear();
		writeKeptAccounts({
			memberOfEmail: new Map([['sample@example.com', 'member-1']]),
			externalsOfMember: new Map([['member-1', ['account-1']]])
		});
	});

	test('are read back on a later visit, before the directory answers', () => {
		const accounts = readKeptAccounts();

		expect(accounts?.memberOfEmail.get('sample@example.com')).toBe('member-1');
		expect(accounts?.externalsOfMember.get('member-1')).toEqual(['account-1']);
	});

	test('are gone after being forgotten', () => {
		forgetKeptPictures();

		expect(readKeptAccounts()).toBeNull();
	});
});
