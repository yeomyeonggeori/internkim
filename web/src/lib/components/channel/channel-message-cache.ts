import type { ChannelMessage } from './channel-api';

const messagesByChannel = new Map<string, ChannelMessage[]>();

export function getCachedMessages(channelID: string | undefined): ChannelMessage[] | undefined {
	if (!channelID) return undefined;
	return messagesByChannel.get(channelID);
}

export function setCachedMessages(channelID: string | undefined, messages: ChannelMessage[]): void {
	if (!channelID) return;
	messagesByChannel.set(channelID, messages);
}
