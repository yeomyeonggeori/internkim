import type { McpServerEntry } from './record-catalog';
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
import type { HeldQuestion, HeldQuestionStore } from './held-question-store';

export const defaultBlueclawACPSocketPath = '/run/internkim/blueclaw-acp.sock';
export const sessionMetaKey = 'kim.intern/session';
export const messageMetaKey = 'kim.intern/message';

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

export type MessageFacts = {
	messageID: string;
	replyTargetID?: string;
	isThread?: boolean;
	context: Record<string, unknown>;
};

export const approvalReplyExtensionMethod = '_kim.intern/approvalReply';

const firstReconnectDelayMilliseconds = 250;
const longestReconnectDelayMilliseconds = 5_000;

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
	catalogFor: (requesterEmail: string, conversationID: string) => McpServerEntry[];
	questions: HeldQuestionStore;
	/** Puts the question to the requester and answers with the words they wrote back. */
	askThePerson: (asked: AskedPermission, addressing: Addressing) => Promise<string>;
	/** Waits for the answer to a question already asked before a restart, without asking again. */
	awaitAnAlreadyAskedQuestion: (addressing: Addressing) => Promise<string>;
	report?: (line: string) => void;
};

type OpenTurn = {
	messageSegments: string[];
	progress: string[];
};

type HeldSession = {
	sessionID: string;
	requester: Requester;
	addressing: Addressing;
};

export class BlueclawACPClient {
	private readonly settings: ACPSessionSettings;
	private connection: ClientSideConnection | null = null;
	private connecting: Promise<ClientSideConnection> | null = null;
	private closeSocket: (() => void) | null = null;
	private isClosed = false;
	private readonly heldByConversation = new Map<string, HeldSession>();
	private readonly heldBySession = new Map<string, HeldSession>();
	private readonly turnBySession = new Map<string, OpenTurn>();
	/**
	 * The words the person answered with, not the agent's reading of them: a
	 * restarted agent asks again and has to be told the same thing, and only the
	 * agent decides what it meant.
	 */
	private readonly answeredPermissions = new Map<string, Promise<string>>();

	constructor(settings: ACPSessionSettings) {
		this.settings = settings;
	}

	async connect(): Promise<void> {
		await this.agent();
	}

	close(): void {
		this.isClosed = true;
		this.closeSocket?.();
		this.closeSocket = null;
		this.connection = null;
		this.connecting = null;
		this.heldByConversation.clear();
		this.heldBySession.clear();
	}

	/**
	 * Reads every question this relay had not delivered before it last stopped,
	 * and puts each conversation back into a state where a re-issued
	 * `RequestPermission` is answered from the store rather than asked again.
	 */
	async restoreOutstandingQuestions(): Promise<void> {
		const stored = await this.settings.questions.all();
		if (stored.length === 0) return;
		for (const held of stored) {
			const heldSession: HeldSession = {
				sessionID: held.sessionID,
				requester: held.requester,
				addressing: held.addressing
			};
			this.heldByConversation.set(held.addressing.conversationID, heldSession);
			this.heldBySession.set(held.sessionID, heldSession);
			this.answeredPermissions.set(held.toolCallID, this.answerFromStoreOrPerson(held));
		}
		// The reconnect makes blueclaw call session/load for the sessions just
		// restored, which is what makes it re-issue the calls it stopped on. It is
		// not awaited: the relay answers /inbound for every other conversation
		// whether or not blueclaw is up yet.
		void this.agent().catch(() => this.reconnectUntilItComesBack());
	}

	private answerFromStoreOrPerson(held: HeldQuestion): Promise<string> {
		if (held.answer !== undefined) return Promise.resolve(held.answer);
		return this.settings.awaitAnAlreadyAskedQuestion(held.addressing).then(async (words) => {
			await this.settings.questions.answer(held.toolCallID, words);
			return words;
		});
	}

	async ask(
		requester: Requester,
		addressing: Addressing,
		message: string,
		facts?: MessageFacts
	): Promise<AnsweredTurn> {
		const agent = await this.agent();
		const sessionID = await this.sessionFor(agent, requester, addressing);
		const openTurn: OpenTurn = { messageSegments: [], progress: [] };
		this.turnBySession.set(sessionID, openTurn);
		try {
			const answer = await agent.prompt({
				sessionId: sessionID,
				prompt: [{ type: 'text', text: message }],
				...(facts ? { _meta: { [messageMetaKey]: messageMetaFrom(facts) } } : {})
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
		if (this.connection) return this.connection;
		if (!this.connecting) this.connecting = this.openConnection();
		try {
			return await this.connecting;
		} finally {
			this.connecting = null;
		}
	}

	private async openConnection(): Promise<ClientSideConnection> {
		const socket = await unixSocketStreams(this.settings.socketPath, () => this.socketEnded());
		this.closeSocket = socket.close;
		const connection = new ClientSideConnection(
			() => this.asTheClient(),
			ndJsonStream(socket.writable, socket.readable)
		);
		await connection.initialize({
			protocolVersion: PROTOCOL_VERSION,
			clientCapabilities: { fs: { readTextFile: false, writeTextFile: false } }
		});
		this.connection = connection;
		await this.loadHeldSessions(connection);
		return connection;
	}

	private socketEnded(): void {
		if (this.isClosed) return;
		if (!this.connection) return;
		this.connection = null;
		this.settings.report?.('blueclaw left the socket; reconnecting');
		void this.reconnectUntilItComesBack();
	}

	/**
	 * A restarting daemon is not listening yet, and its socket file is removed
	 * and remade, so the first attempt lands on nothing.
	 */
	private async reconnectUntilItComesBack(): Promise<void> {
		for (
			let delay = firstReconnectDelayMilliseconds;
			!this.isClosed && !this.connection;
			delay = Math.min(delay * 2, longestReconnectDelayMilliseconds)
		) {
			try {
				await this.agent();
				this.settings.report?.('blueclaw is back');
				return;
			} catch (failure) {
				this.settings.report?.(`blueclaw is not back yet: ${String(failure)}`);
				await Bun.sleep(delay);
			}
		}
	}

	/**
	 * A daemon that restarted knows none of these; loading them is what makes it
	 * ask again about the calls it stopped on.
	 */
	private async loadHeldSessions(connection: ClientSideConnection): Promise<void> {
		for (const held of [...this.heldByConversation.values()]) {
			await connection
				.loadSession({
					sessionId: held.sessionID,
					cwd: this.settings.workspaceRootPath,
					mcpServers: this.settings.catalogFor(
						held.requester.email,
						held.addressing.conversationID
					),
					_meta: { [sessionMetaKey]: { requester: held.requester, addressing: held.addressing } }
				})
				.catch((failure) => {
					this.settings.report?.(`session ${held.sessionID} would not load: ${String(failure)}`);
					this.heldByConversation.delete(held.addressing.conversationID);
				});
		}
	}

	private async sessionFor(
		agent: ClientSideConnection,
		requester: Requester,
		addressing: Addressing
	): Promise<string> {
		const held = this.heldByConversation.get(addressing.conversationID);
		if (held) return held.sessionID;
		const opened = await agent.newSession({
			cwd: this.settings.workspaceRootPath,
			mcpServers: this.settings.catalogFor(requester.email, addressing.conversationID),
			_meta: { [sessionMetaKey]: { requester, addressing } }
		});
		const heldSession: HeldSession = { sessionID: opened.sessionId, requester, addressing };
		this.heldByConversation.set(addressing.conversationID, heldSession);
		this.heldBySession.set(opened.sessionId, heldSession);
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

	private async answerPermission(
		request: RequestPermissionRequest
	): Promise<RequestPermissionResponse> {
		const held = this.heldSessionOf(request.sessionId);
		if (!held) return { outcome: { outcome: 'cancelled' } };

		const toolCallID = request.toolCall.toolCallId;
		const question = request.toolCall.title ?? '';
		const alreadyAsked = this.answeredPermissions.get(toolCallID);
		const asking = alreadyAsked ?? this.askAndPersist(toolCallID, question, held);
		this.answeredPermissions.set(toolCallID, asking);
		const connectionThatAsked = this.connection;
		try {
			const words = await asking;
			// The daemon that asked this is gone; the one that replaced it asked
			// again, and that request is the one worth answering.
			if (this.connection !== connectionThatAsked) return { outcome: { outcome: 'cancelled' } };
			const optionID = await this.readApprovalReply(request.sessionId, toolCallID, words);
			this.answeredPermissions.delete(toolCallID);
			await this.settings.questions.forget(toolCallID);
			return { outcome: { outcome: 'selected', optionId: optionID } };
		} catch (failure) {
			this.settings.report?.(`nobody answered ${toolCallID}: ${String(failure)}`);
			return { outcome: { outcome: 'cancelled' } };
		}
	}

	/**
	 * The only path that puts a genuinely new question to the person: it records
	 * the question before asking, and the answer as soon as it has one, so a
	 * relay that stops between either step finds them on disk when it starts
	 * again.
	 */
	private async askAndPersist(toolCallID: string, question: string, held: HeldSession): Promise<string> {
		await this.settings.questions.keep({
			toolCallID,
			sessionID: held.sessionID,
			requester: held.requester,
			addressing: held.addressing,
			question,
			askedAt: new Date().toISOString()
		});
		const words = await this.settings.askThePerson({ toolCallID, question }, held.addressing);
		await this.settings.questions.answer(toolCallID, words);
		return words;
	}

	private heldSessionOf(sessionID: string): HeldSession | undefined {
		const known = this.heldBySession.get(sessionID);
		if (known) return known;
		for (const held of this.heldByConversation.values()) {
			if (held.sessionID !== sessionID) continue;
			this.heldBySession.set(sessionID, held);
			return held;
		}
		return undefined;
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

function messageMetaFrom(facts: MessageFacts): Record<string, unknown> {
	return {
		messageID: facts.messageID,
		...(facts.replyTargetID ? { replyTargetID: facts.replyTargetID } : {}),
		...(facts.isThread ? { isThread: true } : {}),
		context: facts.context
	};
}

type SocketStreams = {
	readable: ReadableStream<Uint8Array>;
	writable: WritableStream<Uint8Array>;
	close: () => void;
};

async function unixSocketStreams(
	socketPath: string,
	whenEnded: () => void
): Promise<SocketStreams> {
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
				whenEnded();
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
