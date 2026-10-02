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

let readerID = '';

export function getCachedReaderID(): string {
	return readerID;
}

export function setCachedReaderID(id: string): void {
	readerID = id;
}

export function clearChannelMessageCache(): void {
	messagesByChannel.clear();
	readerID = '';
}
