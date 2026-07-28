export type AppShortcutCode = 'Slash' | 'KeyR';

export type AppShortcutMessage = {
	type: 'app-shortcut';
	code: AppShortcutCode;
};

export function isAppShortcutMessage(value: unknown): value is AppShortcutMessage {
	if (typeof value !== 'object' || value === null || !('type' in value)) return false;
	return (value as { type: unknown }).type === 'app-shortcut';
}

export function forwardAppShortcut(code: AppShortcutCode): void {
	if (typeof window === 'undefined' || window.parent === window) return;
	window.parent.postMessage({ type: 'app-shortcut', code } satisfies AppShortcutMessage, window.location.origin);
}
