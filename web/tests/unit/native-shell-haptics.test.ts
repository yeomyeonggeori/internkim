import { afterEach, describe, expect, test } from 'bun:test';
import { feelHaptic } from '$lib/native-shell/haptics';

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
		configurable: true,
		writable: true
	});
	Reflect.set(globalThis, 'isTauri', true);
}

async function invocationsSettle(invocations: Invocation[], expected: number): Promise<void> {
	for (let waited = 0; invocations.length < expected && waited < 1000; waited += 10) {
		await new Promise((resolve) => setTimeout(resolve, 10));
	}
}

afterEach(() => {
	Object.defineProperty(globalThis, 'window', { value: originalWindow, configurable: true });
	Reflect.set(globalThis, 'isTauri', originalIsTauri);
});

describe('feelHaptic inside the Tauri app', () => {
	test('plays each kind through the haptics plugin on a phone', async () => {
		const invocations: Invocation[] = [];
		insideTauri('ios', invocations);
		feelHaptic('selection');
		feelHaptic('touch');
		feelHaptic('success');
		feelHaptic('failure');
		await invocationsSettle(invocations, 4);
		expect(invocations.map((invocation) => invocation.command).sort()).toEqual(
			[
				'plugin:haptics|selection_feedback',
				'plugin:haptics|impact_feedback',
				'plugin:haptics|notification_feedback',
				'plugin:haptics|notification_feedback'
			].sort()
		);
		expect(invocations.map((invocation) => invocation.args)).toContainEqual({ style: 'light' });
		expect(invocations.map((invocation) => invocation.args)).toContainEqual({ type: 'success' });
		expect(invocations.map((invocation) => invocation.args)).toContainEqual({ type: 'error' });
	});

	test('stays still on a desktop window', async () => {
		const invocations: Invocation[] = [];
		insideTauri('macos', invocations);
		feelHaptic('touch');
		await invocationsSettle(invocations, 1);
		expect(invocations).toEqual([]);
	});
});
