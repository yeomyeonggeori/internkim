import type { ChannelSummary } from '$lib/components/channel/channel-api';
import type { CompanyEvent } from '$lib/company-event';

export function shownConversations(conversations: ChannelSummary[]): ChannelSummary[] {
	return conversations.filter((conversation) => conversation.isHidden !== true);
}

export function hiddenConversationAnsweredBy(
	conversations: ChannelSummary[],
	event: CompanyEvent
): ChannelSummary | undefined {
	if (event.kind !== 'message.arrived' || !event.conversationID || !event.authorExternalID) return undefined;
	const conversation = conversations.find((candidate) => candidate.id === event.conversationID);
	if (conversation?.isHidden !== true) return undefined;
	return conversation.counterpart?.externalID === event.authorExternalID ? conversation : undefined;
}
