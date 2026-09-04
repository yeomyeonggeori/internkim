import { afterEach, expect, test } from 'bun:test';
import {
	AgentSideConnection,
	ndJsonStream,
	PROTOCOL_VERSION,
	type Agent,
	type NewSessionRequest,
	type PromptRequest
} from '@agentclientprotocol/sdk';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { BlueclawACPClient, sessionMetaKey } from './acp-session';

type AnAgentThatRecords = {
	socketPath: string;
	sessionsOpened: NewSessionRequest[];
	promptsTaken: PromptRequest[];
	approvalRepliesRead: string[];
};

type AgentBehaviour = {
	reply?: string;
	askPermissionAbout?: { toolCallID: string; question: string };
	approvalReplies?: { reply: string; optionID: string }[];
};

const cleanUps: (() => void)[] = [];

afterEach(() => {
	while (cleanUps.length) cleanUps.pop()?.();
});

function anAgentOnASocket(behaviour: AgentBehaviour): AnAgentThatRecords {
	const directory = mkdtempSync(join(tmpdir(), 'acp-relay-'));
	const socketPath = join(directory, 'agent.sock');
	const sessionsOpened: NewSessionRequest[] = [];
	const promptsTaken: PromptRequest[] = [];
	const approvalRepliesRead: string[] = [];

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
				return { sessionId: 'session-1' };
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

	return { socketPath, sessionsOpened, promptsTaken, approvalRepliesRead };
}

const sampleRequester = { email: 'sample@example.test', name: '이샘플' };
const sampleAddressing = { platform: 'buzz', conversationID: 'conversation-1' };

test('a session names the requester and the conversation it answers in', async () => {
	const agent = anAgentOnASocket({ reply: '보냈습니다' });
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		askThePerson: async () => ''
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
		askThePerson: async () => ''
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
	const client = new BlueclawACPClient({
		socketPath: agent.socketPath,
		workspaceRootPath: '/workspace',
		askThePerson: async (question) => {
			asked.push(question.question);
			return '응 보내줘';
		}
	});
	cleanUps.push(() => client.close());

	const answered = await client.ask(sampleRequester, sampleAddressing, '박예시한테 DM 보내줘');

	expect(asked).toEqual(['박예시에게 보낼까요?']);
	// The relay carries the words and reads none of them: the option comes back
	// from the agent, which is the only side allowed to judge what they meant.
	expect(agent.approvalRepliesRead).toEqual(['응 보내줘']);
	expect(answered.reply).toBe('보냈습니다');
});
