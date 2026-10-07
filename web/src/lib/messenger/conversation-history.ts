import {
	fetchChannelConversation,
	type ChannelConversation,
	type ChannelMessage
} from '$lib/components/channel/channel-api';
import { isSupabaseConfigured } from '$lib/supabase';
import { bridgeConversationRecord } from './channel-over-bridge';

function readPage(channelID: string, before?: string): Promise<ChannelConversation> {
	if (isSupabaseConfigured()) return bridgeConversationRecord(channelID, before);
	return fetchChannelConversation(channelID, before);
}

export async function collectWholeConversation(
	channelID: string,
	onProgress: (loadedCount: number) => void,
	signal: AbortSignal
): Promise<ChannelMessage[]> {
	const seen = new Set<string>();
	let messages: ChannelMessage[] = [];
	let cursor: string | undefined;
	while (true) {
		signal.throwIfAborted();
		const page = await readPage(channelID, cursor);
		signal.throwIfAborted();
		const fresh = page.messages.filter((message) => !seen.has(message.id));
		for (const message of fresh) seen.add(message.id);
		messages = [...fresh, ...messages];
		onProgress(messages.length);
		if (!page.hasMoreBefore || !page.historyCursor || fresh.length === 0) return messages;
		cursor = page.historyCursor;
	}
}
