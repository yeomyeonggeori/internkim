import { afterEach, expect, test } from 'bun:test';
import {
	AgentSideConnection,
	ndJsonStream,
	PROTOCOL_VERSION,
	RequestError,
	type Agent,
	type ContentBlock,
	type LoadSessionRequest,
	type NewSessionRequest,
	type PromptRequest,
	type RequestPermissionResponse
} from '@agentclientprotocol/sdk';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import {
	AgentUnreachable,
	approvalReplyExtensionMethod,
	BlueclawACPClient,
	deliveredExtensionMethod,
	deliveryMetaKey,
	messageMetaKey,
	sessionMetaKey,
	undeliveredExtensionMethod,
	type Addressing,
	type Delivery,
	type MessageFacts,
	type PutQuestion
} from './acp-session';
import { HeldQuestionStore } from './held-question-store';
import { agentFilePoster } from './conversation-post';
import type { KeptAttachment, WorkspaceFile } from './file-transfer';

function aQuestionStore(directoryPath?: string): HeldQuestionStore {
	return new HeldQuestionStore({ directoryPath: directoryPath ?? mkdtempSync(join(tmpdir(), 'acp-questions-')) });
}

function neverAskedAgain(): (addressing: Addressing) => Promise<string> {
	return () => new Promise<string>(() => {});
}

type DeliveryReport = { method: string; params: Record<string, unknown> };

type AnAgentThatRecords = {
	socketPath: string;
	sessionsOpened: NewSessionRequest[];
	sessionsLoaded: LoadSessionRequest[];
	promptsTaken: PromptRequest[];
	approvalRepliesRead: string[];
	deliveryReports: DeliveryReport[];
	connectionsOpened: number;
	reissuePermission: (sessionID: string, toolCallID: string, question: string) => Promise<RequestPermissionResponse>;
	askWithNoTurnOpen: (sessionID: string, toolCallID: string, question: string, delivery: Delivery) => Promise<RequestPermissionResponse>;
	speakWithNoTurnOpen: (sessionID: string, content: ContentBlock, delivery: Delivery) => Promise<void>;
};

type AgentBehaviour = {
	reply?: string;
	replyFor?: (prompt: string) => string;
	askPermissionAbout?: { toolCallID: string; question: string; onlyWhenAskedTo?: string };
	approvalReplies?: { reply: string; optionID: string }[];
	refuseSessionsWith?: string;
};

const cleanUps: (() => void)[] = [];

afterEach(() => {
	while (cleanUps.length) cleanUps.pop()?.();
});

function anAgentOnASocket(behaviour: AgentBehaviour): AnAgentThatRecords {
	const directory = mkdtempSync(join(tmpdir(), 'acp-relay-'));
	const socketPath = join(directory, 'agent.sock');
	const sessionsOpened: NewSessionRequest[] = [];
	const sessionsLoaded: LoadSessionRequest[] = [];
	const promptsTaken: PromptRequest[] = [];
	const approvalRepliesRead: string[] = [];
	const deliveryReports: DeliveryReport[] = [];
	let repliesSent = 0;
	let connectionsOpened = 0;
	let latestConnection: AgentSideConnection | null = null;

	const server = Bun.listen<{ deliver: (chunk: Uint8Array) => void }>({
		unix: socketPath,
		socket: {
			open(socket) {
				let deliver: (chunk: Uint8Array) => void = () => {};
				const readable = new ReadableStream<Uint8Array>({
					start(controller) {
						deliver = (chunk) => controller.enqueue(chunk);
					}
				});
				const writable = new WritableStream<Uint8Array>({
					write(chunk) {
						socket.write(chunk);
					}
				});
				socket.data = { deliver };
				let held: AgentSideConnection | null = null;
				held = new AgentSideConnection(
					() => theAgent(() => held),
					ndJsonStream(writable, readable)
				);
				connectionsOpened += 1;
				latestConnection = held;
			},
			data(socket, chunk) {
				socket.data.deliver(new Uint8Array(chunk));
			}
		}
	});
	cleanUps.push(() => {
		server.stop(true);
		rmSync(directory, { recursive: true, force: true });
	});

	function theAgent(connectionOf: () => AgentSideConnection | null): Agent {
		return {
			initialize: async () => ({
				protocolVersion: PROTOCOL_VERSION,
				agentCapabilities: { mcpCapabilities: { http: true } },
				authMethods: []
			}),
			newSession: async (request: NewSessionRequest) => {
				sessionsOpened.push(request);
				if (behaviour.refuseSessionsWith) {
					throw new RequestError(-32603, 'Internal error', { error: behaviour.refuseSessionsWith });
				}
				return { sessionId: 'session-1' };
			},
			loadSession: async (request: LoadSessionRequest) => {
				sessionsLoaded.push(request);
			},
			prompt: async (request: PromptRequest) => {
				promptsTaken.push(request);
				const connection = connectionOf();
				if (!connection) throw new Error('the agent has no connection to answer on');
				const prompt = promptTextOf(request);
				const asking = behaviour.askPermissionAbout;
				if (asking && (asking.onlyWhenAskedTo ?? prompt) === prompt) {
					await connection.requestPermission({
						sessionId: request.sessionId,
						toolCall: {
							toolCallId: asking.toolCallID,
							title: asking.question
						},
						options: [
							{ optionId: 'approve_once', kind: 'allow_once', name: 'approve this call' },
							{ optionId: 'reject_once', kind: 'reject_once', name: 'decline this call' }
						],
						_meta: {
							[deliveryMetaKey]: { deliveryID: `question-${asking.toolCallID}`, replyTargetID: replyTargetOf(request) }
						}
					});
				}
				const reply = behaviour.replyFor?.(prompt) ?? behaviour.reply;
				if (reply) {
					repliesSent += 1;
					await connection.sessionUpdate({
						sessionId: request.sessionId,
						update: {
							sessionUpdate: 'agent_message_chunk',
							content: { type: 'text', text: reply }
						},
						_meta: {
							[deliveryMetaKey]: { deliveryID: `reply-${repliesSent}`, replyTargetID: replyTargetOf(request) }
						}
					});
					await reportFor(`reply-${repliesSent}`);
				}
				return { stopReason: 'end_turn' };
			},
			cancel: async () => {},
			authenticate: async () => ({}),
			extMethod: async (method: string, params: Record<string, unknown>) => {
				if (method !== approvalReplyExtensionMethod) {
					deliveryReports.push({ method, params });
					return {};
				}
				const reply = String(params.reply ?? '');
				approvalRepliesRead.push(reply);
				const known = behaviour.approvalReplies?.find((answer) => answer.reply === reply);
				return { optionId: known?.optionID ?? 'reject_once' };
			}
		};
	}

	async function reportFor(deliveryID: string): Promise<void> {
		await waitUntil(
			() => deliveryReports.some((report) => report.params.deliveryID === deliveryID),
			`the relay to say what became of ${deliveryID}`
		);
	}

	return {
		socketPath,
		sessionsOpened,
		sessionsLoaded,
		promptsTaken,
		approvalRepliesRead,
		deliveryReports,
		get connectionsOpened() {
			return connectionsOpened;
		},
		reissuePermission: (sessionID, toolCallID, question) => {
			if (!latestConnection) throw new Error('no connection to the agent yet');
			return latestConnection.requestPermission({
				sessionId: sessionID,
				toolCall: { toolCallId: toolCallID, title: question },
				options: [
					{ optionId: 'approve_once', kind: 'allow_once', name: 'approve this call' },
					{ optionId: 'reject_once', kind: 'reject_once', name: 'decline this call' }
				]
			});
		},
		askWithNoTurnOpen: (sessionID, toolCallID, question, delivery) => {
			if (!latestConnection) throw new Error('no connection to the agent yet');
			return latestConnection.requestPermission({
				sessionId: sessionID,
				toolCall: { toolCallId: toolCallID, title: question },
				options: [
					{ optionId: 'approve_once', kind: 'allow_once', name: 'approve this call' },
					{ optionId: 'reject_once', kind: 'reject_once', name: 'decline this call' }
				],
				_meta: { [deliveryMetaKey]: delivery }
			});
		},
		speakWithNoTurnOpen: async (sessionID, content, delivery) => {
			if (!latestConnection) throw new Error('no connection to the agent yet');
			await latestConnection.sessionUpdate({
				sessionId: sessionID,
				update: { sessionUpdate: 'agent_message_chunk', content },
				_meta: { [deliveryMetaKey]: delivery }
			});
		}
	};
}

function replyTargetOf(request: PromptRequest): string | undefined {
	const message = request._meta?.[messageMetaKey];
	if (typeof message !== 'object' || message === null || !('replyTargetID' in message)) return undefined;
	return typeof message.replyTargetID === 'string' ? message.replyTargetID : undefined;
}

function promptTextOf(request: PromptRequest): string {
	const first = request.prompt[0];
	return first?.type === 'text' ? first.text : '';
}

async function waitUntil(isReady: () => boolean | Promise<boolean>, waitedFor: string): Promise<void> {
	const deadline = Date.now() + 3_000;
	while (Date.now() < deadline) {
		if (await isReady()) return;
		await Bun.sleep(1);
	}
	throw new Error(`waited too long for ${waitedFor}`);
}

type Posted = { addressing: Addressing; message: string; attachments?: KeptAttachment[] };

function aConversation(refusal?: string) {
	const posted: Posted[] = [];
	return {
		posted,
		post: async (addressing: Addressing, message: string, attachments?: KeptAttachment[]): Promise<string> => {
			if (refusal) throw new Error(refusal);
			posted.push({ addressing, message, ...(attachments ? { attachments } : {}) });
			return `posted-${posted.length}`;
		}
	};
}

async function noFileExpected(): Promise<string> {
	throw new Error('no file was expected in this conversation');
}

type HandedOver = { requesterEmail: string; file: WorkspaceFile };

function aMessengerKeeping(refusal?: string) {
	const handedOver: HandedOver[] = [];
	return {
		handedOver,
		keep: async (requesterEmail: string, file: WorkspaceFile): Promise<KeptAttachment> => {
			handedOver.push({ requesterEmail, file });
			if (refusal) throw new Error(refusal);
			return { filename: file.filename, contentType: file.contentType, address: 'http://127.0.0.1:3000/media/9f2c.pdf', digest: '9f2c', sizeBytes: 2048 };
		}
	};
}

function putWith(words: string | Promise<string>): PutQuestion {
	return { messageID: 'posted-question', answered: Promise.resolve(words) };
}

function factsIn(addressing: Addressing, messageID: string): MessageFacts {
	return { messageID, replyTargetID: addressing.replyTargetID, isThread: addressing.isThread, context: {} };
}

const sampleRequester = { email: 'sample@example.test', name: '이샘플' };
const sampleAddressing = { platform: 'buzz', conversationID: 'conversation-1' };

test('a session names the requester and the conversation it answers in', async () => {
	const agent = anAgentOnASocket({ reply: '보냈습니다' });
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());

	await client.ask(sampleRequester, sampleAddressing, '박예시한테 DM 보내줘');

	expect(agent.sessionsOpened).toHaveLength(1);
	const carried = agent.sessionsOpened[0]._meta?.[sessionMetaKey];
	expect(carried).toEqual({ requester: sampleRequester, addressing: sampleAddressing });
	expect(agent.promptsTaken[0].prompt[0]).toEqual({ type: 'text', text: '박예시한테 DM 보내줘' });
});

test('a second message in the same conversation reuses the session', async () => {
	const agent = anAgentOnASocket({ reply: '네' });
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());

	await client.ask(sampleRequester, sampleAddressing, '첫 번째');
	await client.ask(sampleRequester, sampleAddressing, '두 번째');

	expect(agent.sessionsOpened).toHaveLength(1);
	expect(agent.promptsTaken).toHaveLength(2);
});

test('the person is asked, and the agent reads what they wrote', async () => {
	const agent = anAgentOnASocket({
		reply: '보냈습니다',
		askPermissionAbout: { toolCallID: 'held-1', question: '박예시에게 보낼까요?' },
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }]
	});
	const asked: string[] = [];
	const questions = aQuestionStore();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions,
		askThePerson: async (question) => {
			asked.push(question.question);
			return putWith('응 보내줘');
		},
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());

	await client.ask(sampleRequester, sampleAddressing, '박예시한테 DM 보내줘');

	expect(asked).toEqual(['박예시에게 보낼까요?']);
	// The relay carries the words and reads none of them: the option comes back
	// from the agent, which is the only side allowed to judge what they meant.
	expect(agent.approvalRepliesRead).toEqual(['응 보내줘']);
	// Delivered to blueclaw, so nothing is left for a restart to find.
	expect(await questions.read('held-1')).toBeNull();
});

test('an already-answered question survives a restart and is delivered without asking again', async () => {
	const agent = anAgentOnASocket({
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }]
	});
	const questions = aQuestionStore();
	await questions.keep({
		toolCallID: 'held-1',
		sessionID: 'session-1',
		requester: sampleRequester,
		addressing: sampleAddressing,
		question: '박예시에게 보낼까요?',
		askedAt: new Date().toISOString()
	});
	await questions.answer('held-1', '응 보내줘');

	const asked: string[] = [];
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions,
		askThePerson: async (question) => {
			asked.push(question.question);
			return putWith('not what a restart should ask again');
		},
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());

	await client.restoreOutstandingQuestions();
	await waitUntil(() => agent.connectionsOpened > 0, 'the relay to reconnect after the restart');

	const answered = await agent.reissuePermission('session-1', 'held-1', '박예시에게 보낼까요?');

	expect(asked).toEqual([]);
	expect(answered.outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(agent.approvalRepliesRead).toEqual(['응 보내줘']);
	await waitUntil(async () => (await questions.read('held-1')) === null, 'the delivered question to be forgotten');
});

test('an unanswered question is not asked again after a restart, and is delivered once the person answers', async () => {
	const agent = anAgentOnASocket({
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }]
	});
	const questions = aQuestionStore();
	await questions.keep({
		toolCallID: 'held-1',
		sessionID: 'session-1',
		requester: sampleRequester,
		addressing: sampleAddressing,
		question: '박예시에게 보낼까요?',
		askedAt: new Date().toISOString()
	});

	const asked: string[] = [];
	const answerTheQuestion: { resolve: ((words: string) => void) | null } = { resolve: null };
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions,
		askThePerson: async (question) => {
			asked.push(question.question);
			return putWith('not what a restart should ask again');
		},
		awaitAnAlreadyAskedQuestion: () =>
			new Promise<string>((resolve) => {
				answerTheQuestion.resolve = resolve;
			})
	});
	cleanUps.push(() => client.close());

	await client.restoreOutstandingQuestions();
	await waitUntil(() => answerTheQuestion.resolve !== null, 'the relay to wait on the question already asked');
	await waitUntil(() => agent.connectionsOpened > 0, 'the relay to reconnect after the restart');

	const answering = agent.reissuePermission('session-1', 'held-1', '박예시에게 보낼까요?');
	await Bun.sleep(10);
	expect(asked, 'the relay asked the person again instead of waiting').toEqual([]);

	answerTheQuestion.resolve?.('응 보내줘');
	const answered = await answering;

	expect(answered.outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(agent.approvalRepliesRead).toEqual(['응 보내줘']);
	await waitUntil(async () => (await questions.read('held-1')) === null, 'the delivered question to be forgotten');
});

test('a question is put in the thread of the message whose turn asked it, not the one that opened the session', async () => {
	const agent = anAgentOnASocket({
		reply: '보냈습니다',
		askPermissionAbout: { toolCallID: 'held-1', question: '박예시에게 보낼까요?', onlyWhenAskedTo: '박예시한테 DM 보내줘' },
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }]
	});
	const askedIn: Addressing[] = [];
	const keptIn: (Addressing | undefined)[] = [];
	const questions = aQuestionStore();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions,
		askThePerson: async (_question, addressing) => {
			askedIn.push(addressing);
			keptIn.push((await questions.read('held-1'))?.addressing);
			return putWith('응 보내줘');
		},
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	const firstThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-1', isThread: false };
	const laterThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-2', isThread: false };

	await client.ask(sampleRequester, firstThread, '안녕하세요', factsIn(firstThread, 'message-1'));
	await client.ask(sampleRequester, laterThread, '박예시한테 DM 보내줘', factsIn(laterThread, 'message-2'));

	expect(agent.sessionsOpened).toHaveLength(1);
	expect(askedIn.map((addressing) => addressing.replyTargetID)).toEqual([laterThread.replyTargetID]);
	expect(keptIn.map((addressing) => addressing?.replyTargetID)).toEqual([laterThread.replyTargetID]);
});

test('each reply is posted in the thread it names, even while another turn waits on a question', async () => {
	const agent = anAgentOnASocket({
		replyFor: (prompt) => `${prompt}에 답합니다`,
		askPermissionAbout: { toolCallID: 'held-1', question: '박예시에게 보낼까요?', onlyWhenAskedTo: '박예시한테 DM 보내줘' },
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }]
	});
	const answer = Promise.withResolvers<string>();
	let isAsked = false;
	const conversation = aConversation();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => {
			isAsked = true;
			return putWith(answer.promise);
		},
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	const askingThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-1' };
	const laterThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-2' };

	const waiting = client.ask(sampleRequester, askingThread, '박예시한테 DM 보내줘', factsIn(askingThread, 'message-1'));
	await waitUntil(() => isAsked, 'the first turn to ask its question');
	await client.ask(sampleRequester, laterThread, '오늘 일정 알려줘', factsIn(laterThread, 'message-2'));
	answer.resolve('응 보내줘');
	await waiting;

	expect(
		conversation.posted.map((posted) => [posted.addressing.replyTargetID, posted.message]),
		'a reply was posted in a thread other than the one its turn named'
	).toEqual([
		[laterThread.replyTargetID, '오늘 일정 알려줘에 답합니다'],
		[askingThread.replyTargetID, '박예시한테 DM 보내줘에 답합니다']
	]);
});

test('a reply is posted as it arrives, and the agent is told once which message it became', async () => {
	const agent = anAgentOnASocket({ reply: '보냈습니다' });
	const conversation = aConversation();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	const thread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-7' };

	await client.ask(sampleRequester, thread, '박예시한테 DM 보내줘', factsIn(thread, 'message-7'));

	expect(conversation.posted).toEqual([{ addressing: thread, message: '보냈습니다' }]);
	expect(agent.deliveryReports).toEqual([
		{ method: deliveredExtensionMethod, params: { deliveryID: 'reply-1', messageID: 'posted-1' } }
	]);
});

test('a reply the conversation refuses is reported undelivered with the reason, never as delivered', async () => {
	const agent = anAgentOnASocket({ reply: '보냈습니다' });
	const conversation = aConversation('chatd refused the post to buzz:conversation-1:message-7 with 503');
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	const thread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-7' };

	await client.ask(sampleRequester, thread, '박예시한테 DM 보내줘', factsIn(thread, 'message-7'));

	expect(agent.deliveryReports.map((report) => report.method)).toEqual([undeliveredExtensionMethod]);
	expect(agent.deliveryReports[0].params.deliveryID).toBe('reply-1');
	expect(String(agent.deliveryReports[0].params.reason)).toContain('with 503');
});

test('a reply that arrives with no turn open is posted in the thread it names and reported', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	await client.ask(sampleRequester, sampleAddressing, '안녕하세요');

	await agent.speakWithNoTurnOpen(
		'session-1',
		{ type: 'text', text: '승인하신 메시지를 보냈습니다' },
		{ deliveryID: 'resumed-1', replyTargetID: 'buzz:conversation-1:message-7' }
	);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the reply');

	expect(conversation.posted).toEqual([
		{
			addressing: { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-7' },
			message: '승인하신 메시지를 보냈습니다'
		}
	]);
	expect(agent.deliveryReports).toEqual([
		{ method: deliveredExtensionMethod, params: { deliveryID: 'resumed-1', messageID: 'posted-1' } }
	]);
});

const agentDeckLink: ContentBlock = {
	type: 'resource_link',
	name: '분기 보고.pdf',
	uri: 'file:///workspace/private/people/person-1/%EB%B6%84%EA%B8%B0%20%EB%B3%B4%EA%B3%A0.pdf',
	mimeType: 'application/pdf',
	size: 2048
};

function aClientPostingFilesThrough(socketPath: string, messenger: ReturnType<typeof aMessengerKeeping>, conversation: ReturnType<typeof aConversation>) {
	const client = new BlueclawACPClient({
		socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: agentFilePoster({ keepForTheMessenger: messenger.keep, postToConversation: conversation.post }),
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	return client;
}

test('a file the agent replies with is kept by the messenger as the person it answers, posted, and reported delivered as that message', async () => {
	const agent = anAgentOnASocket({});
	const messenger = aMessengerKeeping();
	const conversation = aConversation();
	const client = aClientPostingFilesThrough(agent.socketPath, messenger, conversation);
	await client.ask(sampleRequester, sampleAddressing, '한 장짜리 PDF 만들어줘');

	await agent.speakWithNoTurnOpen('session-1', agentDeckLink, { deliveryID: 'file-1', replyTargetID: 'buzz:conversation-1:message-7' });
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the file');

	expect(messenger.handedOver).toEqual([
		{
			requesterEmail: 'sample@example.test',
			file: { filename: '분기 보고.pdf', workspacePath: '/workspace/private/people/person-1/분기 보고.pdf', contentType: 'application/pdf' }
		}
	]);
	expect(conversation.posted).toEqual([
		{
			addressing: { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-7' },
			message: '',
			attachments: [{ filename: '분기 보고.pdf', contentType: 'application/pdf', address: 'http://127.0.0.1:3000/media/9f2c.pdf', digest: '9f2c', sizeBytes: 2048 }]
		}
	]);
	expect(agent.deliveryReports).toEqual([
		{ method: deliveredExtensionMethod, params: { deliveryID: 'file-1', messageID: 'posted-1' } }
	]);
});

test('a file the messenger will not keep is reported undelivered with its reason, and nothing is posted', async () => {
	const agent = anAgentOnASocket({});
	const messenger = aMessengerKeeping('the messenger refused 분기 보고.pdf with 413');
	const conversation = aConversation();
	const client = aClientPostingFilesThrough(agent.socketPath, messenger, conversation);
	await client.ask(sampleRequester, sampleAddressing, '한 장짜리 PDF 만들어줘');

	await agent.speakWithNoTurnOpen('session-1', agentDeckLink, { deliveryID: 'file-1' });
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the file');

	expect(conversation.posted).toEqual([]);
	expect(agent.deliveryReports.map((report) => report.method)).toEqual([undeliveredExtensionMethod]);
	expect(String(agent.deliveryReports[0].params.reason)).toContain('refused 분기 보고.pdf with 413');
});

test('a file named by anything but a path on this computer is reported undelivered, and never handed to the messenger', async () => {
	const agent = anAgentOnASocket({});
	const messenger = aMessengerKeeping();
	const client = aClientPostingFilesThrough(agent.socketPath, messenger, aConversation());
	await client.ask(sampleRequester, sampleAddressing, '한 장짜리 PDF 만들어줘');

	await agent.speakWithNoTurnOpen(
		'session-1',
		{ type: 'resource_link', name: 'deck.pdf', uri: 'https://files.example.com/deck.pdf' },
		{ deliveryID: 'file-1' }
	);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the file');

	expect(messenger.handedOver).toEqual([]);
	expect(agent.deliveryReports.map((report) => report.method)).toEqual([undeliveredExtensionMethod]);
	expect(String(agent.deliveryReports[0].params.reason)).toContain('https://files.example.com/deck.pdf');
});

test('what the relay cannot post is reported undelivered, never dropped', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	await client.ask(sampleRequester, sampleAddressing, '안녕하세요');

	await agent.speakWithNoTurnOpen(
		'session-1',
		{ type: 'image', data: 'iVBORw0KGgo=', mimeType: 'image/png' },
		{ deliveryID: 'picture-1' }
	);
	await agent.speakWithNoTurnOpen('session-nobody-holds', { type: 'text', text: '안녕하세요' }, { deliveryID: 'stray-1' });
	await waitUntil(() => agent.deliveryReports.length === 2, 'the relay to report both');

	expect(conversation.posted).toEqual([]);
	const reasons = new Map(agent.deliveryReports.map((report) => [report.params.deliveryID, String(report.params.reason)]));
	expect(agent.deliveryReports.every((report) => report.method === undeliveredExtensionMethod)).toBe(true);
	expect(reasons.get('picture-1')).toContain('image');
	expect(reasons.get('stray-1')).toContain('session-nobody-holds');
});

test('a question asked with no turn running is put in the thread it names and reported delivered', async () => {
	const agent = anAgentOnASocket({ approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }] });
	const askedIn: Addressing[] = [];
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async (_question, addressing) => {
			askedIn.push(addressing);
			return { messageID: 'posted-question-9', answered: Promise.resolve('응 보내줘') };
		},
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	await client.ask(sampleRequester, sampleAddressing, '안녕하세요');

	const answered = await agent.askWithNoTurnOpen('session-1', 'held-9', '박예시에게 보낼까요?', {
		deliveryID: 'question-9',
		replyTargetID: 'buzz:conversation-1:message-9'
	});

	expect(askedIn.map((addressing) => addressing.replyTargetID)).toEqual(['buzz:conversation-1:message-9']);
	expect(agent.deliveryReports).toEqual([
		{ method: deliveredExtensionMethod, params: { deliveryID: 'question-9', messageID: 'posted-question-9' } }
	]);
	expect(answered.outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
});

test('a question the conversation refuses is reported undelivered and forgotten', async () => {
	const agent = anAgentOnASocket({});
	const questions = aQuestionStore();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions,
		askThePerson: async () => {
			throw new Error('chatd refused the post to buzz:conversation-1:message-9 with 503');
		},
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	await client.ask(sampleRequester, sampleAddressing, '안녕하세요');

	const answered = await agent.askWithNoTurnOpen('session-1', 'held-9', '박예시에게 보낼까요?', {
		deliveryID: 'question-9',
		replyTargetID: 'buzz:conversation-1:message-9'
	});

	expect(answered.outcome).toEqual({ outcome: 'cancelled' });
	expect(agent.deliveryReports.map((report) => report.method)).toEqual([undeliveredExtensionMethod]);
	expect(String(agent.deliveryReports[0].params.reason)).toContain('with 503');
	expect(await questions.read('held-9'), 'a question that never reached the person is still waiting for an answer').toBeNull();
});

function aClientOn(socketPath: string): BlueclawACPClient {
	const client = new BlueclawACPClient({
		socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: aConversation().post,
		postFileToConversation: noFileExpected,
		questions: aQuestionStore(),
		askThePerson: async () => putWith(''),
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());
	return client;
}

test('an agent that is not listening yet is unreachable, which is not a refusal', async () => {
	const directory = mkdtempSync(join(tmpdir(), 'acp-relay-'));
	cleanUps.push(() => rmSync(directory, { recursive: true, force: true }));
	const client = aClientOn(join(directory, 'nobody-listens.sock'));

	const failure = await client.ask(sampleRequester, sampleAddressing, '안녕하세요').catch((caught: unknown) => caught);

	expect(failure).toBeInstanceOf(AgentUnreachable);
});

test('a refusal the agent answers with stays a refusal', async () => {
	const agent = anAgentOnASocket({ refuseSessionsWith: 'a session without a conversation is nothing to answer in' });
	const client = aClientOn(agent.socketPath);

	const failure = await client.ask(sampleRequester, sampleAddressing, '안녕하세요').catch((caught: unknown) => caught);

	expect(failure).toBeInstanceOf(RequestError);
	expect(failure).not.toBeInstanceOf(AgentUnreachable);
});
