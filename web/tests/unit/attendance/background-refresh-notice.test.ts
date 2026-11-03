import { afterAll, beforeEach, describe, expect, mock, test } from 'bun:test';

type ToastOptions = { action: { onClick: () => void } };

let platform: 'ios' | 'android' = 'ios';
let allowed = false;
let settingsOpened = 0;
const shown: string[] = [];
const dismissed: string[] = [];
const toastOptions: ToastOptions[] = [];

mock.module('../../../src/lib/i18n/locale.svelte', () => ({
	currentLocale: { value: 'ko' }
}));

mock.module('svelte-sonner', () => ({
	toast: Object.assign(
		(title: string, options: ToastOptions) => {
			shown.push(title);
			toastOptions.push(options);
		},
		{ dismiss: (id: string) => void dismissed.push(id) }
	)
}));

const stored = new Map<string, string>();
const visibilityListeners: (() => void)[] = [];
const originalWindow = globalThis.window;
const originalDocument = globalThis.document;

const originalIsTauri = Reflect.get(globalThis, 'isTauri');
const pluginCommand = 'plugin:attendance-widget|';

Object.defineProperty(globalThis, 'window', {
	value: {
		get __TAURI_OS_PLUGIN_INTERNALS__() {
			return { platform };
		},
		__TAURI_INTERNALS__: {
			invoke: async (command: string) => {
				if (command === `${pluginCommand}background_refresh_state`) return { allowed };
				if (command === `${pluginCommand}open_settings`) settingsOpened += 1;
				return undefined;
			}
		},
		localStorage: {
			getItem: (key: string) => stored.get(key) ?? null,
			setItem: (key: string, value: string) => void stored.set(key, value),
			removeItem: (key: string) => void stored.delete(key)
		}
	},
	configurable: true
});
Object.defineProperty(globalThis, 'document', {
	value: {
		visibilityState: 'visible',
		addEventListener: (_event: string, listener: () => void) => void visibilityListeners.push(listener)
	},
	configurable: true
});

Reflect.set(globalThis, 'isTauri', true);

const { tellWhenTheWidgetCannotFollow } = await import('../../../src/lib/attendance/background-refresh-notice');

async function settle(): Promise<void> {
	await new Promise((resolve) => setTimeout(resolve, 10));
}

async function comeBackToTheApp(): Promise<void> {
	for (const listener of visibilityListeners) listener();
	await settle();
}

describe('telling an iPhone user the widget cannot follow in the background', () => {
	beforeEach(() => {
		platform = 'ios';
		allowed = false;
		settingsOpened = 0;
		stored.clear();
		shown.length = 0;
		dismissed.length = 0;
		toastOptions.length = 0;
	});

	afterAll(() => {
		Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
		Object.defineProperty(globalThis, 'document', { value: originalDocument, configurable: true });
		Reflect.set(globalThis, 'isTauri', originalIsTauri);
	});

	test('with Background App Refresh off the notice shows once', async () => {
		tellWhenTheWidgetCannotFollow();
		await settle();
		await comeBackToTheApp();
		await comeBackToTheApp();
		expect(shown).toHaveLength(1);
	});

	test('with Background App Refresh on nothing shows', async () => {
		allowed = true;
		await comeBackToTheApp();
		expect(shown).toHaveLength(0);
	});

	test('turning it on and off again shows the notice again', async () => {
		await comeBackToTheApp();
		allowed = true;
		await comeBackToTheApp();
		expect(dismissed).toEqual(['widget-background-refresh-off']);
		allowed = false;
		await comeBackToTheApp();
		expect(shown).toHaveLength(2);
	});

	test('the notice opens Settings when asked', async () => {
		await comeBackToTheApp();
		toastOptions[0]?.action.onClick();
		await settle();
		expect(settingsOpened).toBe(1);
	});

	test('an Android phone is never told', async () => {
		platform = 'android';
		await comeBackToTheApp();
		expect(shown).toHaveLength(0);
	});
});
