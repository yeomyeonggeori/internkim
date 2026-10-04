import type { ChannelMessage } from './channel-api';
import { messengerCacheKey, onMessengerCacheReset } from '$lib/messenger/cache-scope';

const messagesByChannel = new Map<string, ChannelMessage[]>();

export function getCachedMessages(channelID: string | undefined): ChannelMessage[] | undefined {
	const scope = messengerCacheKey();
	if (!channelID || !scope) return undefined;
	return messagesByChannel.get(JSON.stringify([scope, channelID]));
}

export function setCachedMessages(channelID: string | undefined, messages: ChannelMessage[]): void {
	const scope = messengerCacheKey();
	if (!channelID || !scope) return;
	messagesByChannel.set(JSON.stringify([scope, channelID]), messages);
}

let readerID = '';
let generation = 0;

export function channelMessageCacheGeneration(): number {
	return generation;
}

export function getCachedReaderID(): string {
	return readerID;
}

export function setCachedReaderID(id: string): void {
	readerID = id;
}

export function clearChannelMessageCache(): void {
	generation += 1;
	messagesByChannel.clear();
	readerID = '';
}

onMessengerCacheReset(clearChannelMessageCache);
