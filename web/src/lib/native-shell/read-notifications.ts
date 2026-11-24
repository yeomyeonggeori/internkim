import { clearReadNotificationsOnTheMac } from './read-notifications-tauri';
import { shellPlatform } from './shell';

export const readTagKey = 'readTag';

export function readTagOf(pushed: unknown): string {
	if (typeof pushed !== 'object' || pushed === null) return '';
	const readTag = (pushed as Record<string, unknown>)[readTagKey];
	return typeof readTag === 'string' ? readTag.trim() : '';
}

export async function keepReadConversationsCleared(): Promise<() => void> {
	if (shellPlatform() !== 'macos') return () => {};
	return clearReadNotificationsOnTheMac(readTagOf);
}
