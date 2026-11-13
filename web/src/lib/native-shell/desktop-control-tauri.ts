import type { Locale } from '$lib/i18n/locale';
import type { DesktopAnswer, DesktopControlPlugin, DesktopControlState, DesktopWork } from './desktop-control';

const changedEvent = 'desktop-control://changed';

export async function tauriDesktopControl(): Promise<DesktopControlPlugin> {
	const { invoke } = await import('@tauri-apps/api/core');
	const { listen } = await import('@tauri-apps/api/event');
	return {
		state: async () => controlStateOf(await invoke<unknown>('plugin:desktop-control|state')),
		allow: async (language: Locale) => controlStateOf(await invoke<unknown>('plugin:desktop-control|allow', { language })),
		askOnce: async (language: Locale) => controlStateOf(await invoke<unknown>('plugin:desktop-control|ask_once', { language })),
		disallow: async () => controlStateOf(await invoke<unknown>('plugin:desktop-control|disallow')),
		run: async (work: DesktopWork) =>
			answerOf(await invoke<unknown>('plugin:desktop-control|run', { program: work.program, arguments: work.arguments, text: work.text })),
		openPermissionGuide: async () => answerOf(await invoke<unknown>('plugin:desktop-control|open_permission_guide')),
		onChanged: (listener) =>
			listen<unknown>(changedEvent, (event) => {
				const isAllowed = recordOf(event.payload)?.is_allowed;
				if (typeof isAllowed === 'boolean') listener(isAllowed);
			})
	};
}

function recordOf(value: unknown): Record<string, unknown> | null {
	if (typeof value !== 'object' || value === null) return null;
	return { ...value };
}

function controlStateOf(value: unknown): DesktopControlState {
	const record = recordOf(value);
	return { isAllowed: record?.is_allowed === true, accessibility: accessibilityOf(record?.permissions) };
}

function accessibilityOf(permissions: unknown): string | null {
	const accessibility = recordOf(recordOf(recordOf(permissions)?.data)?.accessibility);
	return typeof accessibility?.state === 'string' ? accessibility.state : null;
}

function answerOf(value: unknown): DesktopAnswer {
	const record = recordOf(value);
	if (typeof record?.output === 'string') return { output: record.output };
	if (typeof record?.refusal === 'string') return { refusal: record.refusal };
	return { refusal: 'the desktop-control plugin answered neither an output nor a refusal' };
}
