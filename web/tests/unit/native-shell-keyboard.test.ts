import { afterEach, describe, expect, test } from 'bun:test';
import { followNativeKeyboard } from '$lib/native-shell/keyboard';

type RawMessage = { index: number; message: { isOpen: boolean } };

const originalWindow = globalThis.window;
const originalIsTauri = Reflect.get(globalThis, 'isTauri');

function insideTauri(platform: string, commands: string[], callbacks: ((raw: RawMessage) => void)[]): void {
	Object.defineProperty(globalThis, 'window', {
		value: {
			__TAURI_INTERNALS__: {
				transformCallback: (callback: (raw: RawMessage) => void) => callbacks.push(callback) - 1,
				invoke: async (command: string) => {
					commands.push(command);
				}
			},
			__TAURI_OS_PLUGIN_INTERNALS__: { platform }
		},
		configurable: true,
		writable: true
	});
	Reflect.set(globalThis, 'isTauri', true);
}

const settled = () => new Promise((resolve) => setTimeout(resolve, 0));

afterEach(() => {
	Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
	Reflect.set(globalThis, 'isTauri', originalIsTauri);
});

describe('followNativeKeyboard inside the Tauri app', () => {
	test('hears the Android keyboard open and close through the keyboard plugin', async () => {
		const commands: string[] = [];
		const callbacks: ((raw: RawMessage) => void)[] = [];
		const changes: boolean[] = [];
		insideTauri('android', commands, callbacks);
		const stop = followNativeKeyboard((isOpen) => changes.push(isOpen));
		await settled();
		expect(commands).toEqual(['plugin:keyboard|register_listener']);
		callbacks.at(-1)?.({ index: 0, message: { isOpen: true } });
		callbacks.at(-1)?.({ index: 1, message: { isOpen: false } });
		expect(changes).toEqual([true, false]);
		stop();
		await settled();
		expect(commands).toEqual(['plugin:keyboard|register_listener', 'plugin:keyboard|remove_listener']);
	});

	test('listens to nothing on iOS, where the web view already shrinks for the keyboard', async () => {
		const commands: string[] = [];
		insideTauri('ios', commands, []);
		followNativeKeyboard(() => {})();
		await settled();
		expect(commands).toEqual([]);
	});
});
