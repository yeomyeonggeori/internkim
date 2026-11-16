import { holdBackTauriNotifications } from './open-conversation-tauri';
import { isInsideTauriShell, shellPlatform } from './shell';

const tauriPlatformsThatHoldBack = ['ios', 'macos'];

export function messageNotificationTag(conversationID: string): string {
	return `message:${conversationID}`;
}

export async function holdBackNotificationsFor(conversationID: string | undefined): Promise<void> {
	const tag = conversationID ? messageNotificationTag(conversationID) : '';
	if (!isInsideTauriShell()) return;
	if (tauriPlatformsThatHoldBack.includes(shellPlatform())) await holdBackTauriNotifications(tag);
}
