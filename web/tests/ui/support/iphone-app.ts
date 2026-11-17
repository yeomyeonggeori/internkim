import type { Page } from '@playwright/test';

export async function insideTheIPhoneApp(page: Page) {
	await page.addInitScript(() => {
		const choices: boolean[] = [];
		const invocations: { command: string; args: unknown }[] = [];
		Reflect.set(window, 'backGestureChoices', choices);
		Reflect.set(window, 'tauriInvocations', invocations);
		Reflect.set(window, 'isTauri', true);
		Reflect.set(window, '__TAURI_OS_PLUGIN_INTERNALS__', { platform: 'ios' });
		Reflect.set(window, '__TAURI_INTERNALS__', {
			invoke: async (command: string, args: { isAllowed?: boolean }) => {
				invocations.push({ command, args });
				if (command === 'plugin:back-gesture|apply' && typeof args.isAllowed === 'boolean') choices.push(args.isAllowed);
				return null;
			},
			transformCallback: () => 0
		});
	});
}

export async function lastBackGestureChoice(page: Page): Promise<boolean | null> {
	return page.evaluate(() => (Reflect.get(window, 'backGestureChoices') as boolean[]).at(-1) ?? null);
}

export async function invocationsOf(page: Page, command: string): Promise<unknown[]> {
	return page.evaluate(
		(wanted) =>
			(Reflect.get(window, 'tauriInvocations') as { command: string; args: unknown }[])
				.filter((invocation) => invocation.command === wanted)
				.map((invocation) => invocation.args),
		command
	);
}
