import type { CompanyEvent } from '$lib/company-event';
import { fetchChannelConversation } from '$lib/components/channel/channel-api';
import { setCachedMessages } from '$lib/components/channel/channel-message-cache';
import { messengerCacheScope, requireCurrentMessengerScope } from './cache-scope';

const isWantedAgain = new Map<string, boolean>();

export function prefetchArrivedConversation(event: CompanyEvent, openConversationID: string | undefined): void {
	const conversationID = event.conversationID;
	if (event.kind !== 'message.arrived' || !conversationID || conversationID === openConversationID) return;
	if (isWantedAgain.has(conversationID)) {
		isWantedAgain.set(conversationID, true);
		return;
	}
	isWantedAgain.set(conversationID, false);
	void keepLatestMessages(conversationID)
		.catch((failure: unknown) => console.warn('the messenger did not keep the messages that arrived', failure))
		.finally(() => isWantedAgain.delete(conversationID));
}

async function keepLatestMessages(conversationID: string): Promise<void> {
	do {
		isWantedAgain.set(conversationID, false);
		const scope = await messengerCacheScope();
		const conversation = await fetchChannelConversation(conversationID);
		await requireCurrentMessengerScope(scope);
		setCachedMessages(conversationID, conversation.messages);
	} while (isWantedAgain.get(conversationID));
}
