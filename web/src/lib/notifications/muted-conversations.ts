import { invokeTool } from '$lib/public-api-call';
import { myNotificationSettings } from './settings';

type MutingAnswer = { mutedConversationIDs: string[] };

export async function mutedConversations(): Promise<Set<string>> {
	return new Set((await myNotificationSettings()).mutedConversationIDs);
}

export async function muteConversation(conversationID: string): Promise<Set<string>> {
	const answered = await invokeTool<MutingAnswer>('conversation_mute', { conversationID });
	return new Set(answered.mutedConversationIDs);
}

export async function unmuteConversation(conversationID: string): Promise<Set<string>> {
	const answered = await invokeTool<MutingAnswer>('conversation_unmute', { conversationID });
	return new Set(answered.mutedConversationIDs);
}
