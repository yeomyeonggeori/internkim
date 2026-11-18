import type { Page } from '@playwright/test';

export async function insideTheIPhoneApp(page: Page) {
	await page.addInitScript(() => {
		const choices: boolean[] = [];
		const invocations: { command: string; args: unknown; options: unknown }[] = [];
		Reflect.set(window, 'backGestureChoices', choices);
		Reflect.set(window, 'tauriInvocations', invocations);
		Reflect.set(window, 'isTauri', true);
		Reflect.set(window, '__TAURI_OS_PLUGIN_INTERNALS__', { platform: 'ios' });
		Reflect.set(window, '__TAURI_INTERNALS__', {
			invoke: async (command: string, args: { isAllowed?: boolean }, options: unknown) => {
				invocations.push({ command, args, options });
				const refused: unknown = Reflect.get(window, 'refusedCommands');
				if (Array.isArray(refused) && refused.includes(command)) throw new Error(`${command} was refused`);
				if (command === 'plugin:back-gesture|apply' && typeof args.isAllowed === 'boolean') choices.push(args.isAllowed);
				return null;
			},
			transformCallback: () => 0
		});
	});
}

export async function withTheDownloadPlugins(page: Page) {
	await page.addInitScript(() => {
		Object.defineProperty(window, '__INTERNKIM_MEDIA_SAVE__', { value: true });
		Object.defineProperty(window, '__INTERNKIM_SHARE_SHEET__', { value: true });
	});
}

export async function refuseCommand(page: Page, command: string) {
	await page.evaluate((refused) => Reflect.set(window, 'refusedCommands', [refused]), command);
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

export type SentFile = { filename: string; mediaType: string; text: string };

export async function filesSentTo(page: Page, command: string): Promise<SentFile[]> {
	return page.evaluate(async (wanted) => {
		type Invocation = { command: string; args: unknown; options?: { headers?: Record<string, string> } };
		const invocations: Invocation[] = Reflect.get(window, 'tauriInvocations');
		const files = new Map<string, { filename: string; mediaType: string; chunks: Uint8Array<ArrayBuffer>[] }>();
		for (const { command, args, options } of invocations) {
			if (command !== wanted || !(args instanceof Uint8Array)) continue;
			const headers = options?.headers ?? {};
			const transfer = headers.transfer ?? '';
			const file = files.get(transfer) ?? {
				filename: decodeURIComponent(headers.filename ?? ''),
				mediaType: decodeURIComponent(headers['media-type'] ?? ''),
				chunks: []
			};
			file.chunks.push(new Uint8Array(args));
			files.set(transfer, file);
		}
		return Promise.all(
			[...files.values()].map(async ({ filename, mediaType, chunks }) => ({ filename, mediaType, text: await new Blob(chunks).text() }))
		);
	}, command);
}
