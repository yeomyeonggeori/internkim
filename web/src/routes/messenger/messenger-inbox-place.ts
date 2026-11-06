import type { ChannelSummary } from '$lib/components/channel/channel-api';
import type { InboxEntry } from '$lib/messenger/channel-over-bridge';

export type InboxPlace =
	| { kind: 'direct'; personName: string }
	| { kind: 'thread'; channelName: string }
	| { kind: 'channel'; channelName: string };

export function inboxPlaceOf(entry: InboxEntry, conversation: ChannelSummary | undefined): InboxPlace | undefined {
	if (!conversation) return undefined;
	if (conversation.kind === 'dm') return { kind: 'direct', personName: entry.sender.name || conversation.name };
	if (!entry.threadRootId) return { kind: 'channel', channelName: conversation.name };
	return { kind: 'thread', channelName: conversation.name };
}

export function inboxThreadRootOf(
	entry: InboxEntry,
	conversation: ChannelSummary | undefined,
	opensChannelMessageAsThread: boolean
): string | undefined {
	if (opensChannelMessageAsThread && conversation?.kind === 'group') return entry.threadRootId ?? entry.id;
	return entry.threadRootId;
}
