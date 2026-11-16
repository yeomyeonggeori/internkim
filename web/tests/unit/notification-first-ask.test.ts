import { afterEach, beforeEach, describe, expect, test } from 'bun:test';

type PermissionState = 'granted' | 'denied' | 'prompt';

let held: PermissionState = 'prompt';
let answered: PermissionState = 'denied';

const grantedOf: Record<PermissionState, boolean | null> = { granted: true, denied: false, prompt: null };

const pluginAnswers: Record<string, () => unknown> = {
	'plugin:notifications|is_permission_granted': () => grantedOf[held],
	'plugin:notifications|request_permission': () => (answered === 'prompt' ? 'default' : answered)
};

const stored = new Map<string, string>();
const originalWindow = globalThis.window;
const originalIsTauri = Reflect.get(globalThis, 'isTauri');

function standIn(isNative: boolean): void {
	Reflect.set(globalThis, 'isTauri', isNative);
	Object.defineProperty(globalThis, 'window', {
		value: {
			__TAURI_INTERNALS__: { invoke: async (command: string) => pluginAnswers[command]?.() },
			__TAURI_OS_PLUGIN_INTERNALS__: { platform: 'android' },
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
		Reflect.set(globalThis, 'isTauri', originalIsTauri);
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
