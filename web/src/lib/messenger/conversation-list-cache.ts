import type { ChannelSummary } from '$lib/components/channel/channel-api';
import { conversationStorageKey, type MessengerCacheScope } from './cache-scope';

export function readCachedConversations(scope: MessengerCacheScope): ChannelSummary[] {
	try {
		if (typeof localStorage === 'undefined') return [];
		const cached: unknown = JSON.parse(localStorage.getItem(conversationStorageKey(scope)) ?? '[]');
		return Array.isArray(cached) ? (cached as ChannelSummary[]) : [];
	} catch {
		return [];
	}
}

export function keepCachedConversations(scope: MessengerCacheScope, conversations: ChannelSummary[]): void {
	try {
		if (typeof localStorage !== 'undefined') localStorage.setItem(conversationStorageKey(scope), JSON.stringify(conversations));
	} catch (refusal) {
		console.warn('the messenger could not keep its conversation list on this device', refusal);
	}
}
