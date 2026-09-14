import { isInsideNativeShell } from './shell';

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
	if (!isInsideNativeShell()) return;
	const { shell } = await foregroundNotifications();
	await shell.holdBack({ tag: conversationID ? messageNotificationTag(conversationID) : '' });
}
