import type { BridgedInbox, InboxEntry } from './channel-over-bridge';
import { inboxStorageKey, type MessengerCacheScope } from './cache-scope';

const mostEntriesKept = 50;

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
		if (typeof localStorage !== 'undefined') localStorage.setItem(inboxStorageKey(scope), JSON.stringify(entries));
	} catch (refusal) {
		console.warn('the messenger could not keep its inbox on this device', refusal);
	}
}

export function mergeInbox(current: InboxEntry[], answer: BridgedInbox): InboxEntry[] {
	if (!answer.held) return answer.entries;
	const heldOf = new Map(answer.held.map((held) => [held.id, held]));
	const arrivedIDs = new Set(answer.entries.map((entry) => entry.id));
	const stillHeld = current.flatMap((entry) => {
		const held = heldOf.get(entry.id);
		if (!held || arrivedIDs.has(entry.id)) return [];
		return [{ ...entry, text: held.text, isUnread: held.isUnread }];
	});
	return [...answer.entries, ...stillHeld]
		.sort((first, second) => second.sentAt.localeCompare(first.sentAt))
		.slice(0, mostEntriesKept);
}
