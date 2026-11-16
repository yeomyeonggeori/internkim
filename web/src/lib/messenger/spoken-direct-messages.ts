import type { ChannelSummary } from '$lib/components/channel/channel-api';

export function spokenDirectMessages(directMessages: ChannelSummary[], openID: string | undefined): ChannelSummary[] {
	return directMessages.filter(
		(conversation) => conversation.hasNoMessages !== true || conversation.isWithTheAgent === true || conversation.id === openID
	);
}
