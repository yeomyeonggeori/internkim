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

type AnswerTheTurn = (message: string, addressing: Addressing, facts: MessageFacts | undefined) => Promise<string>;

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

type OpenPermission = {
	isOpen: boolean;
	judged: string[];
	parkedMessageIDs: Set<string>;
	answered: PromiseWithResolvers<string>;
	isAnswer: (reply: string) => boolean;
};

function aPermissionAnsweredBy(isAnswer: (reply: string) => boolean): OpenPermission {
	return { isOpen: false, judged: [], parkedMessageIDs: new Set(), answered: Promise.withResolvers<string>(), isAnswer };
}

function aClientThat(
	answer: AnswerTheTurn,
	calls: AskCall[],
	conversation: Conversation,
	permission: OpenPermission = aPermissionAnsweredBy(() => false)
): BlueclawACPClient {
	const client = {
		hasOpenPermissionIn: (): boolean => permission.isOpen,
		isParkedOnPermission: (_conversationID: string, messageID: string): boolean =>
			permission.isOpen && permission.parkedMessageIDs.has(messageID),
		answerOpenPermission: async (_addressing: Addressing, _messageID: string, reply: string): Promise<boolean> => {
			permission.judged.push(reply);
			if (!permission.isAnswer(reply)) return false;
			permission.isOpen = false;
			permission.answered.resolve(reply);
			return true;
		},
		ask: async (
			requester: Requester,
			addressing: Addressing,
			message: string,
			facts?: MessageFacts
		): Promise<StopReason> => {
			calls.push({ requester, addressing, message, facts });
			const reply = await answer(message, addressing, facts);
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

	const askedTheQuestion = '박예시에게 보낼까요?';
	const messageThatAsks = '박예시한테 DM 보내줘';

	function aTurnsThatAskAndWait(calls: AskCall[], permission: OpenPermission, otherTurn?: Promise<string>) {
		const conversation = aConversation();
		const turns: InboundTurns = new InboundTurns({
			client: aClientThat(
				async (message, addressing, facts) => {
					if (message !== messageThatAsks) return otherTurn ? `${await otherTurn}: ${message}` : `답장: ${message}`;
					await conversation.post(addressing, askedTheQuestion);
					if (facts) permission.parkedMessageIDs.add(facts.messageID);
					permission.isOpen = true;
					turns.handTheRunToBlueclaw(addressing.conversationID);
					return `보냈습니다: ${await permission.answered.promise}`;
				},
				calls,
				conversation,
				permission
			),
			queue: new InboundQueue({ directoryPath: directoryForOneTest() }),
			waitBeforeRetrying: async () => {}
		});
		return { turns, conversation };
	}

	test('a message in a conversation with no open permission is never put to blueclaw as an answer', async () => {
		const permission = aPermissionAnsweredBy(() => true);
		const { turns, conversation } = aTurnsThatAskAndWait([], permission);

		await turns.keep(firstKey, aRootMessage('message-7', '오늘 일정 알려줘'));
		await waitUntil(() => conversation.posted.length === 1, 'the message to be heard');

		expect(permission.judged).toEqual([]);
	});

	test('a reply blueclaw calls an answer resolves the permission and does not start a turn', async () => {
		const calls: AskCall[] = [];
		const permission = aPermissionAnsweredBy((reply) => reply === '응 보내줘');
		const { turns, conversation } = aTurnsThatAskAndWait(calls, permission);

		await turns.keep(firstKey, aRootMessage('message-7', messageThatAsks));
		await waitUntil(() => permission.isOpen, 'the permission to open');
		await turns.keep(secondKey, aReplyInTheThreadOf('message-7', 'message-8', '응 보내줘'));
		await waitUntil(() => conversation.posted.length === 2, 'the turn to finish on the answer');

		expect(calls, 'the answer started a turn of its own').toHaveLength(1);
		expect(conversation.posted).toEqual([askedTheQuestion, '보냈습니다: 응 보내줘']);
		expect(permission.judged).toEqual(['응 보내줘']);
	});

	test('a reply blueclaw calls not an answer starts a turn while the permission stays open', async () => {
		const calls: AskCall[] = [];
		const permission = aPermissionAnsweredBy((reply) => reply === '응 보내줘');
		const { turns, conversation } = aTurnsThatAskAndWait(calls, permission);

		await turns.keep(firstKey, aRootMessage('message-7', messageThatAsks));
		await waitUntil(() => permission.isOpen, 'the permission to open');
		await turns.keep(secondKey, aReplyInTheThreadOf('message-7', 'message-8', '오늘 일정 알려줘'));
		await waitUntil(() => conversation.posted.includes('답장: 오늘 일정 알려줘'), 'the message to start its own turn');

		expect(permission.isOpen, 'the permission was closed by a message that was not its answer').toBe(true);
		expect(calls.map((call) => call.message)).toEqual([messageThatAsks, '오늘 일정 알려줘']);

		await turns.keep(thirdKey, aReplyInTheThreadOf('message-7', 'message-9', '응 보내줘'));
		await waitUntil(() => conversation.posted.includes('보냈습니다: 응 보내줘'), 'the permission to be answered');
		expect(calls).toHaveLength(2);
	});

	test('a message blueclaw called not an answer is never put to it again, also by the relay that follows', async () => {
		const directoryPath = directoryForOneTest();
		const permission = aPermissionAnsweredBy(() => false);
		permission.isOpen = true;
		const otherTurn = Promise.withResolvers<string>();
		const firstConversation = aConversation();
		const beforeTheRestart = new InboundTurns({
			client: aClientThat(() => otherTurn.promise, [], firstConversation, permission),
			queue: new InboundQueue({ directoryPath }),
			waitBeforeRetrying: async () => {}
		});
		await beforeTheRestart.keep(firstKey, aRootMessage('message-7', '오늘 일정 알려줘'));
		await waitUntil(() => permission.judged.length === 1, 'the message to be judged');
		await waitUntil(async () => (await new InboundQueue({ directoryPath }).undelivered())[0]?.isPrompt === true, 'the verdict to reach the disk');

		const afterTheRestart = new InboundTurns({
			client: aClientThat(async () => '', [], aConversation(), permission),
			queue: new InboundQueue({ directoryPath }),
			waitBeforeRetrying: async () => {}
		});
		afterTheRestart.startDraining();
		await afterTheRestart.settled();

		expect(permission.judged, 'the relay that followed asked blueclaw about the same message again').toEqual(['오늘 일정 알려줘']);
		otherTurn.resolve('');
	});

	test('a message that is not an answer waits behind the running turn and is judged once', async () => {
		const calls: AskCall[] = [];
		const permission = aPermissionAnsweredBy(() => false);
		const otherTurn = Promise.withResolvers<string>();
		const { turns, conversation } = aTurnsThatAskAndWait(calls, permission, otherTurn.promise);

		await turns.keep(firstKey, aRootMessage('message-7', messageThatAsks));
		await waitUntil(() => permission.isOpen, 'the permission to open');
		await turns.keep(secondKey, aRootMessage('message-8', '오늘 일정 알려줘'));
		await waitUntil(() => calls.length === 2, 'the second message to start its turn');
		await turns.keep(thirdKey, aRootMessage('message-9', '내일 일정도 알려줘'));
		await Bun.sleep(10);

		expect(calls, 'a second turn started in the conversation while one was still running').toHaveLength(2);
		expect(permission.judged).toEqual(['오늘 일정 알려줘', '내일 일정도 알려줘']);

		otherTurn.resolve('일정');
		await waitUntil(() => calls.length === 3, 'the waiting message to start its turn');
		expect(permission.judged).toEqual(['오늘 일정 알려줘', '내일 일정도 알려줘']);
		expect(conversation.posted).toContain('일정: 내일 일정도 알려줘');
	});

	test('a reply blueclaw calls an answer resolves the permission without waiting for the running turn', async () => {
		const calls: AskCall[] = [];
		const permission = aPermissionAnsweredBy((reply) => reply === '응 보내줘');
		const otherTurn = Promise.withResolvers<string>();
		const { turns, conversation } = aTurnsThatAskAndWait(calls, permission, otherTurn.promise);

		await turns.keep(firstKey, aRootMessage('message-7', messageThatAsks));
		await waitUntil(() => permission.isOpen, 'the permission to open');
		await turns.keep(secondKey, aRootMessage('message-8', '오늘 일정 알려줘'));
		await waitUntil(() => calls.length === 2, 'the other turn to start');
		await turns.keep(thirdKey, aReplyInTheThreadOf('message-7', 'message-9', '응 보내줘'));
		await waitUntil(() => conversation.posted.includes('보냈습니다: 응 보내줘'), 'the answer to resolve the permission');

		expect(permission.isOpen).toBe(false);
		expect(conversation.posted).not.toContain('일정: 오늘 일정 알려줘');
		expect(calls, 'the answer started a turn of its own').toHaveLength(2);

		otherTurn.resolve('일정');
	});

	test('a reply blueclaw calls not an answer still waits for the running turn, and the permission stays open', async () => {
		const calls: AskCall[] = [];
		const permission = aPermissionAnsweredBy((reply) => reply === '응 보내줘');
		const otherTurn = Promise.withResolvers<string>();
		const { turns } = aTurnsThatAskAndWait(calls, permission, otherTurn.promise);

		await turns.keep(firstKey, aRootMessage('message-7', messageThatAsks));
		await waitUntil(() => permission.isOpen, 'the permission to open');
		await turns.keep(secondKey, aRootMessage('message-8', '오늘 일정 알려줘'));
		await waitUntil(() => calls.length === 2, 'the other turn to start');
		await turns.keep(thirdKey, aReplyInTheThreadOf('message-7', 'message-9', '내일 일정도 알려줘'));
		await waitUntil(() => permission.judged.includes('내일 일정도 알려줘'), 'the reply to be judged');
		await Bun.sleep(10);

		expect(permission.isOpen).toBe(true);
		expect(calls, 'a second turn started while one was still running').toHaveLength(2);

		otherTurn.resolve('일정');
		await waitUntil(() => calls.length === 3, 'the waiting reply to start its turn');
	});

	test('a turn the agent keeps refusing is retried and then dropped by name, with its reason', async () => {
		const directoryPath = directoryForOneTest();
		const reported: string[] = [];
		let attempted = 0;
		const client = {
			hasOpenPermissionIn: (): boolean => false,
			isParkedOnPermission: (): boolean => false,
			ask: async (): Promise<StopReason> => {
				attempted += 1;
				throw new RequestError(-32603, 'Internal error', { error: 'a prompt with no text is nothing to answer' });
			}
		};
		const turns = new InboundTurns({
			client: client as unknown as BlueclawACPClient,
			queue: new InboundQueue({ directoryPath, attemptCeiling: 2 }),
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
			hasOpenPermissionIn: (): boolean => false,
			isParkedOnPermission: (): boolean => false,
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
