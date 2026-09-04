import {
	ClientSideConnection,
	ndJsonStream,
	PROTOCOL_VERSION,
	type Client,
	type RequestPermissionRequest,
	type RequestPermissionResponse,
	type SessionNotification,
	type StopReason
} from '@agentclientprotocol/sdk';

export const defaultBlueclawACPSocketPath = '/run/internkim/blueclaw-acp.sock';
export const sessionMetaKey = 'kim.intern/session';

export type Requester = {
	email: string;
	name?: string;
	callingName?: string;
	handle?: string;
};

export type Addressing = {
	platform: string;
	conversationID: string;
	conversationType?: string;
	replyTargetID?: string;
	isThread?: boolean;
	responseLanguage?: string;
};

export const approvalReplyExtensionMethod = '_kim.intern/approvalReply';

export type AskedPermission = {
	toolCallID: string;
	question: string;
};

export type AnsweredTurn = {
	reply: string;
	progress: string[];
	stopReason: StopReason;
};

export type ACPSessionSettings = {
	socketPath: string;
	workspaceRootPath: string;
	/** Puts the question to the requester and answers with the words they wrote back. */
	askThePerson: (asked: AskedPermission, addressing: Addressing) => Promise<string>;
	report?: (line: string) => void;
};

type OpenTurn = {
	messageSegments: string[];
	progress: string[];
};

export class BlueclawACPClient {
	private readonly settings: ACPSessionSettings;
	private connection: ClientSideConnection | null = null;
	private closeSocket: (() => void) | null = null;
	private readonly sessionByConversation = new Map<string, string>();
	private readonly addressingBySession = new Map<string, Addressing>();
	private readonly turnBySession = new Map<string, OpenTurn>();
	private readonly answeredPermissions = new Map<string, Promise<string>>();

	constructor(settings: ACPSessionSettings) {
		this.settings = settings;
	}

	async connect(): Promise<void> {
		if (this.connection) return;
		const socket = await unixSocketStreams(this.settings.socketPath);
		this.closeSocket = socket.close;
		this.connection = new ClientSideConnection(
			() => this.asTheClient(),
			ndJsonStream(socket.writable, socket.readable)
		);
		await this.connection.initialize({
			protocolVersion: PROTOCOL_VERSION,
			clientCapabilities: { fs: { readTextFile: false, writeTextFile: false } }
		});
	}

	close(): void {
		this.closeSocket?.();
		this.closeSocket = null;
		this.connection = null;
		this.sessionByConversation.clear();
		this.addressingBySession.clear();
	}

	async ask(requester: Requester, addressing: Addressing, message: string): Promise<AnsweredTurn> {
		const agent = await this.agent();
		const sessionID = await this.sessionFor(agent, requester, addressing);
		const openTurn: OpenTurn = { messageSegments: [], progress: [] };
		this.turnBySession.set(sessionID, openTurn);
		try {
			const answer = await agent.prompt({
				sessionId: sessionID,
				prompt: [{ type: 'text', text: message }]
			});
			return {
				reply: openTurn.messageSegments.join(''),
				progress: openTurn.progress,
				stopReason: answer.stopReason
			};
		} finally {
			this.turnBySession.delete(sessionID);
		}
	}

	private async agent(): Promise<ClientSideConnection> {
		await this.connect();
		if (!this.connection) throw new Error(`blueclaw did not answer on ${this.settings.socketPath}`);
		return this.connection;
	}

	private async sessionFor(
		agent: ClientSideConnection,
		requester: Requester,
		addressing: Addressing
	): Promise<string> {
		const held = this.sessionByConversation.get(addressing.conversationID);
		if (held) return held;
		const opened = await agent.newSession({
			cwd: this.settings.workspaceRootPath,
			mcpServers: [],
			_meta: { [sessionMetaKey]: { requester, addressing } }
		});
		this.sessionByConversation.set(addressing.conversationID, opened.sessionId);
		this.addressingBySession.set(opened.sessionId, addressing);
		return opened.sessionId;
	}

	private asTheClient(): Client {
		return {
			sessionUpdate: (notification: SessionNotification) => this.readUpdate(notification),
			requestPermission: (request: RequestPermissionRequest) => this.answerPermission(request)
		};
	}

	private readUpdate(notification: SessionNotification): void {
		const openTurn = this.turnBySession.get(notification.sessionId);
		if (!openTurn) return;
		const update = notification.update;
		if (update.sessionUpdate === 'agent_message_chunk' && update.content.type === 'text') {
			openTurn.messageSegments.push(update.content.text);
			return;
		}
		if (update.sessionUpdate === 'agent_thought_chunk' && update.content.type === 'text') {
			openTurn.progress.push(update.content.text);
		}
	}

	private async answerPermission(request: RequestPermissionRequest): Promise<RequestPermissionResponse> {
		const addressing = this.addressingBySession.get(request.sessionId);
		if (!addressing) return { outcome: { outcome: 'cancelled' } };

		const toolCallID = request.toolCall.toolCallId;
		const question = request.toolCall.title ?? '';
		const asking =
			this.answeredPermissions.get(toolCallID) ??
			this.settings.askThePerson({ toolCallID, question }, addressing);
		this.answeredPermissions.set(toolCallID, asking);
		try {
			const words = await asking;
			const optionID = await this.readApprovalReply(request.sessionId, toolCallID, words);
			return { outcome: { outcome: 'selected', optionId: optionID } };
		} catch (failure) {
			this.answeredPermissions.delete(toolCallID);
			this.settings.report?.(`nobody answered ${toolCallID}: ${String(failure)}`);
			return { outcome: { outcome: 'cancelled' } };
		}
	}

	private async readApprovalReply(
		sessionID: string,
		toolCallID: string,
		reply: string
	): Promise<string> {
		const agent = await this.agent();
		const read = await agent.request<{ optionId: string }>(approvalReplyExtensionMethod, {
			sessionId: sessionID,
			toolCallId: toolCallID,
			reply
		});
		return read.optionId;
	}
}

type SocketStreams = {
	readable: ReadableStream<Uint8Array>;
	writable: WritableStream<Uint8Array>;
	close: () => void;
};

async function unixSocketStreams(socketPath: string): Promise<SocketStreams> {
	let deliver: (chunk: Uint8Array) => void = () => {};
	let finish: () => void = () => {};
	let isFinished = false;
	const readable = new ReadableStream<Uint8Array>({
		start(controller) {
			deliver = (chunk) => controller.enqueue(chunk);
			finish = () => {
				if (isFinished) return;
				isFinished = true;
				controller.close();
			};
		}
	});
	const socket = await Bun.connect({
		unix: socketPath,
		socket: {
			data: (_socket, chunk) => deliver(new Uint8Array(chunk)),
			close: () => finish(),
			error: () => finish()
		}
	});
	const writable = new WritableStream<Uint8Array>({
		write(chunk) {
			socket.write(chunk);
		},
		close() {
			socket.end();
		}
	});
	return { readable, writable, close: () => socket.end() };
}
