import type { BridgedInbox, InboxEntry } from './channel-over-bridge';
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

export function mergeInbox(current: InboxEntry[], answer: Pick<BridgedInbox, 'entries' | 'held'>, checkedIDs: string[]): InboxEntry[] {
	if (!answer.held) return answer.entries;
	const heldOf = new Map(answer.held.map((held) => [held.id, held]));
	const arrivedIDs = new Set(answer.entries.map((entry) => entry.id));
	const checked = new Set(checkedIDs);
	const stillHeld = current.flatMap((entry) => {
		if (arrivedIDs.has(entry.id)) return [];
		if (!checked.has(entry.id)) return [entry];
		const held = heldOf.get(entry.id);
		if (!held) return [];
		return [{ ...entry, text: held.text, isUnread: held.isUnread }];
	});
	return [...answer.entries, ...stillHeld].sort((first, second) => second.sentAt.localeCompare(first.sentAt));
}

export function appendOlderInbox(current: InboxEntry[], older: InboxEntry[]): InboxEntry[] {
	const heldIDs = new Set(current.map((entry) => entry.id));
	return [...current, ...older.filter((entry) => !heldIDs.has(entry.id))];
}
