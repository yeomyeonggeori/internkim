import { AskedCalls } from './asked-calls';
import {
	CallLedger,
	WaitingCalls,
	callInFlightAnswer,
	callTimedOutStatus,
	decideCall,
	oneShotAnswerOf,
	parseClientCall,
	parseOneShotCall,
	parseServerMessage,
	tooManyWaitingCallsAnswer,
	type RoutedCall,
	type ServerAnswer
} from './routing';
import {
	clientTag,
	hostTag,
	memberTagOf,
	socketAttachmentOf,
	streamTag,
	streamTagOf,
	type HostAttachment,
	type StreamAttachment
} from './socket-attachment';
import {
	binaryStreamFrame,
	bytesOfBinaryFrame,
	isBinaryFrameTooBig,
	isFrameTooBig,
	isStreamKind,
	parseStreamMessage,
	pingFrame,
	pongFrame,
	sendableClose,
	streamClose,
	streamCloseCode,
	streamFrame,
	streamOpen,
	type StreamMessage
} from './host-protocol';

export const memberHeader = 'x-internkim-member';
export const streamHostHeader = 'x-internkim-stream-host';
export const streamAddressHeader = 'x-internkim-stream-address';
export const streamPathHeader = 'x-internkim-stream-path';

const bearerPrefix = 'Bearer ';
const serverKeyDigestStorageKey = 'serverKeyDigest';
const legacyServerKeyStorageKey = 'serverKey';
const serverSeenAtStorageKey = 'serverSeenAt';
const seenAtResolutionMilliseconds = 1_000;
export const messengerCallsPerCompany = 256;
export const messengerCallBoundMilliseconds = 60_000;

export async function digestOf(value: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value));
	return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
}

export function digestsMatch(first: string, second: string): boolean {
	const encoder = new TextEncoder();
	const firstBytes = encoder.encode(first);
	const secondBytes = encoder.encode(second);
	if (firstBytes.byteLength !== secondBytes.byteLength) return false;
	return crypto.subtle.timingSafeEqual(firstBytes, secondBytes);
}

export function jsonResponse(document: unknown, status: number): Response {
	return new Response(JSON.stringify(document), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

type Presence = { kind: 'presence'; isServerConnected: boolean; serverSeenAt: number };

export class CompanyConnectionObject {
	private readonly ledger = new CallLedger();
	private readonly askedCalls: AskedCalls;
	private readonly waitingCalls = new WaitingCalls();
	private readonly messengerCalls = new WaitingCalls(messengerCallsPerCompany, messengerCallBoundMilliseconds);

	constructor(private readonly state: DurableObjectState) {
		this.askedCalls = new AskedCalls(state.storage.sql);
		for (const requestID of this.askedCalls.requestIDs()) this.ledger.markPending(requestID);
		state.setWebSocketAutoResponse(new WebSocketRequestResponsePair(pingFrame, pongFrame));
	}

	async fetch(request: Request): Promise<Response> {
		const url = new URL(request.url);
		if (url.pathname.endsWith('/server-key')) return this.keepServerKey(request);
		if (url.pathname.endsWith('/call')) return this.takeOneShotCall(request, this.waitingCalls);
		if (url.pathname.endsWith('/messenger-call')) return this.takeOneShotCall(request, this.messengerCalls);
		if (request.headers.get('Upgrade') !== 'websocket') {
			return jsonResponse({ error: 'this endpoint speaks websocket' }, 426);
		}
		if (url.pathname.endsWith('/host-session')) return this.acceptConnectedServer();
		if (url.pathname.endsWith('/stream-session')) return this.acceptStream(request);
		if (url.pathname.endsWith('/server')) return this.acceptServer(request);
		return this.acceptClient(request);
	}

	webSocketMessage(socket: WebSocket, message: string | ArrayBuffer): void {
		const attachment = socketAttachmentOf(socket);
		if (attachment?.role === 'host') return this.onServerMessage(socket, attachment, message);
		if (attachment?.role === 'client') return this.onClientMessage(attachment.memberID, socket, message);
		if (attachment?.role === 'stream') return this.onAppFrame(socket, attachment, message);
	}

	async webSocketClose(socket: WebSocket, code: number, reason: string): Promise<void> {
		this.closeQuietly(socket, code, reason);
		const attachment = socketAttachmentOf(socket);
		if (attachment?.role === 'host') return this.onServerClose(socket, attachment);
		if (attachment?.role === 'stream') this.onAppClose(attachment, code, reason);
	}

	webSocketError(socket: WebSocket): Promise<void> {
		return this.webSocketClose(socket, streamCloseCode.relayFailed, 'the connection failed');
	}

	private async keepServerKey(request: Request): Promise<Response> {
		const { serverKey } = (await request.json()) as { serverKey?: string };
		if (typeof serverKey !== 'string' || serverKey.trim() === '') {
			return jsonResponse({ error: 'a server key is required' }, 400);
		}
		await this.state.storage.put(serverKeyDigestStorageKey, await digestOf(serverKey));
		await this.state.storage.delete(legacyServerKeyStorageKey);
		return jsonResponse({ status: 'ok' }, 200);
	}

	private async takeOneShotCall(request: Request, waitingCalls: WaitingCalls): Promise<Response> {
		const call = parseOneShotCall(await readJSONBody(request));
		if (!call) return jsonResponse({ error: 'a request identifier and a capability are required' }, 400);

		const decision = decideCall(call, undefined, this.ledger, this.currentHost() !== null);
		if (decision.action === 'answer') return answerResponse(decision.answer);
		if (decision.action === 'ignore') return answerResponse(callInFlightAnswer(call.requestID));
		if (waitingCalls.isFull) return answerResponse(tooManyWaitingCallsAnswer(call.requestID));

		const waited = waitingCalls.waitFor(call.requestID);
		this.forward(decision.routed);
		const answer = await waited;
		if (answer.status === callTimedOutStatus) this.ledger.forgetPending(call.requestID);
		return answerResponse(answer);
	}

	private async acceptServer(request: Request): Promise<Response> {
		const offered = request.headers.get('Authorization') ?? '';
		if (!(await this.isTheServerKey(offered))) {
			return jsonResponse({ error: 'this connection is not the company server' }, 401);
		}
		return this.acceptConnectedServer();
	}

	private async isTheServerKey(authorization: string): Promise<boolean> {
		if (!authorization.startsWith(bearerPrefix)) return false;
		const offeredDigest = await digestOf(authorization.slice(bearerPrefix.length));
		const keptDigest = await this.state.storage.get<string>(serverKeyDigestStorageKey);
		if (keptDigest) return digestsMatch(offeredDigest, keptDigest);
		return this.upgradeLegacyServerKey(offeredDigest);
	}

	private async upgradeLegacyServerKey(offeredDigest: string): Promise<boolean> {
		const legacyKey = await this.state.storage.get<string>(legacyServerKeyStorageKey);
		if (!legacyKey) return false;
		if (!digestsMatch(offeredDigest, await digestOf(legacyKey))) return false;
		await this.state.storage.put(serverKeyDigestStorageKey, offeredDigest);
		await this.state.storage.delete(legacyServerKeyStorageKey);
		return true;
	}

	private acceptConnectedServer(): Response {
		this.retireCurrentHost('another server connected');
		const { client, server } = newSocketPair();
		this.state.acceptWebSocket(server, [hostTag]);
		server.serializeAttachment({ role: 'host', seenAt: Date.now(), isRetired: false } satisfies HostAttachment);
		return new Response(null, { status: 101, webSocket: client });
	}

	private async acceptClient(request: Request): Promise<Response> {
		const memberID = request.headers.get(memberHeader);
		if (!memberID) return jsonResponse({ error: 'this connection names no member' }, 401);
		const { client, server } = newSocketPair();
		this.state.acceptWebSocket(server, [clientTag, memberTagOf(memberID)]);
		server.serializeAttachment({ role: 'client', memberID });
		server.send(JSON.stringify(await this.presence()));
		const spoken = request.headers.get('Sec-WebSocket-Protocol')?.split(',')[0]?.trim();
		return new Response(null, {
			status: 101,
			webSocket: client,
			headers: spoken ? { 'Sec-WebSocket-Protocol': spoken } : undefined
		});
	}

	private acceptStream(request: Request): Response {
		const host = this.currentHost();
		if (!host) return jsonResponse({ error: 'the company computer is not connected' }, 503);
		const publicHost = request.headers.get(streamHostHeader) ?? '';
		if (!publicHost) return jsonResponse({ error: 'a stream names the host it was dialled on' }, 400);
		const streamID = crypto.randomUUID();
		const { client, server } = newSocketPair();
		this.state.acceptWebSocket(server, [streamTag, streamTagOf(streamID)]);
		server.serializeAttachment({ role: 'stream', streamID, isClosing: false } satisfies StreamAttachment);
		const forwardedFor = request.headers.get(streamAddressHeader) ?? undefined;
		const path = request.headers.get(streamPathHeader) ?? '/';
		host.send(streamOpen({ streamID, host: publicHost, path, forwardedFor }));
		return new Response(null, { status: 101, webSocket: client });
	}

	private onClientMessage(memberID: string, socket: WebSocket, message: string | ArrayBuffer): void {
		const call = parseClientCall(readJSON(message));
		if (!call) return;
		const decision = decideCall(call, memberID, this.ledger, this.currentHost() !== null);
		if (decision.action === 'ignore') return;
		if (decision.action === 'answer') {
			socket.send(JSON.stringify(decision.answer));
			return;
		}
		this.forward(decision.routed, memberID);
	}

	private forward(routed: RoutedCall, memberID?: string): void {
		this.ledger.markPending(routed.requestID);
		if (memberID) this.askedCalls.remember(routed.requestID, memberID);
		try {
			this.currentHost()?.send(JSON.stringify(routed));
		} catch {
			this.ledger.forgetPending(routed.requestID);
			this.askedCalls.forget(routed.requestID);
		}
	}

	private onServerMessage(socket: WebSocket, attachment: HostAttachment, message: string | ArrayBuffer): void {
		if (attachment.isRetired) return;
		this.noteServerSeen(socket, attachment);
		const payload = readJSON(message);
		if (isStreamKind(recordKindOf(payload))) {
			const streamMessage = parseStreamMessage(payload);
			if (streamMessage) this.onHostStreamMessage(socket, streamMessage);
			return;
		}
		const parsed = parseServerMessage(payload);
		if (!parsed) return;
		if (parsed.kind === 'result') {
			this.answer(parsed);
			return;
		}
		this.deliver(JSON.stringify({ kind: 'deliver', event: parsed.event }), parsed.audienceMemberIDs);
	}

	private onHostStreamMessage(host: WebSocket, message: StreamMessage): void {
		const app = this.streamSocket(message.streamID);
		if (!app) {
			if (message.kind !== 'stream.close') host.send(streamClose(message.streamID, streamCloseCode.normal, 'the app has gone'));
			return;
		}
		if (message.kind === 'stream.frame') {
			this.sendToApp(app, message.isBinary ? bytesOfBinaryFrame(message) : message.data);
			return;
		}
		if (message.kind === 'stream.close') this.closeApp(app, message.code, message.reason);
	}

	private sendToApp(app: WebSocket, data: string | Uint8Array<ArrayBuffer>): void {
		try {
			app.send(data);
		} catch {
			this.closeApp(app, streamCloseCode.relayFailed, 'the app connection failed');
		}
	}

	private onAppFrame(app: WebSocket, attachment: StreamAttachment, message: string | ArrayBuffer): void {
		if (attachment.isClosing) return;
		if (typeof message === 'string' ? isFrameTooBig(message) : isBinaryFrameTooBig(message)) {
			this.refuseAppFrame(app, attachment, streamCloseCode.frameTooBig, 'a frame may carry at most 512 KiB');
			return;
		}
		const host = this.currentHost();
		if (!host) {
			this.closeApp(app, streamCloseCode.hostGone, 'the company computer went away');
			return;
		}
		const streamID = attachment.streamID;
		host.send(typeof message === 'string' ? streamFrame(streamID, message) : binaryStreamFrame(streamID, new Uint8Array(message)));
	}

	private refuseAppFrame(app: WebSocket, attachment: StreamAttachment, code: number, reason: string): void {
		this.closeApp(app, code, reason);
		this.currentHost()?.send(streamClose(attachment.streamID, code, reason));
	}

	private onAppClose(attachment: StreamAttachment, code: number, reason: string): void {
		if (attachment.isClosing) return;
		this.currentHost()?.send(streamClose(attachment.streamID, code, reason));
	}

	private closeApp(app: WebSocket, code: number, reason: string): void {
		const attachment = socketAttachmentOf(app);
		if (attachment?.role === 'stream') app.serializeAttachment({ ...attachment, isClosing: true });
		this.closeQuietly(app, code, reason);
	}

	private closeEveryStream(code: number, reason: string): void {
		for (const app of this.state.getWebSockets(streamTag)) this.closeApp(app, code, reason);
	}

	private streamSocket(streamID: string): WebSocket | null {
		return this.state.getWebSockets(streamTagOf(streamID))[0] ?? null;
	}

	private answer(answer: ServerAnswer): void {
		if (this.messengerCalls.settle(answer)) {
			this.ledger.forgetPending(answer.requestID);
			return;
		}
		this.ledger.recordAnswer(answer);
		if (this.waitingCalls.settle(answer)) return;
		const memberID = this.askedCalls.take(answer.requestID);
		if (!memberID) return;
		this.sendTo(memberID, JSON.stringify(answer));
	}

	private deliver(document: string, audienceMemberIDs?: string[]): void {
		if (!audienceMemberIDs) {
			for (const socket of this.state.getWebSockets(clientTag)) sendIgnoringFailure(socket, document);
			return;
		}
		for (const memberID of audienceMemberIDs) this.sendTo(memberID, document);
	}

	private sendTo(memberID: string, document: string): void {
		for (const socket of this.state.getWebSockets(memberTagOf(memberID))) sendIgnoringFailure(socket, document);
	}

	private async onServerClose(socket: WebSocket, attachment: HostAttachment): Promise<void> {
		if (attachment.isRetired) return;
		socket.serializeAttachment({ ...attachment, isRetired: true });
		this.closeEveryStream(streamCloseCode.hostGone, 'the company computer went away');
		await this.state.storage.put(serverSeenAtStorageKey, this.seenAtOf(socket, attachment));
		this.deliver(JSON.stringify(await this.presence()));
	}

	private retireCurrentHost(reason: string): void {
		const current = this.currentHost();
		if (!current) return;
		const attachment = socketAttachmentOf(current);
		if (attachment?.role === 'host') current.serializeAttachment({ ...attachment, isRetired: true });
		this.closeEveryStream(streamCloseCode.hostGone, reason);
		this.closeQuietly(current, streamCloseCode.hostGone, reason);
	}

	private currentHost(): WebSocket | null {
		for (const socket of this.state.getWebSockets(hostTag)) {
			const attachment = socketAttachmentOf(socket);
			if (attachment?.role === 'host' && !attachment.isRetired) return socket;
		}
		return null;
	}

	private noteServerSeen(socket: WebSocket, attachment: HostAttachment): void {
		const now = Date.now();
		if (now - attachment.seenAt < seenAtResolutionMilliseconds) return;
		socket.serializeAttachment({ ...attachment, seenAt: now });
	}

	private seenAtOf(socket: WebSocket, attachment: HostAttachment): number {
		const ponged = this.state.getWebSocketAutoResponseTimestamp(socket)?.getTime() ?? 0;
		return Math.max(attachment.seenAt, ponged);
	}

	private async presence(): Promise<Presence> {
		const host = this.currentHost();
		const attachment = host ? socketAttachmentOf(host) : null;
		if (host && attachment?.role === 'host') {
			return { kind: 'presence', isServerConnected: true, serverSeenAt: this.seenAtOf(host, attachment) };
		}
		const lastSeen = (await this.state.storage.get<number>(serverSeenAtStorageKey)) ?? 0;
		return { kind: 'presence', isServerConnected: false, serverSeenAt: lastSeen };
	}

	private closeQuietly(socket: WebSocket, code: number, reason: string): void {
		const sendable = sendableClose(code, reason);
		try {
			socket.close(sendable.code, sendable.reason);
		} catch {
			return;
		}
	}
}

function sendIgnoringFailure(socket: WebSocket, document: string): void {
	try {
		socket.send(document);
	} catch {
		return;
	}
}

function newSocketPair(): { client: WebSocket; server: WebSocket } {
	const pair = new WebSocketPair();
	return { client: pair[0], server: pair[1] };
}

async function readJSONBody(request: Request): Promise<unknown> {
	try {
		return await request.json();
	} catch {
		return null;
	}
}

function answerResponse(answer: ServerAnswer): Response {
	return jsonResponse(oneShotAnswerOf(answer), 200);
}

function recordKindOf(payload: unknown): unknown {
	if (typeof payload !== 'object' || payload === null) return undefined;
	return 'kind' in payload ? payload.kind : undefined;
}

function readJSON(data: unknown): unknown {
	if (typeof data !== 'string') return null;
	try {
		return JSON.parse(data);
	} catch {
		return null;
	}
}
