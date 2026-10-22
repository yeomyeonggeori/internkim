import type { ChannelSummary } from '$lib/components/channel/channel-api';
import type { InboxEntry } from '$lib/messenger/channel-over-bridge';

export type InboxPlace = { kind: 'direct'; personName: string } | { kind: 'thread'; channelName: string };

export function inboxPlaceOf(entry: InboxEntry, conversation: ChannelSummary | undefined): InboxPlace | undefined {
	if (!conversation) return undefined;
	if (conversation.kind === 'dm') return { kind: 'direct', personName: entry.sender.name || conversation.name };
	return { kind: 'thread', channelName: conversation.name };
}

export function inboxThreadRootOf(entry: InboxEntry, conversation: ChannelSummary | undefined): string | undefined {
	if (conversation?.kind === 'group') return entry.threadRootId ?? entry.id;
	return entry.threadRootId;
}
