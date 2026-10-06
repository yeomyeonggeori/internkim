import type { ChannelMessage } from './channel-api';
import { forgetStoredMessages, messageStorageKey, messengerCacheKey, onMessengerCacheReset, readerStorageKey } from '$lib/messenger/cache-scope';

const storedMessageLimit = 50;
const messagesByChannel = new Map<string, ChannelMessage[]>();

function readStoredMessages(storageKey: string): ChannelMessage[] | undefined {
	try {
		if (typeof localStorage === 'undefined') return undefined;
		const stored: unknown = JSON.parse(localStorage.getItem(storageKey) ?? 'null');
		return Array.isArray(stored) ? (stored as ChannelMessage[]) : undefined;
	} catch {
		return undefined;
	}
}

function storeMessages(storageKey: string, messages: ChannelMessage[]): void {
	try {
		if (typeof localStorage !== 'undefined') localStorage.setItem(storageKey, JSON.stringify(messages.slice(-storedMessageLimit)));
	} catch (refusal) {
		console.warn('the channel could not keep its messages on this device', refusal);
	}
}

export function getCachedMessages(channelID: string | undefined): ChannelMessage[] | undefined {
	const scope = messengerCacheKey();
	if (!channelID || !scope) return undefined;
	const key = JSON.stringify([scope, channelID]);
	const held = messagesByChannel.get(key);
	if (held) return held;
	const stored = readStoredMessages(messageStorageKey(scope, channelID));
	if (stored) messagesByChannel.set(key, stored);
	return stored;
}

export function setCachedMessages(channelID: string | undefined, messages: ChannelMessage[]): void {
	const scope = messengerCacheKey();
	if (!channelID || !scope) return;
	messagesByChannel.set(JSON.stringify([scope, channelID]), messages);
	storeMessages(messageStorageKey(scope, channelID), messages);
}

let readerID = '';
let generation = 0;

export function channelMessageCacheGeneration(): number {
	return generation;
}

export function getCachedReaderID(): string {
	if (readerID) return readerID;
	const scope = messengerCacheKey();
	if (!scope) return '';
	try {
		return typeof localStorage === 'undefined' ? '' : localStorage.getItem(readerStorageKey(scope)) ?? '';
	} catch {
		return '';
	}
}

export function setCachedReaderID(id: string): void {
	readerID = id;
	const scope = messengerCacheKey();
	if (!scope) return;
	try {
		if (typeof localStorage !== 'undefined') localStorage.setItem(readerStorageKey(scope), id);
	} catch (refusal) {
		console.warn('the channel could not keep its reader on this device', refusal);
	}
}

export function clearChannelMessageCache(): void {
	generation += 1;
	messagesByChannel.clear();
	readerID = '';
	forgetStoredMessages();
}

onMessengerCacheReset(clearChannelMessageCache);
