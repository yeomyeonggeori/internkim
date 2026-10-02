import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'bun:test';

import {
	forgetKeptPictures,
	keptPictureOf,
	readKeptPictures,
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
		writeKeptPictures(new Map([['account-1', keptPictureOf(address, signedURL, keptAt)]]));
	});

	test('are read back on a later visit while the signature has time left', () => {
		expect(readKeptPictures(keptAt + hour).get('account-1')?.signedURL).toBe(signedURL);
	});

	test('are dropped once the signature is within an hour of expiring', () => {
		expect(readKeptPictures(keptAt + 23 * hour + 1).has('account-1')).toBe(false);
	});

	test('are gone after being forgotten', () => {
		forgetKeptPictures();

		expect(readKeptPictures(keptAt).size).toBe(0);
	});
});
