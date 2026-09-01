import type { SupabaseClient } from './service-client.ts';

export type ConversationMemberRow = {
	member_id: string;
	conversation_id: string;
	is_muted: false;
};

export function conversationMemberRows(conversationID: string, memberIDs: string[]): ConversationMemberRow[] {
	if (!conversationID) return [];
	return [...new Set(memberIDs.filter((memberID) => memberID !== ''))].map((member_id) => ({
		member_id,
		conversation_id: conversationID,
		is_muted: false
	}));
}

export async function rememberConversationMembers(
	client: SupabaseClient,
	conversationID: string,
	memberIDs: string[]
): Promise<number> {
	const rows = conversationMemberRows(conversationID, memberIDs);
	if (rows.length === 0) return 0;
	const { error } = await client
		.from('notification')
		.upsert(rows, { onConflict: 'member_id,conversation_id', ignoreDuplicates: true });
	if (error) throw new Error(error.message);
	return rows.length;
}
