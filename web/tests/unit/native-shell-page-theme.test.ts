import { afterEach, describe, expect, test } from 'bun:test';
import { hexOfComputedColor, pageThemeShell } from '$lib/native-shell/page-theme';

describe('hexOfComputedColor', () => {
	test('turns a computed rgb() colour into upper-case hex', () => {
		expect(hexOfComputedColor('rgb(9, 9, 11)')).toBe('#09090B');
		expect(hexOfComputedColor('rgb(255, 255, 255)')).toBe('#FFFFFF');
	});

	test('ignores an alpha channel', () => {
		expect(hexOfComputedColor('rgba(9, 9, 11, 0.5)')).toBe('#09090B');
	});

	test('answers null for a value it cannot read', () => {
		expect(hexOfComputedColor('transparent')).toBeNull();
		expect(hexOfComputedColor('')).toBeNull();
	});
});

describe('pageThemeShell inside the Tauri app', () => {
	type Invocation = { command: string; args: unknown };
	const originalWindow = globalThis.window;
	const originalIsTauri = Reflect.get(globalThis, 'isTauri');

	function insideTauri(platform: string, invocations: Invocation[]): void {
		Object.defineProperty(globalThis, 'window', {
			value: {
				__TAURI_INTERNALS__: {
					invoke: async (command: string, args: unknown) => {
						invocations.push({ command, args });
					}
				},
				__TAURI_OS_PLUGIN_INTERNALS__: { platform }
			},
			configurable: true
		});
		Reflect.set(globalThis, 'isTauri', true);
	}

	afterEach(() => {
		Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
		Reflect.set(globalThis, 'isTauri', originalIsTauri);
	});

	test('hands the page theme to the page-theme plugin on a phone', async () => {
		const invocations: Invocation[] = [];
		insideTauri('android', invocations);
		await (await pageThemeShell())?.apply({ background: '#09090B', isDark: true });
		expect(invocations).toEqual([{ command: 'plugin:page-theme|apply', args: { background: '#09090B', isDark: true } }]);
	});

	test('leaves a desktop window alone', async () => {
		insideTauri('macos', []);
		expect(await pageThemeShell()).toBeNull();
	});
});
