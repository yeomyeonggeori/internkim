import type { ChannelSummary } from '$lib/components/channel/channel-api';
import { shownConversations } from '$lib/messenger/hidden-direct-messages';
import { linkedMessageIDOf } from './messenger-linked-message';

export type InitialChannel =
	| { kind: 'requested'; conversation: ChannelSummary; linkedMessageID: string | undefined }
	| { kind: 'awaited' }
	| { kind: 'usual'; conversation: ChannelSummary | undefined };

export type InitialChannelQuestion = {
	url: URL;
	conversations: ChannelSummary[];
	lastChannelID: string | null;
	firstChannel: ChannelSummary | undefined;
	isListSettled: boolean;
};

export function initialChannelOf(question: InitialChannelQuestion): InitialChannel {
	const requestedID = question.url.searchParams.get('channel');
	const requested = requestedID
		? question.conversations.find((conversation) => conversation.id === requestedID)
		: undefined;
	if (requested) return { kind: 'requested', conversation: requested, linkedMessageID: linkedMessageIDOf(question.url) };
	if (requestedID && !question.isListSettled) return { kind: 'awaited' };
	const shown = shownConversations(question.conversations);
	const rememberedID = requestedID ?? question.lastChannelID;
	const remembered = rememberedID ? shown.find((conversation) => conversation.id === rememberedID) : undefined;
	return { kind: 'usual', conversation: remembered ?? question.firstChannel ?? shown[0] };
}
