import { doublingDelayMilliseconds } from './failure';
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
import { randomUUID } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import type { KeptAttachment, WorkspaceFile } from './file-transfer';
import type { SessionBindingStore } from './session-binding-store';
import { ToolProgress, progressOfToolCall, startedToolCallKind, updatedToolCallKind } from './tool-progress';

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
	isAlreadyPosted?: boolean;
	isFinal?: boolean;
	handoffID?: string;
};

export type HandoffClaim = 'open' | 'ended' | 'refused';

export type HandoffsForQuestions = {
	claim: (handoffID: string, requesterEmail: string, settle: () => void) => Promise<HandoffClaim>;
	linkOf: (handoffID: string) => string;
	answeredInTheThread: (handoffID: string, isDone: boolean) => Promise<void>;
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

export type DeleteInConversation = (addressing: Addressing, messageID: string) => Promise<void>;

export type ACPSessionSettings = {
	socketPath: string;
	workspaceRootPath: string;
	catalogFor: (requesterEmail: string, catalogTicket: string) => McpServerEntry[];
	postToConversation: PostToConversation;
	postFileToConversation: PostFileToConversation;
	editInConversation: EditInConversation;
	deleteInConversation: DeleteInConversation;
	sessions: SessionBindingStore;
	approvalWasRequested: (conversationID: string) => void;
	handoffs?: HandoffsForQuestions;
	report?: (line: string) => void;
};

type PostOutcome = { messageID: string } | { reason: string };

export type SessionBinding = {
	sessionID: string;
	requester: Requester;
	addressing: Addressing;
	catalogTicket: string;
};

type PendingApproval = {
	toolCallID: string;
	waitingMessageIDs: Set<string>;
	select: (optionID: string) => void;
	handoff?: { handoffID: string; handedBackOptionID: string };
};

export class BlueclawACPClient {
	private readonly settings: ACPSessionSettings;
	private connection: ClientSideConnection | null = null;
	private connecting: Promise<ClientSideConnection> | null = null;
	private closeSocket: (() => void) | null = null;
	private isClosed = false;
	private readonly bindingBySpeaker = new Map<string, SessionBinding>();
	private readonly pendingApprovals = new Map<string, PendingApproval>();
	private readonly sessionOfPromptInFlight = new Map<string, string>();
	private readonly toolProgress: ToolProgress;

	constructor(settings: ACPSessionSettings) {
		this.settings = settings;
		this.toolProgress = new ToolProgress({
			postToConversation: settings.postToConversation,
			editInConversation: settings.editInConversation,
			deleteInConversation: settings.deleteInConversation,
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
		this.bindingBySpeaker.clear();
		this.pendingApprovals.clear();
	}

	async restoreSessionBindings(): Promise<void> {
		for (const binding of await this.settings.sessions.list()) {
			this.bindingBySpeaker.set(speakerKeyOf(binding), binding);
			this.catalogOf(binding);
		}
		void this.agent().catch(() => this.reconnectUntilItComesBack());
	}

	private catalogOf(binding: SessionBinding): McpServerEntry[] {
		return this.settings.catalogFor(binding.requester.email, binding.catalogTicket);
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
		await this.loadSessionBindings(connection);
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
		for (let failures = 1; !this.isClosed && !this.connection; failures++) {
			try {
				await this.agent();
				this.settings.report?.('blueclaw is back');
				return;
			} catch (failure) {
				this.settings.report?.(`blueclaw is not back yet: ${String(failure)}`);
				await Bun.sleep(doublingDelayMilliseconds(failures, firstReconnectDelayMilliseconds, longestReconnectDelayMilliseconds));
			}
		}
	}

	private async loadSessionBindings(connection: ClientSideConnection): Promise<void> {
		for (const binding of [...this.bindingBySpeaker.values()]) {
			await connection
				.loadSession({
					sessionId: binding.sessionID,
					cwd: this.settings.workspaceRootPath,
					mcpServers: this.catalogOf(binding),
					_meta: { [sessionMetaKey]: { requester: binding.requester, addressing: binding.addressing } }
				})
				.catch((failure) => {
					this.settings.report?.(`session ${binding.sessionID} would not load: ${String(failure)}`);
					this.bindingBySpeaker.delete(speakerKeyOf(binding));
				});
		}
	}

	private async sessionFor(
		agent: ClientSideConnection,
		requester: Requester,
		addressing: Addressing
	): Promise<string> {
		const binding = this.bindingBySpeaker.get(speakerKey(addressing.conversationID, requester.email));
		if (binding) return binding.sessionID;
		const catalogTicket = randomUUID();
		const opened = await agent.newSession({
			cwd: this.settings.workspaceRootPath,
			mcpServers: this.settings.catalogFor(requester.email, catalogTicket),
			_meta: { [sessionMetaKey]: { requester, addressing } }
		});
		const sessionBinding: SessionBinding = { sessionID: opened.sessionId, requester, addressing, catalogTicket };
		this.bindingBySpeaker.set(speakerKeyOf(sessionBinding), sessionBinding);
		await this.settings.sessions.save(sessionBinding);
		return opened.sessionId;
	}

	private asTheClient(readConnection: () => ClientSideConnection): Client {
		return {
			sessionUpdate: (notification: SessionNotification) => this.readUpdate(notification),
			requestPermission: (request: RequestPermissionRequest) => this.answerApprovalRequest(request, readConnection())
		};
	}

	private readUpdate(notification: SessionNotification): void {
		const update = notification.update;
		if (update.sessionUpdate === 'agent_thought_chunk' && update.content.type === 'text') {
			void this.showCheckpoint(notification.sessionId, update.content.text, deliveryOf(notification._meta));
			return;
		}
		if (update.sessionUpdate === startedToolCallKind || update.sessionUpdate === updatedToolCallKind) {
			void this.showToolProgress(notification.sessionId, update, deliveryOf(notification._meta));
			return;
		}
		if (update.sessionUpdate !== 'agent_message_chunk') return;
		void this.deliverChunk(notification.sessionId, update.content, deliveryOf(notification._meta));
	}

	private async showToolProgress(
		sessionID: string,
		call: Parameters<typeof progressOfToolCall>[0],
		delivery: Delivery
	): Promise<void> {
		const binding = this.sessionBindingOf(sessionID);
		if (!binding || !delivery.deliveryID) {
			this.settings.report?.(`progress for ${call.toolCallId} has no conversation or delivery to show in, so it was not shown`);
			return;
		}
		await this.toolProgress.showToolCall(delivery.deliveryID, addressedBy(binding, delivery), progressOfToolCall(call));
	}

	private async showCheckpoint(sessionID: string, checkpoint: string, delivery: Delivery): Promise<void> {
		const binding = this.sessionBindingOf(sessionID);
		if (!binding || !delivery.deliveryID) {
			this.settings.report?.(`progress: ${checkpoint}`);
			return;
		}
		await this.toolProgress.showCheckpoint(delivery.deliveryID, addressedBy(binding, delivery), checkpoint);
	}

	private async deliverChunk(sessionID: string, content: ContentBlock, delivery: Delivery): Promise<void> {
		const binding = this.sessionBindingOf(sessionID);
		const outcome = binding
			? await attemptPost(() => this.post(content, binding.requester, addressedBy(binding, delivery), delivery))
			: { reason: `this relay holds no session ${sessionID}, so it has no conversation to post in` };
		await this.tellTheAgent(delivery, outcome);
	}

	private async post(
		content: ContentBlock,
		requester: Requester,
		addressing: Addressing,
		delivery: Delivery
	): Promise<string> {
		const post = this.posterOf(content, requester, addressing);
		if (!delivery.isFinal || !delivery.deliveryID) return post();
		return this.toolProgress.postFinal(delivery.deliveryID, addressing, post);
	}

	private posterOf(content: ContentBlock, requester: Requester, addressing: Addressing): () => Promise<string> {
		if (content.type === 'text') return () => this.settings.postToConversation(addressing, content.text);
		if (content.type === 'resource_link') {
			return () => this.settings.postFileToConversation(addressing, requester.email, workspaceFileOf(content));
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

	hasPendingApprovalIn(conversationID: string, requesterEmail: string): boolean {
		const binding = this.bindingBySpeaker.get(speakerKey(conversationID, requesterEmail));
		return binding !== undefined && this.pendingApprovals.has(binding.sessionID);
	}

	isWaitingOnApproval(conversationID: string, messageID: string): boolean {
		return this.bindingsIn(conversationID).some(
			(binding) => this.pendingApprovals.get(binding.sessionID)?.waitingMessageIDs.has(messageID) === true
		);
	}

	private bindingsIn(conversationID: string): SessionBinding[] {
		return [...this.bindingBySpeaker.values()].filter((binding) => binding.addressing.conversationID === conversationID);
	}

	async answerPendingApproval(addressing: Addressing, messageID: string, reply: string, replier: Requester): Promise<boolean> {
		const binding = this.bindingBySpeaker.get(speakerKey(addressing.conversationID, replier.email));
		const pending = binding && this.pendingApprovals.get(binding.sessionID);
		if (!binding || !pending) return false;
		const reading = await this.readApprovalReply(
			approvalReplyRequest(binding.sessionID, pending.toolCallID, reply, messageID, addressing, replier.email)
		);
		if (reading === undefined) return false;
		this.pendingApprovals.delete(binding.sessionID);
		if (pending.handoff) {
			await this.settings.handoffs?.answeredInTheThread(pending.handoff.handoffID, reading.optionID === pending.handoff.handedBackOptionID);
		}
		pending.select(reading.optionID);
		return !reading.isInstruction;
	}

	private async readApprovalReply(request: Record<string, unknown>): Promise<ApprovalReplyReading | undefined> {
		try {
			const agent = await this.agent();
			return readingOf(await agent.request<Record<string, unknown>>(approvalReplyExtensionMethod, request));
		} catch (failure) {
			this.settings.report?.(`blueclaw could not say whether a message answers the pending approval: ${String(failure)}`);
			return undefined;
		}
	}

	private async answerApprovalRequest(
		request: RequestPermissionRequest,
		connectionThatAsked: ClientSideConnection
	): Promise<RequestPermissionResponse> {
		const delivery = deliveryOf(request._meta);
		const binding = this.sessionBindingOf(request.sessionId);
		if (!binding) {
			await this.tellTheAgent(delivery, { reason: `this relay holds no session ${request.sessionId}, so it has no conversation to ask in` });
			return cancelled;
		}
		const approval = this.registerPendingApproval(request, binding, delivery, connectionThatAsked);
		const question = await this.questionToPost(request, binding, delivery, approval.pending);
		if (question === null) {
			approval.withdraw();
			return cancelled;
		}
		if ('handedBackOptionID' in question) {
			approval.withdraw();
			return { outcome: { outcome: 'selected', optionId: question.handedBackOptionID } };
		}
		const outcome = delivery.isAlreadyPosted
			? undefined
			: await attemptPost(() => this.settings.postToConversation(addressedBy(binding, delivery), question.text));
		if (outcome) await this.tellTheAgent(delivery, outcome);
		if (outcome && 'reason' in outcome) {
			approval.withdraw();
			return cancelled;
		}
		this.settings.approvalWasRequested(binding.addressing.conversationID);
		return approval.answer;
	}

	private async questionToPost(
		request: RequestPermissionRequest,
		binding: SessionBinding,
		delivery: Delivery,
		pending: PendingApproval
	): Promise<{ text: string } | { handedBackOptionID: string } | null> {
		const title = request.toolCall.title ?? '';
		if (!delivery.handoffID) return { text: title };
		const handedBack = request.options.find((option) => option.kind === 'allow_once');
		const handoffs = this.settings.handoffs;
		if (!handedBack || !handoffs) {
			this.settings.report?.(`hand-over ${delivery.handoffID} has no way to be handed back here, so it was not put to anyone`);
			return null;
		}
		pending.handoff = { handoffID: delivery.handoffID, handedBackOptionID: handedBack.optionId };
		const claim = await handoffs.claim(delivery.handoffID, binding.requester.email, () =>
			this.settleHandedBack(request.sessionId, pending, handedBack.optionId)
		);
		if (claim === 'refused') return null;
		if (claim === 'ended') return { handedBackOptionID: handedBack.optionId };
		return { text: `${title}\n\n${handoffs.linkOf(delivery.handoffID)}` };
	}

	private settleHandedBack(sessionID: string, pending: PendingApproval, optionID: string): void {
		if (this.pendingApprovals.get(sessionID) === pending) this.pendingApprovals.delete(sessionID);
		pending.select(optionID);
	}

	private registerPendingApproval(
		request: RequestPermissionRequest,
		binding: SessionBinding,
		delivery: Delivery,
		connectionThatAsked: ClientSideConnection
	): { answer: Promise<RequestPermissionResponse>; withdraw: () => void; pending: PendingApproval } {
		const { promise, resolve } = Promise.withResolvers<RequestPermissionResponse>();
		const pending: PendingApproval = {
			toolCallID: request.toolCall.toolCallId,
			waitingMessageIDs: new Set(delivery.isAlreadyPosted ? [] : this.messageIDsInFlightIn(request.sessionId)),
			select: (optionID) => resolve({ outcome: { outcome: 'selected', optionId: optionID } })
		};
		this.pendingApprovals.set(request.sessionId, pending);
		const withdraw = () => {
			if (this.pendingApprovals.get(request.sessionId) === pending) this.pendingApprovals.delete(request.sessionId);
		};
		connectionThatAsked.signal.addEventListener(
			'abort',
			() => {
				withdraw();
				this.settings.report?.(`approval ${pending.toolCallID} was cancelled: blueclaw left the socket before it was answered`);
				resolve(cancelled);
			},
			{ once: true }
		);
		return { answer: promise, withdraw, pending };
	}

	private messageIDsInFlightIn(sessionID: string): string[] {
		return [...this.sessionOfPromptInFlight].filter(([, inSession]) => inSession === sessionID).map(([messageID]) => messageID);
	}

	private sessionBindingOf(sessionID: string): SessionBinding | undefined {
		return [...this.bindingBySpeaker.values()].find((binding) => binding.sessionID === sessionID);
	}
}

const cancelled: RequestPermissionResponse = { outcome: { outcome: 'cancelled' } };

export function speakerKey(conversationID: string, requesterEmail: string): string {
	return JSON.stringify([conversationID, requesterEmail.trim().toLowerCase()]);
}

export function speakerKeyOf(binding: Pick<SessionBinding, 'requester' | 'addressing'>): string {
	return speakerKey(binding.addressing.conversationID, binding.requester.email);
}

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
	const handoffID = nonEmptyStringIn(carried, 'handoffID');
	return {
		...(deliveryID ? { deliveryID } : {}),
		...(handoffID ? { handoffID } : {}),
		...(replyTargetID ? { replyTargetID } : {}),
		...(Reflect.get(carried, 'isAlreadyPosted') === true ? { isAlreadyPosted: true } : {}),
		...(Reflect.get(carried, 'isFinal') === true ? { isFinal: true } : {})
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
	addressing: Addressing,
	replierEmail: string
): Record<string, unknown> {
	return {
		sessionId: sessionID,
		toolCallId: toolCallID,
		reply,
		messageId: messageID,
		replyTargetId: addressing.replyTargetID,
		isThread: addressing.isThread,
		replierEmail
	};
}

export type ApprovalReplyReading = { optionID: string; isInstruction: boolean };

export function readingOf(answer: Record<string, unknown>): ApprovalReplyReading | undefined {
	if (answer.isAnswer !== true) return undefined;
	const optionID = answer.optionId;
	if (typeof optionID !== 'string') throw new Error(`blueclaw read the reply as an answer to no option: ${JSON.stringify(answer)}`);
	return { optionID, isInstruction: answer.isInstruction === true };
}

function addressedBy(binding: SessionBinding, delivery: Delivery): Addressing {
	if (!delivery.replyTargetID) return binding.addressing;
	return { ...binding.addressing, replyTargetID: delivery.replyTargetID };
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
