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
	abandonedOptionKey,
	AgentUnreachable,
	BlueclawACPClient,
	deliveredExtensionMethod,
	deliveryMetaKey,
	handedBackOptionKey,
	handoffClosedExtensionMethod,
	handoffEndedExtensionMethod,
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
import { agentFilePoster, replyThreadOf } from './conversation-post';
import type { WorkWatcher } from './tool-progress';
import type { KeptAttachment, WorkspaceFile } from './file-transfer';

type DeliveryReport = { method: string; params: Record<string, unknown> };

type AnAgentThatRecords = {
	socketPath: string;
	sessionsOpened: NewSessionRequest[];
	sessionsLoaded: LoadSessionRequest[];
	promptsTaken: PromptRequest[];
	deliveryReports: DeliveryReport[];
	connectionsOpened: number;
	askWithNoTurnOpen: (sessionID: string, toolCallID: string, question: string, delivery: Delivery) => Promise<RequestPermissionResponse>;
	speakWithNoTurnOpen: (sessionID: string, content: ContentBlock, delivery: Delivery) => Promise<void>;
	notifyWithNoTurnOpen: (method: string, params: Record<string, unknown>) => Promise<void>;
	updateWithNoTurnOpen: (sessionID: string, update: SessionUpdate, delivery: Delivery) => Promise<void>;
};

type AgentBehaviour = {
	reply?: string;
	replyFor?: (prompt: string) => string;
	holdPrompt?: { text: string; until: Promise<void> };
	refusesHandoffEnds?: boolean;
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
				deliveryReports.push({ method, params });
				await behaviour.whenAReportArrives?.();
				if (method === handoffEndedExtensionMethod && behaviour.refusesHandoffEnds) {
					throw new RequestError(-32603, 'Internal error', { error: 'no run is parked on that hand-over' });
				}
				return {};
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
		notifyWithNoTurnOpen: async (method, params) => {
			if (!latestConnection) throw new Error('no connection to the agent yet');
			await latestConnection.extNotification(method, params);
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
		sessions: aSessionStore()
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
		sessions: aSessionStore()
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

test('a held call arrives as a message: its question is posted in the thread it names and reported delivered', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	await aSessionOpenedBy(client);

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '박예시에게 보낼까요?' }, { ...questionDelivery, isFinal: true });
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the question');

	expect(conversation.posted).toEqual([
		{ addressing: { ...sampleAddressing, replyTargetID: questionThread.replyTargetID }, message: '박예시에게 보낼까요?' }
	]);
	expect(agent.deliveryReports).toEqual([
		{ method: deliveredExtensionMethod, params: { deliveryID: 'question-9', messageID: 'posted-1' } }
	]);
});

test('a reply to a question is a prompt like any other message, and the relay reads nothing into it', async () => {
	const agent = anAgentOnASocket({ reply: '보냈습니다' });
	const client = aClientFor(agent.socketPath, aConversation());
	await aSessionOpenedBy(client);
	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '박예시에게 보낼까요?' }, { ...questionDelivery, isFinal: true });
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the question');

	await client.ask(sampleRequester, questionThread, '응 보내줘', factsIn(questionThread, 'message-10'));

	expect(agent.promptsTaken.map(promptTextOf)).toEqual(['안녕하세요', '응 보내줘']);
	expect(agent.promptsTaken.at(-1)?._meta?.[messageMetaKey]).toEqual({
		messageID: 'message-10',
		replyTargetID: questionThread.replyTargetID,
		isThread: true,
		context: {}
	});
	expect(agent.deliveryReports.map((report) => report.params.deliveryID)).toEqual(['reply-1', 'question-9', 'reply-2']);
});

test('a permission request over the socket is cancelled out loud and nothing is posted for it', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const reported: string[] = [];
	const client = aClientFor(agent.socketPath, conversation, { report: (line) => reported.push(line) });
	await aSessionOpenedBy(client);

	const answered = await agent.askWithNoTurnOpen('session-1', 'call-9', '박예시에게 보낼까요?', questionDelivery);

	expect(answered.outcome).toEqual({ outcome: 'cancelled' });
	expect(conversation.posted).toEqual([]);
	expect(reported.some((line) => line.includes('call-9'))).toBe(true);
});

test('each reply is posted in the thread it names', async () => {
	const agent = anAgentOnASocket({ replyFor: (prompt) => `${prompt}에 답합니다` });
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation);
	const firstThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-1' };
	const laterThread = { ...sampleAddressing, replyTargetID: 'buzz:conversation-1:message-2' };

	await client.ask(sampleRequester, firstThread, '박예시한테 DM 보내줘', factsIn(firstThread, 'message-1'));
	await client.ask(sampleRequester, laterThread, '오늘 일정 알려줘', factsIn(laterThread, 'message-2'));

	expect(
		conversation.posted.map((posted) => [posted.addressing.replyTargetID, posted.message]),
		'a message was posted in a thread other than the one it named'
	).toEqual([
		[firstThread.replyTargetID, '박예시한테 DM 보내줘에 답합니다'],
		[laterThread.replyTargetID, '오늘 일정 알려줘에 답합니다']
	]);
});

test('a question the conversation refuses is reported undelivered with the reason', async () => {
	const agent = anAgentOnASocket({});
	const client = aClientFor(agent.socketPath, aConversation('chatd refused the post to buzz:conversation-1:message-9 with 503'));
	await aSessionOpenedBy(client).catch(() => undefined);

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '박예시에게 보낼까요?' }, { ...questionDelivery, isFinal: true });
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the question');

	expect(agent.deliveryReports.map((report) => report.method)).toEqual([undeliveredExtensionMethod]);
	expect(String(agent.deliveryReports[0].params.reason)).toContain('with 503');
});

const progressThread = 'buzz:conversation-1:message-7';
const progressDelivery = { deliveryID: 'turn-1', replyTargetID: progressThread };
const finalReplyDelivery = { ...progressDelivery, isFinal: true };

function aWorkRecorder() {
	const heard: Record<string, unknown>[] = [];
	const work: WorkWatcher = {
		working: (addressing, seen) => heard.push({ event: 'working', threadID: replyThreadOf(addressing), ...seen }),
		said: (addressing, said) => heard.push({ event: 'said', threadID: replyThreadOf(addressing), ...said })
	};
	return { heard, work };
}

test('where browsers draw the work, steps and what the agent said go out as events and no progress message is posted', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const recorder = aWorkRecorder();
	const client = aClientFor(agent.socketPath, conversation, { work: recorder.work, postsProgressMessages: false });
	await aSessionOpenedBy(client);

	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call', toolCallId: 'call-1', title: 'event_list', status: 'in_progress' },
		progressDelivery
	);
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call_update', toolCallId: 'call-1', status: 'completed' },
		progressDelivery
	);
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: '미팅 2개를 찾았어요' } },
		progressDelivery
	);
	await waitUntil(() => conversation.posted.length === 1, 'the checkpoint to be posted');
	await agent.updateWithNoTurnOpen(
		'session-1',
		{ sessionUpdate: 'tool_call', toolCallId: 'call-2', title: 'room_list', status: 'in_progress' },
		progressDelivery
	);
	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '회의실도 잡혀 있어요' }, finalReplyDelivery);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the reply');

	expect(conversation.posted.map((posted) => posted.message)).toEqual(['미팅 2개를 찾았어요', '회의실도 잡혀 있어요']);
	expect(conversation.edited).toEqual([]);
	expect(conversation.deleted).toEqual([]);
	expect(recorder.heard).toEqual([
		{ event: 'working', threadID: progressThread, deliveryID: 'turn-1', steps: [{ title: 'event_list', status: 'in_progress' }] },
		{ event: 'working', threadID: progressThread, deliveryID: 'turn-1', steps: [{ title: 'event_list', status: 'completed' }] },
		{ event: 'said', threadID: progressThread, deliveryID: 'turn-1', messageID: 'posted-1', isStillWorking: true },
		{ event: 'working', threadID: progressThread, deliveryID: 'turn-1', steps: [{ title: 'room_list', status: 'in_progress' }] },
		{ event: 'said', threadID: progressThread, deliveryID: 'turn-1', messageID: 'posted-2', isStillWorking: false }
	]);
});

test('a direct question shows work from the moment it is asked, and the turn ending stops whatever never said its last word', async () => {
	const agent = anAgentOnASocket({});
	const recorder = aWorkRecorder();
	const client = aClientFor(agent.socketPath, aConversation(), { work: recorder.work, postsProgressMessages: false });
	const directAddressing = { ...sampleAddressing, conversationType: 'direct' };

	await client.ask(sampleRequester, directAddressing, '안녕하세요', { messageID: 'message-1', context: {} });

	expect(recorder.heard).toEqual([
		{ event: 'working', threadID: 'conversation-1', promptID: 'message-1', steps: [] },
		{ event: 'said', threadID: 'conversation-1', promptID: 'message-1', isStillWorking: false }
	]);
});

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

test('after a restart the session bindings are loaded and nothing is waited on or posted', async () => {
	const agent = anAgentOnASocket({});
	const sessions = aSessionStore();
	const beforeTheRestart = aClientFor(agent.socketPath, aConversation(), { sessions });
	await beforeTheRestart.ask(sampleRequester, sampleAddressing, '안녕하세요');
	beforeTheRestart.close();

	const conversation = aConversation();
	const afterTheRestart = aClientFor(agent.socketPath, conversation, { sessions });
	await afterTheRestart.restoreSessionBindings();
	await waitUntil(() => agent.sessionsLoaded.length === 1, 'the session binding to be loaded');

	expect(agent.sessionsLoaded[0].sessionId).toBe('session-1');
	expect(agent.sessionsLoaded[0]._meta?.[sessionMetaKey]).toEqual({ requester: sampleRequester, addressing: sampleAddressing });
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
		sessions: aSessionStore()
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
		sessions: aSessionStore()
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
		sessions: aSessionStore()
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
		sessions: aSessionStore()
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
		sessions: aSessionStore()
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
	const claims: { handoffID: string; sessionID: string }[] = [];
	const closed: string[] = [];
	const handoffs: HandoffsForQuestions = {
		claim: async (handoffID, requesterEmail, sessionID) => {
			claims.push({ handoffID, sessionID });
			return requesterEmail === claimant ? claimOfTheClaimant : 'refused';
		},
		linkOf: (handoffID) => `https://intern.example.com/handoff/${handoffID}`,
		closeUnfinished: async (handoffID) => {
			closed.push(handoffID);
		}
	};
	return { handoffs, claims, closed };
}

const handoffQuestionDelivery = { ...questionDelivery, isFinal: true, handoffID: 'handoff-1' };

test('a hand-over question is posted with its link, and the hand-over is claimed for the session that asked', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const handover = handoffsFor(sampleRequester.email);
	const client = aClientFor(agent.socketPath, conversation, { handoffs: handover.handoffs });
	await aSessionOpenedBy(client);

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '아래 링크에서 직접 로그인해 주세요.' }, handoffQuestionDelivery);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the hand-over question');

	expect(handover.claims).toEqual([{ handoffID: 'handoff-1', sessionID: 'session-1' }]);
	expect(conversation.posted[0]?.message).toBe('아래 링크에서 직접 로그인해 주세요.\n\nhttps://intern.example.com/handoff/handoff-1');
	expect(agent.deliveryReports[0].method).toBe(deliveredExtensionMethod);
});

test('a hand-over the session’s requester cannot claim is never posted and is reported undelivered', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation, { handoffs: handoffsFor('other@example.test').handoffs });
	await aSessionOpenedBy(client);

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '로그인해 주세요.' }, handoffQuestionDelivery);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the hand-over question');

	expect(conversation.posted).toEqual([]);
	expect(agent.deliveryReports[0].method).toBe(undeliveredExtensionMethod);
	expect(String(agent.deliveryReports[0].params.reason)).toContain('handoff-1');
});

test('a hand-over that had already ended is asked about in words, with no link to a page that is gone', async () => {
	const agent = anAgentOnASocket({});
	const conversation = aConversation();
	const client = aClientFor(agent.socketPath, conversation, { handoffs: handoffsFor(sampleRequester.email, 'ended').handoffs });
	await aSessionOpenedBy(client);

	await agent.speakWithNoTurnOpen('session-1', { type: 'text', text: '로그인해 주세요.' }, handoffQuestionDelivery);
	await waitUntil(() => agent.deliveryReports.length === 1, 'the relay to report the hand-over question');

	expect(conversation.posted.map((posted) => posted.message)).toEqual(['로그인해 주세요.']);
});

test('a hand-over that ends is reported to blueclaw by its id and the option it ended on', async () => {
	const agent = anAgentOnASocket({});
	const client = aClientFor(agent.socketPath, aConversation());
	await aSessionOpenedBy(client);

	await client.reportHandoffEnded({ handoffID: 'handoff-1', sessionID: 'session-1', isHandedBack: true });
	await client.reportHandoffEnded({ handoffID: 'handoff-2', sessionID: 'session-1', isHandedBack: false });

	expect(agent.deliveryReports).toEqual([
		{ method: handoffEndedExtensionMethod, params: { sessionId: 'session-1', handoffID: 'handoff-1', optionKey: handedBackOptionKey } },
		{ method: handoffEndedExtensionMethod, params: { sessionId: 'session-1', handoffID: 'handoff-2', optionKey: abandonedOptionKey } }
	]);
});

test('a hand-over end blueclaw refuses is said once and not sent again', async () => {
	const agent = anAgentOnASocket({ refusesHandoffEnds: true });
	const reported: string[] = [];
	const client = aClientFor(agent.socketPath, aConversation(), { report: (line) => reported.push(line) });
	await aSessionOpenedBy(client);

	await client.reportHandoffEnded({ handoffID: 'handoff-1', sessionID: 'session-1', isHandedBack: true });

	expect(agent.deliveryReports).toHaveLength(1);
	expect(reported.some((line) => line.includes('handoff-1'))).toBe(true);
});

test('a hand-over blueclaw says was left unfinished is closed here, and nothing is reported back', async () => {
	const agent = anAgentOnASocket({});
	const handover = handoffsFor(sampleRequester.email);
	const client = aClientFor(agent.socketPath, aConversation(), { handoffs: handover.handoffs });
	await aSessionOpenedBy(client);
	const reportsBefore = agent.deliveryReports.length;

	await agent.notifyWithNoTurnOpen(handoffClosedExtensionMethod, { handoffID: 'handoff-1' });
	await waitUntil(() => handover.closed.length === 1, 'the hand-over to be closed');

	expect(handover.closed).toEqual(['handoff-1']);
	expect(agent.deliveryReports).toHaveLength(reportsBefore);
});

test('a notification this relay does not know closes nothing', async () => {
	const agent = anAgentOnASocket({ reply: '네' });
	const handover = handoffsFor(sampleRequester.email);
	const client = aClientFor(agent.socketPath, aConversation(), { handoffs: handover.handoffs });
	await aSessionOpenedBy(client);

	await agent.notifyWithNoTurnOpen('_kim.intern/somethingElse', { handoffID: 'handoff-1' });
	await client.ask(sampleRequester, sampleAddressing, '두 번째');

	expect(handover.closed).toEqual([]);
});
