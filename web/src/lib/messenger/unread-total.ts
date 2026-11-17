import type { ChannelSummary } from '$lib/components/channel/channel-api';
import type { InboxEntry } from './channel-over-bridge';
import { shownConversations } from './hidden-direct-messages';
import type { MessengerReadCount } from './messenger-api';

export type UnreadInboxEntry = Pick<InboxEntry, 'conversationID' | 'isUnread'>;

export type MessengerUnreadTotal = {
	addressedToMe: number;
	hasChannelNews: boolean;
};

export function messengerUnreadTotal(
	conversations: ChannelSummary[],
	inboxEntries: UnreadInboxEntry[],
	mutedIDs: ReadonlySet<string>
): MessengerUnreadTotal {
	const heard = shownConversations(conversations).filter((conversation) => !mutedIDs.has(conversation.id));
	const directIDs = new Set(conversations.filter(isDirect).map((conversation) => conversation.id));
	const directMessages = heard.filter(isDirect).reduce((sum, conversation) => sum + unreadCountOf(conversation.unread), 0);
	const channelMentions = inboxEntries.filter(
		(entry) => entry.isUnread && !directIDs.has(entry.conversationID) && !mutedIDs.has(entry.conversationID)
	).length;
	return {
		addressedToMe: directMessages + channelMentions,
		hasChannelNews: heard.some((conversation) => !isDirect(conversation) && unreadCountOf(conversation.unread) > 0)
	};
}

function isDirect(conversation: ChannelSummary): boolean {
	return conversation.kind === 'dm';
}

function unreadCountOf(unread: MessengerReadCount | undefined): number {
	if (!unread || unread.status === 'unknown') return 0;
	return Math.max(unread.value, 0);
}
