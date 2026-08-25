import { supabase } from '$lib/supabase';

export async function mutedConversations(): Promise<Set<string>> {
	const { data, error } = await supabase()
		.from('muted_conversation')
		.select('conversation_id')
		.returns<{ conversation_id: string }[]>();
	if (error) throw new Error(error.message);
	return new Set((data ?? []).map((row) => row.conversation_id));
}

export async function muteConversation(conversationID: string): Promise<void> {
	const { error } = await supabase().rpc('mute_conversation', { conversation: conversationID });
	if (error) throw new Error(error.message);
}

export async function unmuteConversation(conversationID: string): Promise<void> {
	const { error } = await supabase().rpc('unmute_conversation', { conversation: conversationID });
	if (error) throw new Error(error.message);
}
