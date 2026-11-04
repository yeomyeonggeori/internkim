import type { Page } from '@playwright/test';

export async function insideTheIPhoneApp(page: Page) {
	await page.addInitScript(() => {
		const choices: boolean[] = [];
		Reflect.set(window, 'backGestureChoices', choices);
		Reflect.set(window, 'isTauri', true);
		Reflect.set(window, '__TAURI_OS_PLUGIN_INTERNALS__', { platform: 'ios' });
		Reflect.set(window, '__TAURI_INTERNALS__', {
			invoke: async (command: string, args: { isAllowed?: boolean }) => {
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
