import { expect, test } from 'bun:test';
import type { Addressing, BlueclawACPClient, Requester } from './acp-session';
import { readInboundMessage } from './inbound-message';
import { InboundTurns } from './inbound-turn';

type AskedTurn = { requester: Requester; addressing: Addressing; message: string };

function aClientThatAnswers(reply: string, asked: AskedTurn[]): BlueclawACPClient {
	const client = {
		ask: async (requester: Requester, addressing: Addressing, message: string) => {
			asked.push({ requester, addressing, message });
			return { reply, progress: [], stopReason: 'end_turn' as const };
		}
	};
	return client as unknown as BlueclawACPClient;
}

const anInboundMessage = {
	requesterEmail: 'Sample@Example.test',
	requesterName: '이샘플',
	platform: 'buzz',
	conversationID: 'conversation-1',
	messageID: 'message-1',
	message: '박예시한테 DM 보내줘'
};

test('a message with no sender, conversation or words is not a turn', () => {
	expect(readInboundMessage({ ...anInboundMessage, requesterEmail: '' })).toBeNull();
	expect(readInboundMessage({ ...anInboundMessage, conversationID: '' })).toBeNull();
	expect(readInboundMessage({ ...anInboundMessage, message: '  ' })).toBeNull();
	expect(readInboundMessage('not a message')).toBeNull();
});

test('the sender is named by a lowercased address', () => {
	const inbound = readInboundMessage(anInboundMessage);
	expect(inbound?.requester.email).toBe('sample@example.test');
	expect(inbound?.addressing.conversationID).toBe('conversation-1');
});

test('a message becomes a turn and its reply goes back to the conversation', async () => {
	const asked: AskedTurn[] = [];
	const posted: string[] = [];
	const turns = new InboundTurns({
		client: aClientThatAnswers('보냈습니다', asked),
		postToConversation: async (_addressing, message) => {
			posted.push(message);
		}
	});
	const inbound = readInboundMessage(anInboundMessage);
	if (!inbound) throw new Error('the fixture is not a message');

	const answered = await turns.receive(inbound);

	expect(asked).toHaveLength(1);
	expect(asked[0].message).toBe('박예시한테 DM 보내줘');
	expect(answered?.reply).toBe('보냈습니다');
	expect(posted).toEqual(['보냈습니다']);
});

test('the next message in a conversation waiting on an approval is the answer, not a new turn', async () => {
	const asked: AskedTurn[] = [];
	const posted: string[] = [];
	const turns = new InboundTurns({
		client: aClientThatAnswers('보냈습니다', asked),
		postToConversation: async (_addressing, message) => {
			posted.push(message);
		}
	});
	const answering = turns.askThePerson(
		{ toolCallID: 'held-1', question: '박예시에게 보낼까요?' },
		{ platform: 'buzz', conversationID: 'conversation-1' }
	);
	const inbound = readInboundMessage({ ...anInboundMessage, message: '응 보내줘' });
	if (!inbound) throw new Error('the fixture is not a message');

	const answered = await turns.receive(inbound);

	expect(answered).toBeNull();
	expect(asked, 'the approval reply started a second turn instead of answering the question').toHaveLength(0);
	expect(await answering).toBe('응 보내줘');
	expect(posted).toEqual(['박예시에게 보낼까요?']);
});
