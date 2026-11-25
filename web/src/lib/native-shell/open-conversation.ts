import { holdBackTauriNotifications } from './open-conversation-tauri';
import { isInsideTauriShell, shellPlatform } from './shell';
import { messageNotificationTag } from '$lib/notifications/message-tag';

const tauriPlatformsThatHoldBack = ['ios', 'macos'];

export async function holdBackNotificationsFor(conversationID: string | undefined): Promise<void> {
	const tag = conversationID ? messageNotificationTag(conversationID) : '';
	if (!isInsideTauriShell()) return;
	if (tauriPlatformsThatHoldBack.includes(shellPlatform())) await holdBackTauriNotifications(tag);
}
