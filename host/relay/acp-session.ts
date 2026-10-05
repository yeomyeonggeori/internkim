import type { McpServerEntry } from './record-catalog';
import {
	ClientSideConnection,
	ndJsonStream,
	PROTOCOL_VERSION,
	RequestError,
	type Client,
	type ContentBlock,
	type RequestPermissionRequest,
	type RequestPermissionResponse,
	type SessionNotification,
	type StopReason
} from '@agentclientprotocol/sdk';
import { fileURLToPath } from 'node:url';
import type { KeptAttachment, WorkspaceFile } from './file-transfer';
import type { HeldSessionStore } from './held-session-store';
import { ToolProgress, toolCallProgressOf, toolCallStartKind, toolCallUpdateKind } from './tool-progress';

export const defaultBlueclawACPSocketPath = '/run/internkim/acp/blueclaw-acp.sock';
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
export const deliveryMetaKey = 'kim.intern/delivery';
export const deliveredExtensionMethod = '_kim.intern/delivered';
export const undeliveredExtensionMethod = '_kim.intern/undelivered';

export type Delivery = {
	deliveryID?: string;
	replyTargetID?: string;
	alreadyPosted?: boolean;
	final?: boolean;
};

export class AgentUnreachable extends Error {
	constructor(cause: unknown) {
		super(`the agent could not be reached: ${String(cause)}`, { cause });
		this.name = 'AgentUnreachable';
	}
}

async function fromTheAgent<T>(request: Promise<T>): Promise<T> {
	try {
		return await request;
	} catch (failure) {
		if (failure instanceof RequestError) throw failure;
		throw new AgentUnreachable(failure);
	}
}

const firstReconnectDelayMilliseconds = 250;
const longestReconnectDelayMilliseconds = 5_000;

export type PostToConversation = (addressing: Addressing, message: string, attachments?: KeptAttachment[]) => Promise<string>;

export type PostFileToConversation = (addressing: Addressing, requesterEmail: string, file: WorkspaceFile) => Promise<string>;

export type EditInConversation = (addressing: Addressing, messageID: string, message: string) => Promise<void>;

export type ACPSessionSettings = {
	socketPath: string;
	workspaceRootPath: string;
	catalogFor: (requesterEmail: string, conversationID: string) => McpServerEntry[];
	postToConversation: PostToConversation;
	postFileToConversation: PostFileToConversation;
	editInConversation: EditInConversation;
	sessions: HeldSessionStore;
	permissionWasOpened: (conversationID: string) => void;
	report?: (line: string) => void;
};

type PostOutcome = { messageID: string } | { reason: string };

export type HeldSession = {
	sessionID: string;
	requester: Requester;
	addressing: Addressing;
};

type OpenPermission = {
	toolCallID: string;
	parkedMessageIDs: Set<string>;
	select: (optionID: string) => void;
};

export class BlueclawACPClient {
	private readonly settings: ACPSessionSettings;
	private connection: ClientSideConnection | null = null;
	private connecting: Promise<ClientSideConnection> | null = null;
	private closeSocket: (() => void) | null = null;
	private isClosed = false;
	private readonly heldByConversation = new Map<string, HeldSession>();
	private readonly openPermissions = new Map<string, OpenPermission>();
	private readonly sessionOfPromptInFlight = new Map<string, string>();
	private readonly toolProgress: ToolProgress;

	constructor(settings: ACPSessionSettings) {
		this.settings = settings;
		this.toolProgress = new ToolProgress({
			postToConversation: settings.postToConversation,
			editInConversation: settings.editInConversation,
			report: settings.report
		});
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
		this.openPermissions.clear();
	}

	async restoreHeldSessions(): Promise<void> {
		for (const held of await this.settings.sessions.all()) {
			this.heldByConversation.set(held.addressing.conversationID, held);
		}
		void this.agent().catch(() => this.reconnectUntilItComesBack());
	}

	async ask(
		requester: Requester,
		addressing: Addressing,
		message: string,
		facts?: MessageFacts
	): Promise<StopReason> {
		const agent = await fromTheAgent(this.agent());
		const sessionID = await fromTheAgent(this.sessionFor(agent, requester, addressing));
		const messageID = facts?.messageID;
		if (messageID !== undefined) this.sessionOfPromptInFlight.set(messageID, sessionID);
		try {
			const answer = await fromTheAgent(
				agent.prompt({
					sessionId: sessionID,
					prompt: [{ type: 'text', text: message }],
					...(facts ? { _meta: { [messageMetaKey]: messageMetaFrom(facts) } } : {})
				})
			);
			return answer.stopReason;
		} finally {
			if (messageID !== undefined) this.sessionOfPromptInFlight.delete(messageID);
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
		const connection: ClientSideConnection = new ClientSideConnection(
			() => this.asTheClient(() => connection),
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
		await this.settings.sessions.keep(heldSession);
		return opened.sessionId;
	}

	private asTheClient(readConnection: () => ClientSideConnection): Client {
		return {
			sessionUpdate: (notification: SessionNotification) => this.readUpdate(notification),
			requestPermission: (request: RequestPermissionRequest) => this.answerPermission(request, readConnection())
		};
	}

	private readUpdate(notification: SessionNotification): void {
		const update = notification.update;
		if (update.sessionUpdate === 'agent_thought_chunk' && update.content.type === 'text') {
			this.settings.report?.(`progress: ${update.content.text}`);
			return;
		}
		if (update.sessionUpdate === toolCallStartKind || update.sessionUpdate === toolCallUpdateKind) {
			void this.showToolProgress(notification.sessionId, update, deliveryOf(notification._meta));
			return;
		}
		if (update.sessionUpdate !== 'agent_message_chunk') return;
		void this.deliverChunk(notification.sessionId, update.content, deliveryOf(notification._meta));
	}

	private async showToolProgress(
		sessionID: string,
		call: Parameters<typeof toolCallProgressOf>[0],
		delivery: Delivery
	): Promise<void> {
		const held = this.heldSessionOf(sessionID);
		if (!held || !delivery.deliveryID) {
			this.settings.report?.(`progress for ${call.toolCallId} has no conversation or delivery to show in, so it was not shown`);
			return;
		}
		await this.toolProgress.show(delivery.deliveryID, addressedBy(held, delivery), toolCallProgressOf(call));
	}

	private async deliverChunk(sessionID: string, content: ContentBlock, delivery: Delivery): Promise<void> {
		const held = this.heldSessionOf(sessionID);
		const outcome = held
			? await attemptPost(() => this.post(content, held.requester, addressedBy(held, delivery), delivery))
			: { reason: `this relay holds no session ${sessionID}, so it has no conversation to post in` };
		await this.tellTheAgent(delivery, outcome);
	}

	private async post(
		content: ContentBlock,
		requester: Requester,
		addressing: Addressing,
		delivery: Delivery
	): Promise<string> {
		if (content.type === 'text') {
			const progressMessageID =
				delivery.final && delivery.deliveryID
					? await this.toolProgress.replaceWith(delivery.deliveryID, addressing, content.text)
					: undefined;
			return progressMessageID ?? this.settings.postToConversation(addressing, content.text);
		}
		if (content.type === 'resource_link') {
			return this.settings.postFileToConversation(addressing, requester.email, workspaceFileOf(content));
		}
		throw new Error(`this relay posts text and files, so a ${content.type} block did not reach the person`);
	}

	private async tellTheAgent(delivery: Delivery, outcome: PostOutcome): Promise<void> {
		if ('reason' in outcome) this.settings.report?.(`a message did not reach the person: ${outcome.reason}`);
		if (!delivery.deliveryID) return;
		const [method, report] =
			'messageID' in outcome
				? [deliveredExtensionMethod, deliveredReport(delivery.deliveryID, outcome.messageID)]
				: [undeliveredExtensionMethod, undeliveredReport(delivery.deliveryID, outcome.reason)];
		try {
			const agent = await this.agent();
			await agent.request(method, report);
		} catch (failure) {
			this.settings.report?.(`blueclaw was not told what became of ${delivery.deliveryID}: ${String(failure)}`);
		}
	}

	hasOpenPermissionIn(conversationID: string): boolean {
		const held = this.heldByConversation.get(conversationID);
		return held !== undefined && this.openPermissions.has(held.sessionID);
	}

	isParkedOnPermission(conversationID: string, messageID: string): boolean {
		const held = this.heldByConversation.get(conversationID);
		return held !== undefined && this.openPermissions.get(held.sessionID)?.parkedMessageIDs.has(messageID) === true;
	}

	async answerOpenPermission(addressing: Addressing, messageID: string, reply: string): Promise<boolean> {
		const held = this.heldByConversation.get(addressing.conversationID);
		const open = held && this.openPermissions.get(held.sessionID);
		if (!held || !open) return false;
		const optionID = await this.readApprovalReply(
			approvalReplyRequest(held.sessionID, open.toolCallID, reply, messageID, addressing)
		);
		if (optionID === undefined) return false;
		this.openPermissions.delete(held.sessionID);
		open.select(optionID);
		return true;
	}

	private async readApprovalReply(request: Record<string, unknown>): Promise<string | undefined> {
		try {
			const agent = await this.agent();
			return optionChosenIn(await agent.request<Record<string, unknown>>(approvalReplyExtensionMethod, request));
		} catch (failure) {
			this.settings.report?.(`blueclaw could not say whether a message answers the open permission: ${String(failure)}`);
			return undefined;
		}
	}

	private async answerPermission(
		request: RequestPermissionRequest,
		connectionThatAsked: ClientSideConnection
	): Promise<RequestPermissionResponse> {
		const delivery = deliveryOf(request._meta);
		const held = this.heldSessionOf(request.sessionId);
		const outcome: PostOutcome | undefined = !held
			? { reason: `this relay holds no session ${request.sessionId}, so it has no conversation to ask in` }
			: delivery.alreadyPosted
				? undefined
				: await attemptPost(() => this.settings.postToConversation(addressedBy(held, delivery), request.toolCall.title ?? ''));
		if (outcome) await this.tellTheAgent(delivery, outcome);
		if (!held || (outcome && 'reason' in outcome)) return cancelled;
		return this.waitForAnAnswer(request, held, delivery, connectionThatAsked);
	}

	private waitForAnAnswer(
		request: RequestPermissionRequest,
		held: HeldSession,
		delivery: Delivery,
		connectionThatAsked: ClientSideConnection
	): Promise<RequestPermissionResponse> {
		const { promise, resolve } = Promise.withResolvers<RequestPermissionResponse>();
		const open: OpenPermission = {
			toolCallID: request.toolCall.toolCallId,
			parkedMessageIDs: new Set(delivery.alreadyPosted ? [] : this.messageIDsInFlightIn(request.sessionId)),
			select: (optionID) => resolve({ outcome: { outcome: 'selected', optionId: optionID } })
		};
		this.openPermissions.set(request.sessionId, open);
		connectionThatAsked.signal.addEventListener(
			'abort',
			() => {
				if (this.openPermissions.get(request.sessionId) === open) this.openPermissions.delete(request.sessionId);
				this.settings.report?.(`permission ${open.toolCallID} was cancelled: blueclaw left the socket before it was answered`);
				resolve(cancelled);
			},
			{ once: true }
		);
		this.settings.permissionWasOpened(held.addressing.conversationID);
		return promise;
	}

	private messageIDsInFlightIn(sessionID: string): string[] {
		return [...this.sessionOfPromptInFlight].filter(([, inSession]) => inSession === sessionID).map(([messageID]) => messageID);
	}

	private heldSessionOf(sessionID: string): HeldSession | undefined {
		return [...this.heldByConversation.values()].find((held) => held.sessionID === sessionID);
	}
}

const cancelled: RequestPermissionResponse = { outcome: { outcome: 'cancelled' } };

async function attemptPost(post: () => Promise<string>): Promise<PostOutcome> {
	try {
		return { messageID: await post() };
	} catch (failure) {
		return { reason: String(failure) };
	}
}

export function deliveryOf(meta: Record<string, unknown> | null | undefined): Delivery {
	const carried = meta?.[deliveryMetaKey];
	if (typeof carried !== 'object' || carried === null) return {};
	const deliveryID = nonEmptyStringIn(carried, 'deliveryID');
	const replyTargetID = nonEmptyStringIn(carried, 'replyTargetID');
	return {
		...(deliveryID ? { deliveryID } : {}),
		...(replyTargetID ? { replyTargetID } : {}),
		...(Reflect.get(carried, 'alreadyPosted') === true ? { alreadyPosted: true } : {}),
		...(Reflect.get(carried, 'final') === true ? { final: true } : {})
	};
}

function nonEmptyStringIn(carried: object, name: string): string | undefined {
	const value: unknown = Reflect.get(carried, name);
	return typeof value === 'string' && value !== '' ? value : undefined;
}

export function deliveredReport(deliveryID: string, messageID: string): Record<string, string> {
	return { deliveryID, messageID };
}

export function undeliveredReport(deliveryID: string, reason: string): Record<string, string> {
	return { deliveryID, reason };
}

export function approvalReplyRequest(
	sessionID: string,
	toolCallID: string,
	reply: string,
	messageID: string,
	addressing: Addressing
): Record<string, unknown> {
	return {
		sessionId: sessionID,
		toolCallId: toolCallID,
		reply,
		messageId: messageID,
		replyTargetId: addressing.replyTargetID,
		isThread: addressing.isThread
	};
}

export function optionChosenIn(answer: Record<string, unknown>): string | undefined {
	if (answer.isAnswer !== true) return undefined;
	const optionID = answer.optionId;
	if (typeof optionID !== 'string') throw new Error(`blueclaw read the reply as an answer to no option: ${JSON.stringify(answer)}`);
	return optionID;
}

function addressedBy(held: HeldSession, delivery: Delivery): Addressing {
	if (!delivery.replyTargetID) return held.addressing;
	return { ...held.addressing, replyTargetID: delivery.replyTargetID };
}

const unnamedContentType = 'application/octet-stream';

function workspaceFileOf(link: { name: string; uri: string; mimeType?: string | null }): WorkspaceFile {
	if (!URL.canParse(link.uri) || new URL(link.uri).protocol !== 'file:') {
		throw new Error(`the agent named ${link.name} by ${link.uri}, which is not a file on this computer`);
	}
	return { filename: link.name, workspacePath: fileURLToPath(link.uri), contentType: link.mimeType || unnamedContentType };
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
