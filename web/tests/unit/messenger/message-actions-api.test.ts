import { afterAll, describe, expect, mock, test } from 'bun:test';

const asked: { capability: string; body: Record<string, unknown> }[] = [];

const hostBridge = { ...(await import('../../../src/lib/host-bridge')) };

afterAll(() => {
	mock.module('../../../src/lib/host-bridge', () => hostBridge);
});

mock.module('../../../src/lib/host-bridge', () => ({
	...hostBridge,
	onCompanyEvent: () => () => undefined,
	callCompanyApp: async ({ capability, body }: { capability: string; body: Record<string, unknown> }) => {
		asked.push({ capability, body });
		return { status: 200, body: {} };
	}
}));

const { deletePost, addReaction, removeReaction, markChannelRead, editPost } = await import('../../../src/lib/messenger/messenger-api');
const { reactionValueFor } = await import('../../../src/lib/components/channel/channel-reactions');

describe('what the message-action capabilities are asked to carry', () => {
	test('editPost asks person.message.edit with the conversation, message and new body it names', async () => {
		asked.length = 0;

		await editPost('channel-1', 'message-1', '고친 문장');

		const call = asked.find((entry) => entry.capability === 'person.message.edit');
		expect(call?.body).toEqual({ conversationID: 'channel-1', messageID: 'message-1', body: '고친 문장' });
	});

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

describe('what the read-state capability is asked to carry', () => {
	test('markChannelRead asks person.read_state.mark with the conversation and the time it was read to', async () => {
		asked.length = 0;

		await markChannelRead('channel-1', '2026-09-28T01:05:00Z');

		const call = asked.find((entry) => entry.capability === 'person.read_state.mark');
		expect(call?.body).toEqual({ conversationID: 'channel-1', readAt: '2026-09-28T01:05:00Z' });
	});
});
