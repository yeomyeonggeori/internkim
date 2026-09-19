import { describe, expect, mock, test } from 'bun:test';

const asked: { capability: string; body: Record<string, unknown> }[] = [];

mock.module('../../../src/lib/host-bridge', () => ({
	callCompanyApp: async ({ capability, body }: { capability: string; body: Record<string, unknown> }) => {
		asked.push({ capability, body });
		return { status: 200, body: {} };
	}
}));

const { deletePost, addReaction, removeReaction } = await import('../../../src/lib/messenger/messenger-api');
const { reactionValueFor } = await import('../../../src/lib/components/channel/channel-reactions');

describe('what the message-action capabilities are asked to carry', () => {
	test('deletePost asks person.message.delete with the conversation and message it names', async () => {
		asked.length = 0;

		await deletePost('channel-1', 'message-1');

		const call = asked.find((entry) => entry.capability === 'person.message.delete');
		expect(call?.body).toEqual({ conversationID: 'channel-1', messageID: 'message-1' });
	});

	test('addReaction asks person.reaction.add with the conversation, message and emoji it names', async () => {
		asked.length = 0;

		await addReaction('channel-1', 'message-1', 'thumbsup');

		const call = asked.find((entry) => entry.capability === 'person.reaction.add');
		expect(call?.body).toEqual({ conversationID: 'channel-1', messageID: 'message-1', emoji: 'thumbsup' });
	});

	test('removeReaction asks person.reaction.remove with the conversation, message and emoji it names', async () => {
		asked.length = 0;

		await removeReaction('channel-1', 'message-1', 'thumbsup');

		const call = asked.find((entry) => entry.capability === 'person.reaction.remove');
		expect(call?.body).toEqual({ conversationID: 'channel-1', messageID: 'message-1', emoji: 'thumbsup' });
	});
});

describe('which reaction a picked glyph joins', () => {
	const existing = [
		{ emoji: '👍', value: 'thumbsup', count: 2 },
		{ emoji: '🎉', value: 'custom:party', count: 1, imageURL: 'https://example.com/party.png' }
	];

	test('a glyph already on the message joins that reaction, not a twin of it', () => {
		expect(reactionValueFor('👍', existing)).toBe('thumbsup');
	});

	test('a glyph not on the message starts a reaction under its own value', () => {
		expect(reactionValueFor('😀', existing)).toBe('😀');
	});

	test('a glyph carried only by an image reaction is ignored, so it still starts its own value', () => {
		expect(reactionValueFor('🎉', existing)).toBe('🎉');
	});
});
