import { afterAll, describe, expect, test } from 'bun:test';
import { mkdtempSync, rmSync } from 'node:fs';
import { readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type {
	Addressing,
	AnsweredTurn,
	BlueclawACPClient,
	MessageFacts,
	Requester
} from './acp-session';
import { InboundQueue } from './inbound-queue';
import { InboundTurns } from './inbound-turn';

const root = mkdtempSync(join(tmpdir(), 'inbound-turn-'));
let taken = 0;

function directoryForOneTest(): string {
	taken += 1;
	return join(root, `turns-${taken}`);
}

afterAll(() => rmSync(root, { recursive: true, force: true }));

type AskCall = {
	requester: Requester;
	addressing: Addressing;
	message: string;
	facts: MessageFacts | undefined;
};

type AnswerTheTurn = (message: string, addressing: Addressing) => Promise<string>;

function aClientThat(answer: AnswerTheTurn, calls: AskCall[]): BlueclawACPClient {
	const client = {
		ask: async (
			requester: Requester,
			addressing: Addressing,
			message: string,
			facts?: MessageFacts
		): Promise<AnsweredTurn> => {
			calls.push({ requester, addressing, message, facts });
			return { reply: await answer(message, addressing), progress: [], stopReason: 'end_turn' };
		}
	};
	return client as unknown as BlueclawACPClient;
}

function aClientThatSays(reply: string, calls: AskCall[]): BlueclawACPClient {
	return aClientThat(async () => reply, calls);
}

function aChatdBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		platform: 'buzz',
		conversationID: 'conversation-1',
		messageID: 'message-7',
		prompt: '박예시한테 DM 보내줘',
		context: {
			conversationType: 'direct',
			sender: { email: 'sample@example.com', name: '이샘플', handle: 'sample' }
		},
		...overrides
	};
}

const firstKey = 'buzz:conversation-1:message-7';
const secondKey = 'buzz:conversation-1:message-8';

const longestWaitMilliseconds = 3_000;

async function waitUntil(
	isReady: () => boolean | Promise<boolean>,
	waitedFor: string
): Promise<void> {
	const deadline = Date.now() + longestWaitMilliseconds;
	while (Date.now() < deadline) {
		if (await isReady()) return;
		await Bun.sleep(1);
	}
	throw new Error(`waited too long for ${waitedFor}`);
}

async function eventsStillOnDisk(directoryPath: string): Promise<string[]> {
	return (await readdir(directoryPath)).filter((name) => name.endsWith('.json'));
}

describe('InboundTurns', () => {
	test('a kept message becomes a turn and its reply goes back to the conversation', async () => {
		const directoryPath = directoryForOneTest();
		const calls: AskCall[] = [];
		const posted: { addressing: Addressing; message: string }[] = [];
		const turns = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: async (addressing, message) => {
				posted.push({ addressing, message });
			},
			waitBeforeRetrying: async () => {}
		});

		expect(await turns.keep(firstKey, aChatdBody())).toBe(true);
		await turns.settled();
		await waitUntil(() => posted.length === 1, 'the reply to reach the conversation');

		expect(calls).toHaveLength(1);
		expect(calls[0].message).toBe('박예시한테 DM 보내줘');
		expect(calls[0].requester.email).toBe('sample@example.com');
		expect(posted[0].message).toBe('보냈습니다');
		expect(posted[0].addressing.conversationID).toBe('conversation-1');
		await waitUntil(
			async () => (await eventsStillOnDisk(directoryPath)).length === 0,
			'the delivered event to leave the queue'
		);
	});

	test('the same message kept twice runs one turn and is refused the second time', async () => {
		const directoryPath = directoryForOneTest();
		const calls: AskCall[] = [];
		const posted: string[] = [];
		const turns = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: async (_addressing, message) => {
				posted.push(message);
			},
			waitBeforeRetrying: async () => {}
		});

		expect(await turns.keep(firstKey, aChatdBody())).toBe(true);
		expect(await turns.keep(firstKey, aChatdBody({ prompt: 'a later copy of the same message' }))).toBe(
			false
		);
		await turns.settled();
		await waitUntil(() => posted.length === 1, 'the reply to reach the conversation');
		await turns.settled();

		expect(calls).toHaveLength(1);
		expect(posted).toEqual(['보냈습니다']);
	});

	test('the facts of the message reach the agent alongside the words', async () => {
		const calls: AskCall[] = [];
		const posted: string[] = [];
		const turns = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: async (_addressing, message) => {
				posted.push(message);
			},
			waitBeforeRetrying: async () => {}
		});

		await turns.keep(firstKey, aChatdBody({ replyTargetID: 'message-6', isThread: true }));
		await turns.settled();
		await waitUntil(() => posted.length === 1, 'the reply to reach the conversation');

		expect(calls[0].facts?.messageID).toBe('message-7');
		expect(calls[0].facts?.replyTargetID).toBe('message-6');
		expect(calls[0].facts?.isThread).toBe(true);
		expect(calls[0].facts?.context).toEqual({
			conversationType: 'direct',
			sender: { email: 'sample@example.com', name: '이샘플', handle: 'sample' }
		});
	});

	test('the answer to a question the turn asked is handed to that turn, not run as a new one', async () => {
		const calls: AskCall[] = [];
		const posted: string[] = [];
		const askedTheQuestion = '박예시에게 보낼까요?';
		let putTheQuestion: ((addressing: Addressing) => Promise<string>) | null = null;
		const turns = new InboundTurns({
			client: aClientThat(async (_message, addressing) => {
				if (!putTheQuestion) throw new Error('the test never handed over askThePerson');
				const words = await putTheQuestion(addressing);
				return `보냈습니다: ${words}`;
			}, calls),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: async (_addressing, message) => {
				posted.push(message);
			},
			waitBeforeRetrying: async () => {}
		});
		putTheQuestion = (addressing) =>
			turns.askThePerson({ toolCallID: 'held-1', question: askedTheQuestion }, addressing);

		await turns.keep(firstKey, aChatdBody());
		await waitUntil(() => posted.includes(askedTheQuestion), 'the question to reach the requester');

		await turns.keep(secondKey, aChatdBody({ messageID: 'message-8', prompt: '응 보내줘' }));
		await waitUntil(
			() => posted.length === 2 || calls.length === 2,
			'the turn to finish on the answer'
		);

		expect(
			calls,
			'the answer to the question started a second turn instead of answering it'
		).toHaveLength(1);
		expect(posted[1]).toBe('보냈습니다: 응 보내줘');
	});

	test('a turn that keeps failing is retried and then dropped by name', async () => {
		const directoryPath = directoryForOneTest();
		const reported: string[] = [];
		let attempted = 0;
		const client = {
			ask: async (): Promise<AnsweredTurn> => {
				attempted += 1;
				throw new Error('blueclaw is not listening');
			}
		};
		const turns = new InboundTurns({
			client: client as unknown as BlueclawACPClient,
			queue: new InboundQueue({ directoryPath, attemptCeiling: 2 }),
			postToConversation: async () => {},
			waitBeforeRetrying: async () => {},
			report: (line) => reported.push(line)
		});

		await turns.keep(firstKey, aChatdBody());
		await waitUntil(
			() => reported.some((line) => line.startsWith('dropped')),
			'the event to be dropped'
		);

		expect(attempted).toBe(2);
		expect(reported.filter((line) => line.includes('failed on attempt'))).toHaveLength(1);
		const dropped = reported.find((line) => line.startsWith('dropped'));
		expect(dropped).toContain(firstKey);
		expect(dropped).toContain('after 2 attempts');
		expect(dropped).toContain('blueclaw is not listening');
		expect(await eventsStillOnDisk(directoryPath)).toEqual([]);
	});

	test('an event a stopped relay never answered is delivered once by the relay that follows it', async () => {
		const directoryPath = directoryForOneTest();
		const neverAnswered: AskCall[] = [];
		const beforeRestart = new InboundTurns({
			client: aClientThat(() => new Promise<string>(() => {}), neverAnswered),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: async () => {},
			waitBeforeRetrying: async () => {}
		});
		await beforeRestart.keep(firstKey, aChatdBody());
		await waitUntil(() => neverAnswered.length === 1, 'the stopped relay to start its turn');

		const calls: AskCall[] = [];
		const posted: string[] = [];
		const afterRestart = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: async (_addressing, message) => {
				posted.push(message);
			},
			waitBeforeRetrying: async () => {}
		});

		afterRestart.startDraining();
		await afterRestart.settled();
		await waitUntil(() => posted.length === 1, 'the second relay to deliver the event');
		await afterRestart.settled();

		expect(calls).toHaveLength(1);
		expect(posted).toEqual(['보냈습니다']);
		expect(neverAnswered).toHaveLength(1);
		expect(await eventsStillOnDisk(directoryPath)).toEqual([]);
	});

	test('an event that is not a message the agent can answer is dropped rather than retried', async () => {
		const directoryPath = directoryForOneTest();
		const reported: string[] = [];
		const calls: AskCall[] = [];
		const turns = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: async () => {},
			waitBeforeRetrying: async () => {},
			report: (line) => reported.push(line)
		});

		await turns.keep(firstKey, { prompt: '박예시한테 DM 보내줘', conversationID: 'conversation-1' });
		await turns.settled();

		expect(calls).toEqual([]);
		expect(reported).toHaveLength(1);
		expect(reported[0]).toContain(firstKey);
		expect(reported[0]).toContain('not a message the agent can answer');
		expect(await eventsStillOnDisk(directoryPath)).toEqual([]);
	});
});
