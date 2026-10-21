import { afterEach, describe, expect, test } from 'bun:test';
import { tauriPushDevice } from '$lib/notifications/native-device-tauri';

type Invocation = { command: string; args: unknown };
type RawMessage = { index: number; message: unknown };

const originalWindow = globalThis.window;
const originalIsTauri = Reflect.get(globalThis, 'isTauri');

function insideTauri(answers: Record<string, unknown>, invocations: Invocation[], callbacks: ((raw: RawMessage) => void)[]): void {
	Object.defineProperty(globalThis, 'window', {
		value: {
			__TAURI_INTERNALS__: {
				transformCallback: (callback: (raw: RawMessage) => void) => callbacks.push(callback) - 1,
				invoke: async (command: string, args: unknown) => {
					invocations.push({ command, args });
					return answers[command];
				}
			},
			__TAURI_OS_PLUGIN_INTERNALS__: { platform: 'android' }
		},
		configurable: true,
		writable: true
	});
	Reflect.set(globalThis, 'isTauri', true);
}

afterEach(() => {
	Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
	Reflect.set(globalThis, 'isTauri', originalIsTauri);
});

describe('the Tauri push device', () => {
	test('reads a granted permission, and treats anything else as still to ask', async () => {
		const invocations: Invocation[] = [];
		insideTauri({ 'plugin:notifications|is_permission_granted': true }, invocations, []);
		expect(await tauriPushDevice.permission()).toBe('granted');
		insideTauri({ 'plugin:notifications|is_permission_granted': false }, invocations, []);
		expect(await tauriPushDevice.permission()).toBe('prompt');
	});

	test('maps the asked answer, with default standing for not decided', async () => {
		const invocations: Invocation[] = [];
		insideTauri({ 'plugin:notifications|request_permission': 'denied' }, invocations, []);
		expect(await tauriPushDevice.askPermission()).toBe('denied');
		insideTauri({ 'plugin:notifications|request_permission': 'default' }, invocations, []);
		expect(await tauriPushDevice.askPermission()).toBe('prompt');
	});

	test('does not ask again once granted, because Android never answers that ask', async () => {
		const invocations: Invocation[] = [];
		insideTauri({ 'plugin:notifications|is_permission_granted': true }, invocations, []);
		expect(await tauriPushDevice.askPermission()).toBe('granted');
		expect(invocations.map((invocation) => invocation.command)).toEqual(['plugin:notifications|is_permission_granted']);
	});

	test('registers for push and hands back the device token', async () => {
		const invocations: Invocation[] = [];
		insideTauri({ 'plugin:notifications|register_for_push_notifications': 'fcm-sample-token' }, invocations, []);
		expect(await tauriPushDevice.registeredToken()).toBe('fcm-sample-token');
		await tauriPushDevice.unregister();
		expect(invocations.map((invocation) => invocation.command)).toEqual([
			'plugin:notifications|register_for_push_notifications',
			'plugin:notifications|unregister_for_push_notifications'
		]);
	});

	test('hears a tapped notification in the shape the page reads', async () => {
		const invocations: Invocation[] = [];
		const callbacks: ((raw: RawMessage) => void)[] = [];
		const tapped: unknown[] = [];
		insideTauri({}, invocations, callbacks);
		const stop = await tauriPushDevice.whenTapped((carried) => tapped.push(carried));
		expect(invocations.map((invocation) => invocation.command)).toEqual([
			'plugin:notifications|register_listener',
			'plugin:notifications|set_click_listener_active'
		]);
		callbacks.at(-1)?.({ index: 0, message: { id: -1, data: { openPath: '/attendance/' } } });
		expect(tapped).toEqual([{ notification: { data: { openPath: '/attendance/' } } }]);
		stop();
	});
});
