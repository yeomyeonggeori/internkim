import type { InboxEntry } from './channel-over-bridge';
import { inboxStorageKey, type MessengerCacheScope } from './cache-scope';

export const INBOX_PAGE_SIZE = 50;

export function readCachedInbox(scope: MessengerCacheScope): InboxEntry[] {
	try {
		if (typeof localStorage === 'undefined') return [];
		const cached: unknown = JSON.parse(localStorage.getItem(inboxStorageKey(scope)) ?? '[]');
		return Array.isArray(cached) ? (cached as InboxEntry[]) : [];
	} catch {
		return [];
	}
}

export function keepCachedInbox(scope: MessengerCacheScope, entries: InboxEntry[]): void {
	try {
		if (typeof localStorage !== 'undefined') localStorage.setItem(inboxStorageKey(scope), JSON.stringify(entries.slice(0, INBOX_PAGE_SIZE)));
	} catch (refusal) {
		console.warn('the messenger could not keep its inbox on this device', refusal);
	}
}

export function appendOlderInbox(current: InboxEntry[], older: InboxEntry[]): InboxEntry[] {
	const shownIDs = new Set(current.map((entry) => entry.id));
	return [...current, ...older.filter((entry) => !shownIDs.has(entry.id))];
}
