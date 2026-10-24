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
	type RequestPermissionResponse,
	type SessionUpdate
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
	type ACPSessionSettings,
	type Addressing,
	type Delivery,
	type HandoffClaim,
	type HandoffsForQuestions,
	type MessageFacts
} from './acp-session';
import { SessionBindingStore } from './session-binding-store';
import { RecordCatalogs, ticketOf } from './record-catalog';
import { agentFilePoster } from './conversation-post';
import type { KeptAttachment, WorkspaceFile } from './file-transfer';

type DeliveryReport = { method: string; params: Record<string, unknown> };

type AnAgentThatRecords = {
	socketPath: string;
	sessionsOpened: NewSessionRequest[];
	sessionsLoaded: LoadSessionRequest[];
	promptsTaken: PromptRequest[];
	approvalRequestsRead: Record<string, unknown>[];
	deliveryReports: DeliveryReport[];
	connectionsOpened: number;
	askWithNoTurnOpen: (sessionID: string, toolCallID: string, question: string, delivery: Delivery) => Promise<RequestPermissionResponse>;
	speakWithNoTurnOpen: (sessionID: string, content: ContentBlock, delivery: Delivery) => Promise<void>;
	updateWithNoTurnOpen: (sessionID: string, update: SessionUpdate, delivery: Delivery) => Promise<void>;
};

type AgentBehaviour = {
	reply?: string;
	replyFor?: (prompt: string) => string;
	holdPrompt?: { text: string; until: Promise<void> };
	askApprovalAbout?: { toolCallID: string; question: string; onlyWhenAskedTo?: string };
	approvalReplies?: { reply: string; optionID: string; isInstruction?: boolean }[];
	refuseSessionsWith?: string;
	whenAReportArrives?: () => Promise<void>;
};

const cleanUps: (() => void)[] = [];

function aSessionStore(filePath?: string): SessionBindingStore {
	return new SessionBindingStore({ filePath: filePath ?? join(mkdtempSync(join(tmpdir(), 'acp-sessions-')), 'sessions.json') });
}

afterEach(() => {
	while (cleanUps.length) cleanUps.pop()?.();
});

function anAgentOnASocket(behaviour: AgentBehaviour): AnAgentThatRecords {
	const directory = mkdtempSync(join(tmpdir(), 'acp-relay-'));
	const socketPath = join(directory, 'agent.sock');
	const sessionsOpened: NewSessionRequest[] = [];
	const sessionsLoaded: LoadSessionRequest[] = [];
	const promptsTaken: PromptRequest[] = [];
	const approvalRequestsRead: Record<string, unknown>[] = [];
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
				return { sessionId: `session-${sessionsOpened.length}` };
			},
			loadSession: async (request: LoadSessionRequest) => {
				sessionsLoaded.push(request);
			},
			prompt: async (request: PromptRequest) => {
				promptsTaken.push(request);
				const connection = connectionOf();
				if (!connection) throw new Error('the agent has no connection to answer on');
				const prompt = promptTextOf(request);
				const asking = behaviour.askApprovalAbout;
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
				if (behaviour.holdPrompt?.text === prompt) await behaviour.holdPrompt.until;
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
					await behaviour.whenAReportArrives?.();
					return {};
				}
				approvalRequestsRead.push(params);
				const known = behaviour.approvalReplies?.find((answer) => answer.reply === params.reply);
				return known ? { isAnswer: true, optionId: known.optionID, isInstruction: known.isInstruction === true } : { isAnswer: false };
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
		approvalRequestsRead,
		deliveryReports,
		get connectionsOpened() {
			return connectionsOpened;
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
		},
		updateWithNoTurnOpen: async (sessionID, update, delivery) => {
			if (!latestConnection) throw new Error('no connection to the agent yet');
			await latestConnection.sessionUpdate({ sessionId: sessionID, update, _meta: { [deliveryMetaKey]: delivery } });
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

type Edited = { addressing: Addressing; messageID: string; message: string };

function aConversation(refusal?: string) {
	const posted: Posted[] = [];
	const edited: Edited[] = [];
	const deleted: string[] = [];
	const thread: ThreadMessage[] = [];
	const editsBehindANewerMessage: string[] = [];
	return {
		posted,
		edited,
		deleted,
		editsBehindANewerMessage,
		visible: () => thread.filter((message) => !message.isDeleted),
		post: async (addressing: Addressing, message: string, attachments?: KeptAttachment[]): Promise<string> => {
			if (refusal) throw new Error(refusal);
			posted.push({ addressing, message, ...(attachments ? { attachments } : {}) });
			const messageID = `posted-${posted.length}`;
			thread.push({ messageID, text: message, isEdited: false, isDeleted: false });
			return messageID;
		},
		edit: async (addressing: Addressing, messageID: string, message: string): Promise<void> => {
			edited.push({ addressing, messageID, message });
			const visible = thread.filter((shown) => !shown.isDeleted);
			if (visible.at(-1)?.messageID !== messageID) editsBehindANewerMessage.push(messageID);
			const target = thread.find((shown) => shown.messageID === messageID);
			if (target) Object.assign(target, { text: message, isEdited: true });
		},
		delete: async (_addressing: Addressing, messageID: string): Promise<void> => {
			deleted.push(messageID);
			const target = thread.find((shown) => shown.messageID === messageID);
			if (target) target.isDeleted = true;
		}
	};
}

type ThreadMessage = { messageID: string; text: string; isEdited: boolean; isDeleted: boolean };

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
		editInConversation: async () => {},
		deleteInConversation: async () => {},
		sessions: aSessionStore(),
		approvalWasRequested: () => {}
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
		editInConversation: async () => {},
		deleteInConversation: async () => {},
		sessions: aSessionStore(),
		approvalWasRequested: () => {}
	});
	cleanUps.push(() => client.close());

	await client.ask(sampleRequester, sampleAddressing, '첫 번째');
	await client.ask(sampleRequester, sampleAddressing, '두 번째');

	expect(agent.sessionsOpened).toHaveLength(1);
	expect(agent.promptsTaken).toHaveLength(2);
});

const questionThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-9', isThread: true };
const questionDelivery = { deliveryID: 'question-9', replyTargetID: questionThread.replyTargetID };

async function aSessionOpenedBy(client: BlueclawACPClient): Promise<void> {
	await client.ask(sampleRequester, sampleAddressing, '안녕하세요');
}

const secondSpeaker = { email: 'second@example.test', name: '박예시' };

function sessionRequesterOf(opened: NewSessionRequest): unknown {
	const carried = opened._meta?.[sessionMetaKey];
	return typeof carried === 'object' && carried !== null && 'requester' in carried ? carried.requester : undefined;
}

test('a second person in the same conversation is answered in a session of their own, as themselves', async () => {
	const agent = anAgentOnASocket({ reply: '네' });
	const catalogsAskedFor: string[] = [];
	const client = aClientFor(agent.socketPath, aConversation(), {
		catalogFor: (requesterEmail) => {
			catalogsAskedFor.push(requesterEmail);
			return [];
		}
	});

	await client.ask(sampleRequester, sampleAddressing, '첫 번째');
	await client.ask(secondSpeaker, sampleAddressing, '두 번째');
	await client.ask(sampleRequester, sampleAddressing, '세 번째');

	expect(agent.sessionsOpened.map(sessionRequesterOf)).toEqual([sampleRequester, secondSpeaker]);
	expect(agent.promptsTaken.map((prompt) => prompt.sessionId)).toEqual(['session-1', 'session-2', 'session-1']);
	expect(catalogsAskedFor).toEqual([sampleRequester.email, secondSpeaker.email]);
});

test('another person speaking while the requester is asked to approve is neither read as the answer nor run as the requester', async () => {
	const agent = anAgentOnASocket({ approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }] });
	const client = aClientFor(agent.socketPath, aConversation());
	await aSessionOpenedBy(client);
	const asking = agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', questionDelivery);
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the approval to be requested');

	expect(client.hasPendingApprovalIn('conversation-1', secondSpeaker.email)).toBe(false);
	expect(await client.answerPendingApproval(questionThread, 'message-10', '응 보내줘', secondSpeaker)).toBe(false);
	await client.ask(secondSpeaker, sampleAddressing, '응 보내줘');

	expect(agent.approvalRequestsRead).toEqual([]);
	expect(agent.promptsTaken.at(-1)?.sessionId).toBe('session-2');
	expect(sessionRequesterOf(agent.sessionsOpened[1])).toEqual(secondSpeaker);
	expect(await client.answerPendingApproval(questionThread, 'message-11', '응 보내줘', sampleRequester)).toBe(true);
	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
});


test('a reply that arrives the moment the question is reported delivered still answers it', async () => {
	let wasAnswer: boolean | undefined;
	const agent = anAgentOnASocket({
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }],
		whenAReportArrives: async () => {
			wasAnswer = await client.answerPendingApproval(questionThread, 'message-10', '응 보내줘', sampleRequester);
		}
	});
	const client = aClientFor(agent.socketPath, aConversation());
	await aSessionOpenedBy(client);

	const asking = agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', questionDelivery);

	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(wasAnswer).toBe(true);
});

test('a reply blueclaw calls an answer resolves the approval with the option it names', async () => {
	const agent = anAgentOnASocket({ approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }] });
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	await aSessionOpenedBy(client);

	const asking = agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', questionDelivery);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the question');
	const wasAnswer = await client.answerPendingApproval(questionThread, 'message-10', '응 보내줘', sampleRequester);

	expect(wasAnswer).toBe(true);
	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(conversation.posted).toEqual([
		{ addressing: { ...sampleAddressing, replyTargetID: questionThread.replyTargetID }, message: '박예시에게 보낼까요?' }
	]);
	expect(agent.deliveryReports).toEqual([
		{ method: deliveredExtensionMethod, params: { deliveryID: 'question-9', messageID: 'posted-1' } }
	]);
	expect(agent.approvalRequestsRead).toEqual([
		{
			sessionId: 'session-1',
			toolCallId: 'call-9',
			reply: '응 보내줘',
			messageId: 'message-10',
			replyTargetId: questionThread.replyTargetID,
			isThread: true,
			replierEmail: sampleRequester.email
		}
	]);
	expect(client.hasPendingApprovalIn('conversation-1', sampleRequester.email)).toBe(false);
});

test('a reply blueclaw reads as the next instruction rejects the approval once and goes on as a prompt', async () => {
	const agent = anAgentOnASocket({
		approvalReplies: [
			{ reply: '오늘 일정 알려줘', optionID: 'reject_once', isInstruction: true },
			{ reply: '응 보내줘', optionID: 'approve_once' }
		]
	});
	const client = aClientFor(agent.socketPath, aConversation());
	await aSessionOpenedBy(client);
	const asking = agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', questionDelivery);
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the approval to be requested');

	const wasConsumed = await client.answerPendingApproval(sampleAddressing, 'message-10', '오늘 일정 알려줘', sampleRequester);

	expect(wasConsumed).toBe(false);
	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'reject_once' });
	expect(client.hasPendingApprovalIn('conversation-1', sampleRequester.email)).toBe(false);
	expect(await client.answerPendingApproval(sampleAddressing, 'message-11', '응 보내줘', sampleRequester)).toBe(false);
});

test('an approval blueclaw says it already posted is not posted again, and still waits for its answer', async () => {
	const agent = anAgentOnASocket({ approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }] });
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	await aSessionOpenedBy(client);

	const asking = agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', {
		...questionDelivery,
		isAlreadyPosted: true
	});
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the approval to be requested');
	await client.answerPendingApproval(sampleAddressing, 'message-10', '응 보내줘', sampleRequester);

	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(conversation.posted).toEqual([]);
	expect(agent.deliveryReports).toEqual([]);
});

test('each reply is posted in the thread it names, even while another turn waits on an approval', async () => {
	const agent = anAgentOnASocket({
		replyFor: (prompt) => `${prompt}에 답합니다`,
		askApprovalAbout: { toolCallID: 'call-1', question: '박예시에게 보낼까요?', onlyWhenAskedTo: '박예시한테 DM 보내줘' },
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }]
	});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	const askingThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-1' };
	const laterThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-2' };

	const waiting = client.ask(sampleRequester, askingThread, '박예시한테 DM 보내줘', factsIn(askingThread, 'message-1'));
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the first turn to ask its question');
	await client.ask(sampleRequester, laterThread, '오늘 일정 알려줘', factsIn(laterThread, 'message-2'));
	await client.answerPendingApproval(askingThread, 'message-3', '응 보내줘', sampleRequester);
	await waiting;

	expect(
		conversation.posted.map((posted) => [posted.addressing.replyTargetID, posted.message]),
		'a message was posted in a thread other than the one it named'
	).toEqual([
		[askingThread.replyTargetID, '박예시에게 보낼까요?'],
		[laterThread.replyTargetID, '오늘 일정 알려줘에 답합니다'],
		[askingThread.replyTargetID, '박예시한테 DM 보내줘에 답합니다']
	]);
});

test('a question the conversation refuses cancels the approval and is reported undelivered with the reason', async () => {
	const agent = anAgentOnASocket({});
	const client = aClientFor(agent.socketPath, aConversation('chatd refused the post to buzz:conversation-1:message-9 with 503'));
	await aSessionOpenedBy(client);

	const answered = await agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', questionDelivery);

	expect(answered.outcome).toEqual({ outcome: 'cancelled' });
	expect(agent.deliveryReports.map((report) => report.method)).toEqual([undeliveredExtensionMethod]);
	expect(String(agent.deliveryReports[0].params.reason)).toContain('with 503');
	expect(client.hasPendingApprovalIn('conversation-1', sampleRequester.email)).toBe(false);
});

test('an approval asked in a session this relay has no binding for is cancelled and reported undelivered with the reason', async () => {
	const agent = anAgentOnASocket({});
	const client = aClientFor(agent.socketPath, aConversation());
	await aSessionOpenedBy(client);

	const answered = await agent.askWithNoTurnOpen('session-nobody-binds', 'call-9', '박예시에게 보낼까요?', questionDelivery);

	expect(answered.outcome).toEqual({ outcome: 'cancelled' });
	expect(agent.deliveryReports.map((report) => report.method)).toEqual([undeliveredExtensionMethod]);
	expect(String(agent.deliveryReports[0].params.reason)).toContain('session-nobody-binds');
});

test('an approval that blueclaw leaves the socket without hearing an answer for is cancelled out loud', async () => {
	const agent = anAgentOnASocket({});
	const reported: string[] = [];
	const client = aClientFor(agent.socketPath, aConversation(), { report: (line) => reported.push(line) });
	await aSessionOpenedBy(client);
	const asking = agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', questionDelivery);
	asking.catch(() => {});
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the approval to be requested');

	client.close();

	await waitUntil(
		() => reported.some((line) => line.includes('call-9') && line.includes('cancelled')),
		'the cancelled approval to be reported'
	);
});

const progressThread = 'buzz:conversation-1:message-7';
const progressDelivery = { deliveryID: 'turn-1', replyTargetID: progressThread };
const finalReplyDelivery = { ...progressDelivery, isFinal: true };

test('tool progress is posted once, edited on each update, and taken down once the reply is posted after it', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	await aSessionOpenedBy(client);
	const threadAddressing = { ...sampleAddressing, replyTargetID: progressThread };

	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call', toolCallId: 'call-1', title: '일정 읽기', status: 'in_progress' },
		progressDelivery
	);
	await waitUntil(() => conversation.posted.length === 1, 'the progress message to be posted');
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call_update', toolCallId: 'call-1', status: 'completed' },
		progressDelivery
	);
	await waitUntil(() => conversation.edited.length === 1, 'the progress message to be edited');
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call', toolCallId: 'call-2', title: '메일 보내기', status: 'pending' },
		progressDelivery
	);
	await waitUntil(() => conversation.edited.length === 2, 'the progress message to show both calls');

	expect(conversation.posted).toEqual([{ addressing: threadAddressing, message: '◐ 일정 읽기' }]);
	expect(conversation.edited.map((edit) => [edit.messageID, edit.message])).toEqual([
		['posted-1', '✓ 일정 읽기'],
		['posted-1', '✓ 일정 읽기\n○ 메일 보내기']
	]);

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '보냈습니다' }, finalReplyDelivery);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the reply');

	expect(conversation.posted.at(-1)).toEqual({ addressing: threadAddressing, message: '보냈습니다' });
	expect(conversation.edited).toHaveLength(2);
	expect(conversation.deleted).toEqual(['posted-1']);
	expect(agent.deliveryReports).toEqual([
		{ method: deliveredExtensionMethod, params: { deliveryID: 'turn-1', messageID: 'posted-2' } }
	]);
});

test('a reply that is not marked final leaves the progress message and is posted as its own', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	await aSessionOpenedBy(client);
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call', toolCallId: 'call-1', title: '일정 읽기', status: 'in_progress' },
		progressDelivery
	);
	await waitUntil(() => conversation.posted.length === 1, 'the progress message to be posted');

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '잠시만요' }, progressDelivery);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the reply');

	expect(conversation.edited).toEqual([]);
	expect(conversation.posted.map((posted) => posted.message)).toEqual(['◐ 일정 읽기', '잠시만요']);
});

test('a final reply of another delivery does not replace the progress of this one', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	await aSessionOpenedBy(client);
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call', toolCallId: 'call-1', title: '일정 읽기', status: 'in_progress' },
		progressDelivery
	);
	await waitUntil(() => conversation.posted.length === 1, 'the progress message to be posted');

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '다른 답' }, { ...finalReplyDelivery, deliveryID: 'turn-2' });
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the reply');

	expect(conversation.edited).toEqual([]);
	expect(conversation.posted.map((posted) => posted.message)).toEqual(['◐ 일정 읽기', '다른 답']);
});

test('a turn waiting on an approval is told apart from one that is not, also after a reissue', async () => {
	const release = Promise.withResolvers<void>();
	const agent = anAgentOnASocket({
		askApprovalAbout: { toolCallID: 'call-1', question: '박예시에게 보낼까요?', onlyWhenAskedTo: '박예시한테 DM 보내줘' },
		holdPrompt: { text: '오늘 일정 알려줘', until: release.promise },
		approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }]
	});
	const client = aClientFor(agent.socketPath, aConversation());
	const waiting = client.ask(sampleRequester, sampleAddressing, '박예시한테 DM 보내줘', factsIn(sampleAddressing, 'message-1'));
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the first turn to wait');
	const running = client.ask(sampleRequester, sampleAddressing, '오늘 일정 알려줘', factsIn(sampleAddressing, 'message-2'));
	await waitUntil(() => agent.promptsTaken.length === 2, 'the second turn to start');

	expect(client.isWaitingOnApproval('conversation-1', 'message-1')).toBe(true);
	expect(client.isWaitingOnApproval('conversation-1', 'message-2')).toBe(false);

	await client.answerPendingApproval(sampleAddressing, 'message-3', '응 보내줘', sampleRequester);
	await waiting;
	const reissued = agent.askWithNoTurnOpen('session-1', 'call-2', '다시 보낼까요?', { ...questionDelivery, isAlreadyPosted: true });
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the approval to be reissued');

	expect(client.isWaitingOnApproval('conversation-1', 'message-1')).toBe(false);
	expect(client.isWaitingOnApproval('conversation-1', 'message-2'), 'a turn that was running when the approval was reissued was taken as waiting').toBe(false);

	await client.answerPendingApproval(sampleAddressing, 'message-4', '응 보내줘', sampleRequester);
	await reissued;
	release.resolve();
	await running;
});

test('after a restart the session bindings are loaded, a reissued approval waits without posting, and the next message answers it', async () => {
	const agent = anAgentOnASocket({ approvalReplies: [{ reply: '응 보내줘', optionID: 'approve_once' }] });
	const sessions = aSessionStore();
	const beforeTheRestart = aClientFor(agent.socketPath, aConversation(), { sessions });
	await beforeTheRestart.ask(sampleRequester, sampleAddressing, '안녕하세요');
	beforeTheRestart.close();

	const conversation = aConversation();
	const afterTheRestart = aClientFor(agent.socketPath, conversation, { sessions });
	await afterTheRestart.restoreSessionBindings();
	await waitUntil(() => agent.sessionsLoaded.length === 1, 'the session binding to be loaded');
	const asking = agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', { ...questionDelivery, isAlreadyPosted: true });
	await waitUntil(() => afterTheRestart.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the approval to be waited on');

	expect(agent.sessionsLoaded[0].sessionId).toBe('session-1');
	expect(agent.sessionsLoaded[0]._meta?.[sessionMetaKey]).toEqual({ requester: sampleRequester, addressing: sampleAddressing });
	expect(await afterTheRestart.answerPendingApproval(sampleAddressing, 'message-10', '응 보내줘', sampleRequester)).toBe(true);
	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(conversation.posted).toEqual([]);
	expect(agent.sessionsOpened).toHaveLength(1);
});

test('after a restart a session keeps the record catalog address it was opened with, and the new relay answers it', async () => {
	const agent = anAgentOnASocket({ reply: '네' });
	const sessions = aSessionStore();
	const catalogsBefore = aRecordCatalog();
	const beforeTheRestart = aClientFor(agent.socketPath, aConversation(), {
		sessions,
		catalogFor: (email, ticket) => catalogsBefore.serversFor(email, ticket)
	});
	await beforeTheRestart.ask(sampleRequester, sampleAddressing, '안녕하세요');
	beforeTheRestart.close();

	const catalogsAfter = aRecordCatalog();
	const afterTheRestart = aClientFor(agent.socketPath, aConversation(), {
		sessions,
		catalogFor: (email, ticket) => catalogsAfter.serversFor(email, ticket)
	});
	await afterTheRestart.restoreSessionBindings();
	await waitUntil(() => agent.sessionsLoaded.length === 1, 'the session binding to be loaded');

	const openedAt = agent.sessionsOpened[0].mcpServers[0];
	const loadedAt = agent.sessionsLoaded[0].mcpServers[0];
	expect('url' in loadedAt && loadedAt.url).toBe('url' in openedAt && openedAt.url);
	const address = new URL('url' in openedAt ? openedAt.url : '');
	const originalFetch = globalThis.fetch;
	globalThis.fetch = (async () => Response.json({ jsonrpc: '2.0', id: 1, result: { tools: [] } })) as unknown as typeof fetch;
	try {
		const answered = await catalogsAfter.serve(aCatalogCall(address), ticketOf(address.pathname));
		expect(await answered.json()).toEqual({ jsonrpc: '2.0', id: 1, result: { tools: [] } });
	} finally {
		globalThis.fetch = originalFetch;
	}
});

function aRecordCatalog(): RecordCatalogs {
	return new RecordCatalogs({
		appURL: 'http://app.test',
		loopbackURL: 'http://127.0.0.1:18091',
		mintFor: async () => ({ accessToken: 'a-member-token', expiresAt: Math.floor(Date.now() / 1000) + 3600 })
	});
}

function aCatalogCall(address: URL): Request {
	return new Request(address, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/list', params: {} })
	});
}

test('a checkpoint that cannot be edited into the progress is posted after it and the progress is taken down', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const reported: string[] = [];
	const client = aClientFor(agent.socketPath, conversation, {
		editInConversation: async () => {
			throw new Error('chatd refused the edit with 503');
		},
		report: (line) => reported.push(line)
	});
	await aSessionOpenedBy(client);
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call', toolCallId: 'call-1', title: '일정 읽기', status: 'in_progress' },
		progressDelivery
	);
	await agent.updateWithNoTurnOpen('session-1', aCheckpoint('일정을 읽었습니다'), progressDelivery);
	await waitUntil(() => conversation.deleted.length === 1, 'the progress message to be taken down');

	expect(conversation.posted.map((posted) => posted.message)).toEqual(['◐ 일정 읽기', '일정을 읽었습니다']);
	expect(conversation.deleted).toEqual(['posted-1']);
	expect(reported.some((line) => line.includes('with 503'))).toBe(true);
});

function aCheckpoint(text: string): SessionUpdate {
	return { sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text } };
}

function aToolCall(toolCallID: string, title: string): SessionUpdate[] {
	return [
		{ sessionUpdate: 'tool_call', toolCallId: toolCallID, title, status: 'in_progress' },
		{ sessionUpdate: 'tool_call_update', toolCallId: toolCallID, status: 'completed' }
	];
}

type TurnStep = { tool: string } | { checkpoint: string };

const threadOrderCases: { name: string; steps: TurnStep[]; final: ContentBlock[]; thread: string[] }[] = [
	{
		name: 'checkpoints become the progress before them and the answer is posted last',
		steps: [{ tool: '일정 읽기' }, { checkpoint: '일정을 읽었습니다' }, { tool: '메일 쓰기' }, { checkpoint: '메일을 썼습니다' }, { tool: '메일 보내기' }],
		final: [{ type: 'text', text: '보냈습니다' }],
		thread: ['일정을 읽었습니다', '메일을 썼습니다', '보냈습니다']
	},
	{
		name: 'a turn without a checkpoint takes its progress down and posts the answer',
		steps: [{ tool: '일정 읽기' }, { tool: '메일 보내기' }],
		final: [{ type: 'text', text: '보냈습니다' }],
		thread: ['보냈습니다']
	},
	{
		name: 'two checkpoints in a row post the second one new',
		steps: [{ tool: '일정 읽기' }, { checkpoint: '일정을 읽었습니다' }, { checkpoint: '조금 더 걸립니다' }],
		final: [{ type: 'text', text: '보냈습니다' }],
		thread: ['일정을 읽었습니다', '조금 더 걸립니다', '보냈습니다']
	},
	{
		name: 'an answer that carries a file is posted after the checkpoints, the file before its words',
		steps: [{ tool: '일정 읽기' }, { checkpoint: '일정을 읽었습니다' }, { tool: '보고서 쓰기' }],
		final: [
			{ type: 'resource_link', name: 'report.pdf', uri: 'file:///workspace/report.pdf' },
			{ type: 'text', text: '보고서입니다' }
		],
		thread: ['일정을 읽었습니다', 'file report.pdf', '보고서입니다']
	}
];

for (const threadOrderCase of threadOrderCases) {
	test(`a turn reads in the order it happened: ${threadOrderCase.name}`, async () => {
		const agent = anAgentOnASocket({});
		const conversation = aConversation();
		const client = aClientFor(agent.socketPath, conversation, {
			postFileToConversation: (addressing, _requesterEmail, file) => conversation.post(addressing, `file ${file.filename}`)
		});
		await aSessionOpenedBy(client);

		for (const [index, step] of threadOrderCase.steps.entries()) {
			const updates = 'tool' in step ? aToolCall(`call-${index}`, step.tool) : [aCheckpoint(step.checkpoint)];
			for (const update of updates) await agent.updateWithNoTurnOpen('session-1', update, progressDelivery);
		}
		for (const content of threadOrderCase.final) {
			const reported = agent.deliveryReports.length;
			await agent.speakWithNoTurnOpen('session-1', content, finalReplyDelivery);
			await waitUntil(() => agent.deliveryReports.length === reported + 1, 'the relay to report the reply');
		}

		const visible = conversation.visible();
		expect(visible.map((message) => message.text)).toEqual(threadOrderCase.thread);
		expect(visible.at(-1)?.isEdited, 'the answer arrived as an edit, which tells nobody').toBe(false);
		expect(conversation.editsBehindANewerMessage, 'a message was rewritten with something already after it').toEqual([]);
	});
}

test('a reply is posted as it arrives, and the agent is told once which message it became', async () => {
	const agent = anAgentOnASocket({ reply: '보냈습니다' });
	const conversation = aConversation();
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: noFileExpected,
		editInConversation: async () => {},
		deleteInConversation: async () => {},
		sessions: aSessionStore(),
		approvalWasRequested: () => {}
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
		editInConversation: async () => {},
		deleteInConversation: async () => {},
		sessions: aSessionStore(),
		approvalWasRequested: () => {}
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
		editInConversation: async () => {},
		deleteInConversation: async () => {},
		sessions: aSessionStore(),
		approvalWasRequested: () => {}
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
		editInConversation: async () => {},
		deleteInConversation: async () => {},
		sessions: aSessionStore(),
		approvalWasRequested: () => {}
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
		editInConversation: async () => {},
		deleteInConversation: async () => {},
		sessions: aSessionStore(),
		approvalWasRequested: () => {}
	});
	cleanUps.push(() => client.close());
	await client.ask(sampleRequester, sampleAddressing, '안녕하세요');

	await agent.speakWithNoTurnOpen(
		'session-1',
		{ type: 'image', data: 'iVBORw0KGgo=', mimeType: 'image/png' },
		{ deliveryID: 'picture-1' }
	);
	await agent.speakWithNoTurnOpen('session-nobody-binds', { type: 'text', text: '안녕하세요' }, { deliveryID: 'stray-1' });
	await waitUntil(() => agent.deliveryReports.length === 2, 'the relay to report both');

	expect(conversation.posted).toEqual([]);
	const reasons = new Map(agent.deliveryReports.map((report) => [report.params.deliveryID, String(report.params.reason)]));
	expect(agent.deliveryReports.every((report) => report.method === undeliveredExtensionMethod)).toBe(true);
	expect(reasons.get('picture-1')).toContain('image');
	expect(reasons.get('stray-1')).toContain('session-nobody-binds');
});

function aClientOn(socketPath: string): BlueclawACPClient {
	return aClientFor(socketPath, aConversation());
}

function aClientFor(
	socketPath: string,
	conversation: ReturnType<typeof aConversation>,
	overrides: Partial<ACPSessionSettings> = {}
): BlueclawACPClient {
	const client = new BlueclawACPClient({
		socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		postToConversation: conversation.post,
		postFileToConversation: noFileExpected,
		editInConversation: conversation.edit,
		deleteInConversation: conversation.delete,
		sessions: aSessionStore(),
		approvalWasRequested: () => {},
		...overrides
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

function handoffsFor(claimant: string, claimOfTheClaimant: HandoffClaim = 'open') {
	const claims: string[] = [];
	const answeredInThread: { handoffID: string; isDone: boolean }[] = [];
	let settle: () => void = () => {};
	const handoffs: HandoffsForQuestions = {
		claim: async (handoffID, requesterEmail, settleTheQuestion) => {
			claims.push(handoffID);
			settle = settleTheQuestion;
			return requesterEmail === claimant ? claimOfTheClaimant : 'refused';
		},
		linkOf: (handoffID) => `https://intern.example.com/handoff/${handoffID}`,
		answeredInTheThread: async (handoffID, isDone) => {
			answeredInThread.push({ handoffID, isDone });
		}
	};
	return { handoffs, claims, answeredInThread, handBack: () => settle() };
}

test('a hand-over is posted with its link and answered when the requester hands the browser back', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const handover = handoffsFor(sampleRequester.email);
	const client = aClientFor(agent.socketPath, conversation, { handoffs: handover.handoffs });
	await aSessionOpenedBy(client);

	const asking = agent.askWithNoTurnOpen('session-1', 'hold-1', '아래 링크에서 직접 로그인해 주세요.', { ...questionDelivery, handoffID: 'handoff-1' });
	await waitUntil(() => conversation.posted.length === 1, 'the hand-over to be posted');
	handover.handBack();

	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(handover.claims).toEqual(['handoff-1']);
	expect(conversation.posted[0]?.message).toBe('아래 링크에서 직접 로그인해 주세요.\n\nhttps://intern.example.com/handoff/handoff-1');
	expect(client.hasPendingApprovalIn('conversation-1', sampleRequester.email)).toBe(false);
});

test('a hand-over the session’s requester cannot claim is never posted', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation, { handoffs: handoffsFor('other@example.test').handoffs });
	await aSessionOpenedBy(client);

	const asking = agent.askWithNoTurnOpen('session-1', 'hold-1', '로그인해 주세요.', { ...questionDelivery, handoffID: 'handoff-1' });

	expect((await asking).outcome).toEqual({ outcome: 'cancelled' });
	expect(conversation.posted).toEqual([]);
});

test('a hand-over that ended while nobody held its question answers at once and posts nothing', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation, { handoffs: handoffsFor(sampleRequester.email, 'ended').handoffs });
	await aSessionOpenedBy(client);

	const asking = agent.askWithNoTurnOpen('session-1', 'hold-1', '로그인해 주세요.', { ...questionDelivery, handoffID: 'handoff-1' });

	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'approve_once' });
	expect(conversation.posted).toEqual([]);
	expect(client.hasPendingApprovalIn('conversation-1', sampleRequester.email)).toBe(false);
});

test('declining a hand-over in the thread ends it as abandoned', async () => {
	const agent = anAgentOnASocket({ approvalReplies: [{ reply: '안 할래', optionID: 'reject_once' }] });
	const conversation = aConversation();
	const handover = handoffsFor(sampleRequester.email);
	const client = aClientFor(agent.socketPath, conversation, { handoffs: handover.handoffs });
	await aSessionOpenedBy(client);
	const asking = agent.askWithNoTurnOpen('session-1', 'hold-1', '로그인해 주세요.', { ...questionDelivery, handoffID: 'handoff-1' });
	await waitUntil(() => client.hasPendingApprovalIn('conversation-1', sampleRequester.email), 'the hand-over to be asked');

	await client.answerPendingApproval(questionThread, 'message-10', '안 할래', sampleRequester);

	expect((await asking).outcome).toEqual({ outcome: 'selected', optionId: 'reject_once' });
	expect(handover.answeredInThread).toEqual([{ handoffID: 'handoff-1', isDone: false }]);
});
