import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { conversationMemberRows, rememberConversationMembers } from '../../src/lib/server/conversation-members';

function clientThatRecords(failure?: string) {
	const written: { rows: unknown; options: unknown }[] = [];
	const client = {
		from(table: string) {
			if (table !== 'notification') throw new Error(`unexpected table ${table}`);
			return {
				upsert: async (rows: unknown, options: unknown) => {
					written.push({ rows, options });
					return { error: failure ? { message: failure } : null };
				}
			};
		}
	} as unknown as SupabaseClient;
	return { client, written };
}

describe('conversationMemberRows', () => {
	test('gives everyone in the conversation an unmuted row', () => {
		expect(conversationMemberRows('channel-a', ['m1', 'm2'])).toEqual([
			{ member_id: 'm1', conversation_id: 'channel-a', is_muted: false },
			{ member_id: 'm2', conversation_id: 'channel-a', is_muted: false }
		]);
	});

	test('names each member once, however many times they were listed', () => {
		expect(conversationMemberRows('channel-a', ['m1', 'm1', '', 'm2']).map((row) => row.member_id)).toEqual(['m1', 'm2']);
	});

	test('a notification about no conversation records nobody', () => {
		expect(conversationMemberRows('', ['m1'])).toEqual([]);
	});
});

describe('rememberConversationMembers', () => {
	test('writes the rows and leaves an existing mute alone', async () => {
		const { client, written } = clientThatRecords();

		const count = await rememberConversationMembers(client, 'channel-a', ['m1', 'm2']);

		expect(count).toBe(2);
		expect(written).toHaveLength(1);
		expect(written[0]?.options).toEqual({ onConflict: 'member_id,conversation_id', ignoreDuplicates: true });
	});

	test('does not touch the table when there is nothing to record', async () => {
		const { client, written } = clientThatRecords();

		expect(await rememberConversationMembers(client, '', ['m1'])).toBe(0);
		expect(await rememberConversationMembers(client, 'channel-a', [])).toBe(0);
		expect(written).toEqual([]);
	});

	test('a refused write is an error, never a silent gap', async () => {
		const { client } = clientThatRecords('permission denied');

		await expect(rememberConversationMembers(client, 'channel-a', ['m1'])).rejects.toThrow('permission denied');
	});
});
