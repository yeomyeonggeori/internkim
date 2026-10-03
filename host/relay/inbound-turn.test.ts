import { afterAll, describe, expect, test } from 'bun:test';
import { mkdtempSync, rmSync } from 'node:fs';
import { readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { RequestError, type StopReason } from '@agentclientprotocol/sdk';
import {
	AgentUnreachable,
	type Addressing,
	type BlueclawACPClient,
	type MessageFacts,
	type Requester
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

type Conversation = {
	posted: string[];
	postedTo: Addressing[];
	post: (addressing: Addressing, message: string) => Promise<string>;
};

function aConversation(refusal?: string): Conversation {
	const posted: string[] = [];
	const postedTo: Addressing[] = [];
	return {
		posted,
		postedTo,
		post: async (addressing, message) => {
			if (refusal) throw new Error(refusal);
			posted.push(message);
			postedTo.push(addressing);
			return `posted-${posted.length}`;
		}
	};
}

function aClientThat(answer: AnswerTheTurn, calls: AskCall[], conversation: Conversation): BlueclawACPClient {
	const client = {
		ask: async (
			requester: Requester,
			addressing: Addressing,
			message: string,
			facts?: MessageFacts
		): Promise<StopReason> => {
			calls.push({ requester, addressing, message, facts });
			const reply = await answer(message, addressing);
			if (reply) await conversation.post(addressing, reply);
			return 'end_turn';
		}
	};
	return client as unknown as BlueclawACPClient;
}

function aClientThatSays(reply: string, calls: AskCall[], conversation: Conversation): BlueclawACPClient {
	return aClientThat(async () => reply, calls, conversation);
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
const thirdKey = 'buzz:conversation-1:message-9';

function threadOf(rootMessageID: string): string {
	return `buzz:conversation-1:${rootMessageID}`;
}

function aRootMessage(messageID: string, prompt: string): Record<string, unknown> {
	return aChatdBody({ messageID, replyTargetID: threadOf(messageID), isThread: false, prompt });
}

function aReplyInTheThreadOf(rootMessageID: string, messageID: string, prompt: string): Record<string, unknown> {
	return aChatdBody({ messageID, replyTargetID: threadOf(rootMessageID), isThread: true, prompt });
}

const addressingOfTheFirstThread: Addressing = {
	platform: 'buzz',
	conversationID: 'conversation-1',
	replyTargetID: threadOf('message-7'),
	isThread: false
};

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
		const conversation = aConversation();
		const posted = conversation.posted;
		const turns = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls, conversation),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: conversation.post,
			waitBeforeRetrying: async () => {}
		});

		expect(await turns.keep(firstKey, aChatdBody())).toBe(true);
		await turns.settled();
		await waitUntil(() => posted.length === 1, 'the reply to reach the conversation');

		expect(calls).toHaveLength(1);
		expect(calls[0].message).toBe('박예시한테 DM 보내줘');
		expect(calls[0].requester.email).toBe('sample@example.com');
		expect(posted[0]).toBe('보냈습니다');
		expect(conversation.postedTo[0].conversationID).toBe('conversation-1');
		await waitUntil(
			async () => (await eventsStillOnDisk(directoryPath)).length === 0,
			'the delivered event to leave the queue'
		);
	});

	test('the same message kept twice runs one turn and is refused the second time', async () => {
		const directoryPath = directoryForOneTest();
		const calls: AskCall[] = [];
		const conversation = aConversation();
		const posted = conversation.posted;
		const turns = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls, conversation),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: conversation.post,
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
		const conversation = aConversation();
		const posted = conversation.posted;
		const turns = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls, conversation),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: conversation.post,
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

	test('a reply in the thread of a question the turn asked is handed to that turn, not run as a new one', async () => {
		const calls: AskCall[] = [];
		const conversation = aConversation();
		const { posted, postedTo } = conversation;
		const askedTheQuestion = '박예시에게 보낼까요?';
		let putTheQuestion: ((addressing: Addressing) => Promise<string>) | null = null;
		const turns = new InboundTurns({
			client: aClientThat(async (_message, addressing) => {
				if (!putTheQuestion) throw new Error('the test never handed over askThePerson');
				const words = await putTheQuestion(addressing);
				return `보냈습니다: ${words}`;
			}, calls, conversation),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: conversation.post,
			waitBeforeRetrying: async () => {}
		});
		putTheQuestion = (addressing) =>
			turns.askThePerson({ toolCallID: 'held-1', question: askedTheQuestion }, addressing).then((put) => put.answered);

		await turns.keep(firstKey, aRootMessage('message-7', '박예시한테 DM 보내줘'));
		await waitUntil(() => posted.includes(askedTheQuestion), 'the question to reach the requester');
		expect(postedTo[0].replyTargetID).toBe(threadOf('message-7'));

		await turns.keep(secondKey, aReplyInTheThreadOf('message-7', 'message-8', '응 보내줘'));
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

	test('a question already asked before a restart is answered by the next reply in its thread, without posting again', async () => {
		const conversation = aConversation();
		const posted = conversation.posted;
		const turns = new InboundTurns({
			client: aClientThatSays('unused', [], conversation),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: conversation.post,
			waitBeforeRetrying: async () => {}
		});
		const answering = turns.awaitAnAlreadyAskedQuestion(addressingOfTheFirstThread);

		await turns.keep(secondKey, aReplyInTheThreadOf('message-7', 'message-8', '응 보내줘'));

		expect(await answering).toBe('응 보내줘');
		expect(posted).toEqual([]);
	});

	test('after a restart, the request a held question came from does not answer that question', async () => {
		const directoryPath = directoryForOneTest();
		const askedTheQuestion = '박예시에게 보낼까요?';
		let putTheQuestion: ((addressing: Addressing) => Promise<string>) | null = null;
		const conversationBeforeTheRestart = aConversation();
		const postedBeforeTheRestart = conversationBeforeTheRestart.posted;
		const turnsBeforeTheRestart = new InboundTurns({
			client: aClientThat(async (_message, addressing) => {
				if (!putTheQuestion) throw new Error('the test never handed over askThePerson');
				await putTheQuestion(addressing);
				return new Promise<string>(() => {});
			}, [], conversationBeforeTheRestart),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: conversationBeforeTheRestart.post,
			waitBeforeRetrying: async () => {}
		});
		putTheQuestion = (addressing) =>
			turnsBeforeTheRestart
				.askThePerson({ toolCallID: 'held-1', question: askedTheQuestion }, addressing)
				.then((put) => put.answered);
		await turnsBeforeTheRestart.keep(firstKey, aRootMessage('message-7', '박예시한테 DM 보내줘'));
		await waitUntil(() => postedBeforeTheRestart.includes(askedTheQuestion), 'the question to reach the requester');

		const turnsAfterTheRestart = new InboundTurns({
			client: aClientThatSays('unused', [], aConversation()),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: aConversation().post,
			waitBeforeRetrying: async () => {}
		});
		const answering = turnsAfterTheRestart.awaitAnAlreadyAskedQuestion(addressingOfTheFirstThread);
		turnsAfterTheRestart.startDraining();
		await turnsAfterTheRestart.keep(secondKey, aReplyInTheThreadOf('message-7', 'message-8', '응 보내줘'));

		expect(await answering, 'the held question was answered by the request that asked it').toBe('응 보내줘');
	});

	test('a root message while a question is pending starts its own turn and leaves the question pending', async () => {
		const calls: AskCall[] = [];
		const conversation = aConversation();
		const posted = conversation.posted;
		const askedTheQuestion = '박예시에게 보낼까요?';
		let putTheQuestion: ((addressing: Addressing) => Promise<string>) | null = null;
		const turns = new InboundTurns({
			client: aClientThat(async (message, addressing) => {
				if (message !== '박예시한테 DM 보내줘') return `답장: ${message}`;
				if (!putTheQuestion) throw new Error('the test never handed over askThePerson');
				return `보냈습니다: ${await putTheQuestion(addressing)}`;
			}, calls, conversation),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: conversation.post,
			waitBeforeRetrying: async () => {}
		});
		putTheQuestion = (addressing) =>
			turns.askThePerson({ toolCallID: 'held-1', question: askedTheQuestion }, addressing).then((put) => put.answered);

		await turns.keep(firstKey, aRootMessage('message-7', '박예시한테 DM 보내줘'));
		await waitUntil(() => posted.includes(askedTheQuestion), 'the question to reach the requester');

		await turns.keep(secondKey, aRootMessage('message-8', '오늘 일정 알려줘'));
		await waitUntil(
			() => posted.includes('답장: 오늘 일정 알려줘') || posted.includes('보냈습니다: 오늘 일정 알려줘'),
			'the root message to be heard'
		);

		expect(posted, 'the root message was taken as the answer to the pending question').toEqual([
			askedTheQuestion,
			'답장: 오늘 일정 알려줘'
		]);
		expect(calls.map((call) => call.message)).toEqual(['박예시한테 DM 보내줘', '오늘 일정 알려줘']);

		await turns.keep(thirdKey, aReplyInTheThreadOf('message-7', 'message-9', '응 보내줘'));
		await waitUntil(() => posted.includes('보냈습니다: 응 보내줘'), 'the reply in its thread to answer the question');
		expect(calls).toHaveLength(2);
	});

	test('a reply in another thread while a question is pending starts its own turn and leaves the question pending', async () => {
		const calls: AskCall[] = [];
		const conversation = aConversation();
		const posted = conversation.posted;
		const askedTheQuestion = '박예시에게 보낼까요?';
		let putTheQuestion: ((addressing: Addressing) => Promise<string>) | null = null;
		const turns = new InboundTurns({
			client: aClientThat(async (message, addressing) => {
				if (message !== '박예시한테 DM 보내줘') return `답장: ${message}`;
				if (!putTheQuestion) throw new Error('the test never handed over askThePerson');
				return `보냈습니다: ${await putTheQuestion(addressing)}`;
			}, calls, conversation),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: conversation.post,
			waitBeforeRetrying: async () => {}
		});
		putTheQuestion = (addressing) =>
			turns.askThePerson({ toolCallID: 'held-1', question: askedTheQuestion }, addressing).then((put) => put.answered);

		await turns.keep(firstKey, aRootMessage('message-7', '박예시한테 DM 보내줘'));
		await waitUntil(() => posted.includes(askedTheQuestion), 'the question to reach the requester');

		await turns.keep(secondKey, aReplyInTheThreadOf('message-5', 'message-8', '그건 취소해줘'));
		await waitUntil(() => posted.length === 2, 'the reply in another thread to be heard');

		expect(posted, 'a reply in another thread was taken as the answer to the pending question').toEqual([
			askedTheQuestion,
			'답장: 그건 취소해줘'
		]);
		expect(calls).toHaveLength(2);
	});

	test('a reply to a pending question waits while another turn in the conversation is still running', async () => {
		const conversation = aConversation();
		const posted = conversation.posted;
		const askedTheQuestion = '박예시에게 보낼까요?';
		const otherTurn = Promise.withResolvers<string>();
		let putTheQuestion: ((addressing: Addressing) => Promise<string>) | null = null;
		const turns = new InboundTurns({
			client: aClientThat(async (message, addressing) => {
				if (message === '오늘 일정 알려줘') return otherTurn.promise;
				if (!putTheQuestion) throw new Error('the test never handed over askThePerson');
				return `보냈습니다: ${await putTheQuestion(addressing)}`;
			}, [], conversation),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: conversation.post,
			waitBeforeRetrying: async () => {}
		});
		putTheQuestion = (addressing) =>
			turns.askThePerson({ toolCallID: 'held-1', question: askedTheQuestion }, addressing).then((put) => put.answered);

		await turns.keep(firstKey, aRootMessage('message-7', '박예시한테 DM 보내줘'));
		await waitUntil(() => posted.includes(askedTheQuestion), 'the question to reach the requester');
		await turns.keep(secondKey, aRootMessage('message-8', '오늘 일정 알려줘'));
		await turns.keep(thirdKey, aReplyInTheThreadOf('message-7', 'message-9', '응 보내줘'));
		await Bun.sleep(10);

		expect(posted, 'the question was answered while another turn was speaking in the conversation').toEqual([
			askedTheQuestion
		]);

		otherTurn.resolve('오늘은 일정이 없습니다');
		await waitUntil(() => posted.includes('보냈습니다: 응 보내줘'), 'the waiting answer to reach its question');
		expect(posted).toEqual([askedTheQuestion, '오늘은 일정이 없습니다', '보냈습니다: 응 보내줘']);
	});

	test('a turn the agent keeps refusing is retried and then dropped by name, with its reason', async () => {
		const directoryPath = directoryForOneTest();
		const reported: string[] = [];
		let attempted = 0;
		const client = {
			ask: async (): Promise<StopReason> => {
				attempted += 1;
				throw new RequestError(-32603, 'Internal error', { error: 'a prompt with no text is nothing to answer' });
			}
		};
		const turns = new InboundTurns({
			client: client as unknown as BlueclawACPClient,
			queue: new InboundQueue({ directoryPath, attemptCeiling: 2 }),
			postToConversation: aConversation().post,
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
		expect(dropped).toContain('a prompt with no text is nothing to answer');
		expect(await eventsStillOnDisk(directoryPath)).toEqual([]);
	});

	test('a message the agent cannot be reached for outlasts the attempt ceiling and is answered once it is back', async () => {
		const directoryPath = directoryForOneTest();
		const reported: string[] = [];
		const conversation = aConversation();
		const posted = conversation.posted;
		const delays: number[] = [];
		let attempted = 0;
		const client = {
			ask: async (): Promise<StopReason> => {
				attempted += 1;
				if (attempted <= 5) throw new AgentUnreachable(new Error('connect ENOENT /run/internkim/acp/blueclaw-acp.sock'));
				await conversation.post(addressingOfTheFirstThread, '받았습니다');
				return 'end_turn';
			}
		};
		const turns = new InboundTurns({
			client: client as unknown as BlueclawACPClient,
			queue: new InboundQueue({ directoryPath, attemptCeiling: 2 }),
			postToConversation: conversation.post,
			waitBeforeRetrying: async (milliseconds) => {
				delays.push(milliseconds);
			},
			report: (line) => reported.push(line)
		});

		await turns.keep(firstKey, aChatdBody());
		await waitUntil(() => posted.length > 0, 'the reply once the agent is back');
		await turns.settled();

		expect(posted).toEqual(['받았습니다']);
		expect(reported.filter((line) => line.startsWith('dropped'))).toEqual([]);
		expect(delays).toEqual([250, 500, 1_000, 2_000, 4_000]);
		expect(await eventsStillOnDisk(directoryPath)).toEqual([]);
	});

	test('an event a stopped relay never answered is delivered once by the relay that follows it', async () => {
		const directoryPath = directoryForOneTest();
		const neverAnswered: AskCall[] = [];
		const beforeRestart = new InboundTurns({
			client: aClientThat(() => new Promise<string>(() => {}), neverAnswered, aConversation()),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: aConversation().post,
			waitBeforeRetrying: async () => {}
		});
		await beforeRestart.keep(firstKey, aChatdBody());
		await waitUntil(() => neverAnswered.length === 1, 'the stopped relay to start its turn');

		const calls: AskCall[] = [];
		const conversation = aConversation();
		const posted = conversation.posted;
		const afterRestart = new InboundTurns({
			client: aClientThatSays('보냈습니다', calls, conversation),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: conversation.post,
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
			client: aClientThatSays('보냈습니다', calls, aConversation()),
			queue: new InboundQueue({ directoryPath }),
			postToConversation: aConversation().post,
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

	test('a question the conversation refuses is not left waiting for an answer', async () => {
		const calls: AskCall[] = [];
		const conversation = aConversation();
		const refusingConversation = aConversation('chatd refused the post with 503');
		const questionFailures: unknown[] = [];
		let putTheQuestion: ((addressing: Addressing) => Promise<unknown>) | null = null;
		const turns = new InboundTurns({
			client: aClientThat(async (message, addressing) => {
				if (message !== '박예시한테 DM 보내줘') return `답장: ${message}`;
				if (!putTheQuestion) throw new Error('the test never handed over askThePerson');
				await putTheQuestion(addressing).catch((failure: unknown) => questionFailures.push(failure));
				return '';
			}, calls, conversation),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			postToConversation: refusingConversation.post,
			waitBeforeRetrying: async () => {}
		});
		putTheQuestion = (addressing) =>
			turns.askThePerson({ toolCallID: 'held-1', question: '박예시에게 보낼까요?' }, addressing);

		await turns.keep(firstKey, aRootMessage('message-7', '박예시한테 DM 보내줘'));
		await waitUntil(() => questionFailures.length === 1, 'the refused question to fail');
		await turns.keep(secondKey, aReplyInTheThreadOf('message-7', 'message-8', '응 보내줘'));
		await waitUntil(() => calls.length === 2, 'the reply in the thread to be heard');

		expect(
			conversation.posted,
			'a reply in the thread of a question nobody saw was swallowed as its answer'
		).toEqual(['답장: 응 보내줘']);
	});
});
