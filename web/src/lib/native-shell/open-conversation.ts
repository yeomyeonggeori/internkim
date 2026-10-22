import { holdBackTauriNotifications } from './open-conversation-tauri';
import { isInsideCapacitorShell, isInsideTauriShell, shellPlatform } from './shell';

type ForegroundNotificationsPlugin = {
	holdBack(options: { tag: string }): Promise<void>;
};

let held: { shell: ForegroundNotificationsPlugin } | null = null;

export function messageNotificationTag(conversationID: string): string {
	return `message:${conversationID}`;
}

async function foregroundNotifications(): Promise<{ shell: ForegroundNotificationsPlugin }> {
	if (held) return held;
	const { registerPlugin } = await import('@capacitor/core');
	held = { shell: registerPlugin<ForegroundNotificationsPlugin>('ForegroundNotifications') };
	return held;
}

export async function holdBackNotificationsFor(conversationID: string | undefined): Promise<void> {
	const tag = conversationID ? messageNotificationTag(conversationID) : '';
	if (isInsideTauriShell()) {
		if (shellPlatform() === 'ios') await holdBackTauriNotifications(tag);
		return;
	}
	if (!isInsideCapacitorShell()) return;
	const { shell } = await foregroundNotifications();
	await shell.holdBack({ tag });
}
