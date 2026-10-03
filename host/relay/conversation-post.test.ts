import { describe, expect, test } from 'bun:test';
import { conversationPoster, type ChatdAnswer } from './conversation-post';
import { readInboundMessage } from './inbound-message';
import type { Addressing } from './acp-session';

const directConversationID = 'buzz:613e3118-2515-4918-951f-85b783c68cda';
const answeredThreadID = `${directConversationID}:5f1c0de4a7`;

function addressingOf(offered: Record<string, unknown>): Addressing {
	const inbound = readInboundMessage({
		platform: 'buzz',
		messageID: '5f1c0de4a7',
		prompt: 'hello from this install',
		context: { sender: { email: 'member1@example.com' } },
		...offered
	});
	if (!inbound) throw new Error('the chatd inbound body under test did not read as a message');
	return inbound.addressing;
}

function posterAnswering(answer: ChatdAnswer) {
	const asked: { capability: string; body: Record<string, unknown> }[] = [];
	const told: { conversationID: string; messageID: string }[] = [];
	const post = conversationPoster({
		askChatd: async (capability, body) => {
			asked.push({ capability, body });
			return answer;
		},
		tellBrowsers: (conversationID, messageID) => told.push({ conversationID, messageID })
	});
	return { post, asked, told };
}

const keptDeck = {
	filename: '분기 보고.pdf',
	contentType: 'application/pdf',
	address: 'http://127.0.0.1:3000/media/9f2c.pdf',
	digest: '9f2c',
	sizeBytes: 2048
};

describe('conversationPoster', () => {
	test('posts a file the messenger keeps as a message referencing it', async () => {
		const { post, asked } = posterAnswering({ status: 200, body: { messageID: 'reply-file-1' } });

		const messageID = await post(addressingOf({ conversationID: directConversationID, replyTargetID: answeredThreadID }), '', [keptDeck]);

		expect(messageID).toBe('reply-file-1');
		expect(asked).toEqual([
			{ capability: 'message.post', body: { threadID: answeredThreadID, message: '', attachments: [keptDeck] } }
		]);
	});

	test('posts the reply into the thread chatd named, not to a channel', async () => {
		const { post, asked, told } = posterAnswering({ status: 200, body: { messageID: 'reply-1' } });

		const messageID = await post(addressingOf({ conversationID: directConversationID, replyTargetID: answeredThreadID }), 'received');

		expect(messageID).toBe('reply-1');
		expect(asked).toEqual([{ capability: 'message.post', body: { threadID: answeredThreadID, message: 'received' } }]);
		expect(told).toEqual([{ conversationID: directConversationID, messageID: 'reply-1' }]);
	});

	test('posts into the conversation itself when chatd named no reply target', async () => {
		const { post, asked } = posterAnswering({ status: 200, body: { messageID: 'reply-1' } });

		await post(addressingOf({ conversationID: directConversationID }), 'received');

		expect(asked).toEqual([{ capability: 'message.post', body: { threadID: directConversationID, message: 'received' } }]);
	});

	test('a refused post fails, saying where it was going and how chatd answered, and tells no browser', async () => {
		const refusal = { error: 'relay rejected event: invalid: channel-scoped events must include an h tag' };
		const { post, told } = posterAnswering({ status: 502, body: refusal });

		const failure = await post(
			addressingOf({ conversationID: directConversationID, replyTargetID: answeredThreadID }),
			'received'
		).catch((caught: unknown) => caught);

		expect(told).toEqual([]);
		expect(failure, 'a refused post read as posted').toBeInstanceOf(Error);
		expect(String(failure)).toContain(answeredThreadID);
		expect(String(failure)).toContain('502');
		expect(String(failure)).toContain(refusal.error);
	});
});
