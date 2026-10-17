import { afterEach, describe, expect, test } from 'bun:test';
import { attendanceWidgetShell } from '$lib/widget/attendance-widget-bridge';

type Invocation = { command: string; args: unknown };

const originalWindow = globalThis.window;
const originalIsTauri = Reflect.get(globalThis, 'isTauri');

function insideTauri(platform: string | null, invocations: Invocation[]): void {
	const tauriWindow: Record<string, unknown> = {
		__TAURI_INTERNALS__: {
			invoke: async (command: string, args: unknown) => {
				invocations.push({ command, args });
				return { installID: 'abc', tokenName: '' };
			}
		}
	};
	if (platform) tauriWindow.__TAURI_OS_PLUGIN_INTERNALS__ = { platform };
	Object.defineProperty(globalThis, 'window', { value: tauriWindow, configurable: true });
	Reflect.set(globalThis, 'isTauri', true);
}

afterEach(() => {
	Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
	Reflect.set(globalThis, 'isTauri', originalIsTauri);
});

describe('attendanceWidgetShell inside the Tauri app', () => {
	test('reaches the attendance-widget plugin by the commands the app registers', async () => {
		const invocations: Invocation[] = [];
		insideTauri('ios', invocations);
		const shell = await attendanceWidgetShell();
		expect(shell?.platform).toBe('ios');
		await shell?.widget.install();
		await shell?.widget.supply({ token: 'ik_sample', tokenName: 'ios-widget-abc', origin: 'https://example.com' });
		await shell?.widget.forget();
		expect(invocations).toEqual([
			{ command: 'plugin:attendance-widget|install', args: {} },
			{
				command: 'plugin:attendance-widget|supply',
				args: { token: 'ik_sample', tokenName: 'ios-widget-abc', origin: 'https://example.com' }
			},
			{ command: 'plugin:attendance-widget|forget', args: {} }
		]);
	});

	test('offers no widget on a desktop build', async () => {
		insideTauri('macos', []);
		expect(await attendanceWidgetShell()).toBeNull();
	});

	test('offers no widget while the app cannot say which platform it runs on', async () => {
		insideTauri(null, []);
		expect(await attendanceWidgetShell()).toBeNull();
	});
});
