import { afterEach, expect, test } from 'bun:test';
import {
	AgentSideConnection,
	ndJsonStream,
	PROTOCOL_VERSION,
	RequestError,
	type Agent,
	type LoadSessionRequest,
	type NewSessionRequest,
	type PromptRequest,
	type RequestPermissionResponse
} from '@agentclientprotocol/sdk';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { AgentUnreachable, BlueclawACPClient, sessionMetaKey, type Addressing } from './acp-session';
import { HeldQuestionStore } from './held-question-store';

function aQuestionStore(directoryPath?: string): HeldQuestionStore {
	return new HeldQuestionStore({ directoryPath: directoryPath ?? mkdtempSync(join(tmpdir(), 'acp-questions-')) });
}

function neverAskedAgain(): (addressing: Addressing) => Promise<string> {
	return () => new Promise<string>(() => {});
}

type AnAgentThatRecords = {
	socketPath: string;
	sessionsOpened: NewSessionRequest[];
	sessionsLoaded: LoadSessionRequest[];
	promptsTaken: PromptRequest[];
	approvalRepliesRead: string[];
	connectionsOpened: number;
	/** Simulates blueclaw re-issuing a call it stopped on, over the connection currently held. */
	reissuePermission: (sessionID: string, toolCallID: string, question: string) => Promise<RequestPermissionResponse>;
};

type AgentBehaviour = {
	reply?: string;
	askPermissionAbout?: { toolCallID: string; question: string };
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
				if (behaviour.askPermissionAbout) {
					await connection.requestPermission({
						sessionId: request.sessionId,
						toolCall: {
							toolCallId: behaviour.askPermissionAbout.toolCallID,
							title: behaviour.askPermissionAbout.question
						},
						options: [
							{ optionId: 'approve_once', kind: 'allow_once', name: 'approve this call' },
							{ optionId: 'reject_once', kind: 'reject_once', name: 'decline this call' }
						]
					});
				}
				if (behaviour.reply) {
					await connection.sessionUpdate({
						sessionId: request.sessionId,
						update: {
							sessionUpdate: 'agent_message_chunk',
							content: { type: 'text', text: behaviour.reply }
						}
					});
				}
				return { stopReason: 'end_turn' };
			},
			cancel: async () => {},
			authenticate: async () => ({}),
			extMethod: async (_method: string, params: Record<string, unknown>) => {
				const reply = String(params.reply ?? '');
				approvalRepliesRead.push(reply);
				const known = behaviour.approvalReplies?.find((answer) => answer.reply === reply);
				return { optionId: known?.optionID ?? 'reject_once' };
			}
		};
	}

	return {
		socketPath,
		sessionsOpened,
		sessionsLoaded,
		promptsTaken,
		approvalRepliesRead,
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
		}
	};
}

async function waitUntil(isReady: () => boolean | Promise<boolean>, waitedFor: string): Promise<void> {
	const deadline = Date.now() + 3_000;
	while (Date.now() < deadline) {
		if (await isReady()) return;
		await Bun.sleep(1);
	}
	throw new Error(`waited too long for ${waitedFor}`);
}

const sampleRequester = { email: 'sample@example.test', name: '이샘플' };
const sampleAddressing = { platform: 'buzz', conversationID: 'conversation-1' };

test('a session names the requester and the conversation it answers in', async () => {
	const agent = anAgentOnASocket({ reply: '보냈습니다' });
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		questions: aQuestionStore(),
		askThePerson: async () => '',
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());

	const answered = await client.ask(sampleRequester, sampleAddressing, '박예시한테 DM 보내줘');

	expect(answered.reply).toBe('보냈습니다');
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
		questions: aQuestionStore(),
		askThePerson: async () => '',
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
		questions,
		askThePerson: async (question) => {
			asked.push(question.question);
			return '응 보내줘';
		},
		awaitAnAlreadyAskedQuestion: neverAskedAgain()
	});
	cleanUps.push(() => client.close());

	const answered = await client.ask(sampleRequester, sampleAddressing, '박예시한테 DM 보내줘');

	expect(asked).toEqual(['박예시에게 보낼까요?']);
	// The relay carries the words and reads none of them: the option comes back
	// from the agent, which is the only side allowed to judge what they meant.
	expect(agent.approvalRepliesRead).toEqual(['응 보내줘']);
	expect(answered.reply).toBe('보냈습니다');
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
		questions,
		askThePerson: async (question) => {
			asked.push(question.question);
			return 'not what a restart should ask again';
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
		questions,
		askThePerson: async (question) => {
			asked.push(question.question);
			return 'not what a restart should ask again';
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

function aClientOn(socketPath: string): BlueclawACPClient {
	const client = new BlueclawACPClient({
		socketPath,
		workspaceRootPath: '/workspace',
		catalogFor: () => [],
		questions: aQuestionStore(),
		askThePerson: async () => '',
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
