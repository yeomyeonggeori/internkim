import { afterEach, beforeEach, describe, expect, mock, test } from 'bun:test';

type PermissionState = 'granted' | 'denied' | 'prompt';

let held: PermissionState = 'prompt';
let answered: PermissionState = 'denied';

mock.module('@capacitor/push-notifications', () => ({
	PushNotifications: {
		checkPermissions: async () => ({ receive: held }),
		requestPermissions: async () => ({ receive: answered }),
		unregister: async () => {},
		register: async () => {},
		addListener: async () => ({ remove: async () => {} })
	}
}));

const stored = new Map<string, string>();
const originalWindow = globalThis.window;

function standIn(isNative: boolean): void {
	Object.defineProperty(globalThis, 'window', {
		value: {
			Capacitor: { isNativePlatform: () => isNative, getPlatform: () => 'android' },
			localStorage: {
				getItem: (key: string) => stored.get(key) ?? null,
				setItem: (key: string, value: string) => void stored.set(key, value),
				removeItem: (key: string) => void stored.delete(key)
			}
		},
		configurable: true
	});
}

async function askOnce() {
	const { askToBeReachedOnce } = await import('../../src/lib/notifications/ask-once');
	return askToBeReachedOnce();
}

describe('asking about notifications once', () => {
	beforeEach(() => {
		stored.clear();
		held = 'prompt';
		answered = 'denied';
	});

	afterEach(() => {
		Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
	});

	test('a browser is never asked', async () => {
		standIn(false);
		expect(await askOnce()).toBe('not-asked');
		expect(stored.size).toBe(0);
	});

	test('a refusal is reported and remembered', async () => {
		standIn(true);
		expect(await askOnce()).toBe('refused');
		expect(stored.get('internkim.push.asked')).toBe('yes');
	});

	test('a device that was already asked is left alone', async () => {
		standIn(true);
		stored.set('internkim.push.asked', 'yes');
		expect(await askOnce()).toBe('not-asked');
	});

	test('a device that already refuses in its own settings is not asked', async () => {
		standIn(true);
		held = 'denied';
		expect(await askOnce()).toBe('not-asked');
		expect(stored.get('internkim.push.asked')).toBe('yes');
	});
});
