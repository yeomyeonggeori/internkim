import { describe, expect, test } from 'bun:test';
import type { Addressing } from './acp-session';
import { postToConversation, replySendBody } from './post-to-conversation';

const directAddressing: Addressing = {
	platform: 'buzz',
	conversationID: 'buzz:conversation-1:person-1',
	conversationType: 'direct',
	answeringMessageID: 'message-7'
};

describe('postToConversation', () => {
	test('posts through reply.send with canonical addressing', async () => {
		const calls: Record<string, unknown>[] = [];
		await postToConversation(
			async (_capability, body) => {
				calls.push(body);
				return { status: 200, body: { messageID: 'message-8' } };
			},
			directAddressing,
			'답변입니다'
		);

		expect(calls).toEqual([
			{
				replyTargetID: 'buzz:conversation-1:person-1',
				answeringMessageID: 'message-7',
				message: '답변입니다'
			}
		]);
	});

	test('uses the reply target when the inbound message is threaded', () => {
		expect(
			replySendBody(
				{
					...directAddressing,
					replyTargetID: 'message-6',
					isThread: true
				},
				'답변입니다'
			)
		).toEqual({ replyTargetID: 'message-6', answeringMessageID: 'message-7', message: '답변입니다' });
	});

	test('uses the conversation fallback when no reply target is present', () => {
		expect(
			replySendBody(
				{
					platform: 'buzz',
					conversationID: 'channel-1',
					conversationType: 'channel'
				},
				'답변입니다'
			)
		).toEqual({ replyTargetID: 'channel-1', message: '답변입니다' });
	});

	test('reports a non-successful chatd response', async () => {
		await expect(
			postToConversation(
				async () => ({ status: 502, body: { error: 'upstream unavailable' } }),
				directAddressing,
				'답변입니다'
			)
		).rejects.toThrow('chatd reply.send returned HTTP 502: {"error":"upstream unavailable"}');
	});
});
