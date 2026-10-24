import { AskedCalls } from './asked-calls';
import { WorkQueue } from './work-queue';
import { MediaReads, type MediaRequest } from './media-reads';
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
	workerTagOf,
	type HostAttachment,
	type StreamAttachment
} from './socket-attachment';
import {
	binaryStreamFrame,
	bytesOfBinaryFrame,
	hostSupportsHeader,
	isMediaKind,
	messengerMediaStreamCapability,
	parseHostMediaMessage,
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
export const workerBoxHeader = 'x-internkim-worker-box';
export const hostReadsMediaByCallHeader = 'x-internkim-host-reads-media-by-call';
export const workerCallsPerCompany = 64;
export const workerCallBoundMilliseconds = 120_000;
export const workHoldMilliseconds = 3_000;
export const noBoxTookTheWorkStatus = 503;

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
	private readonly workerCalls = new WaitingCalls(workerCallsPerCompany, workerCallBoundMilliseconds);
	private readonly workQueue = new WorkQueue();
	private readonly mediaReads = new MediaReads((document) => this.isSentToHost(document));

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
		if (url.pathname.endsWith('/messenger-media')) return this.takeMediaRead(request);
		if (url.pathname.endsWith('/work')) return this.takeWork(request);
		if (request.headers.get('Upgrade') !== 'websocket') {
			return jsonResponse({ error: 'this endpoint speaks websocket' }, 426);
		}
		if (url.pathname.endsWith('/host-session')) return this.acceptConnectedServer(request);
		if (url.pathname.endsWith('/worker-session')) return this.acceptWorker(request);
		if (url.pathname.endsWith('/stream-session')) return this.acceptStream(request);
		if (url.pathname.endsWith('/server')) return this.acceptServer(request);
		return this.acceptClient(request);
	}

	webSocketMessage(socket: WebSocket, message: string | ArrayBuffer): void {
		const attachment = socketAttachmentOf(socket);
		if (attachment?.role === 'host') return this.onServerMessage(socket, attachment, message);
		if (attachment?.role === 'client') return this.onClientMessage(attachment.memberID, socket, message);
		if (attachment?.role === 'stream') return this.onAppFrame(socket, attachment, message);
		if (attachment?.role === 'worker') return this.onWorkerMessage(attachment.boxKey, message);
	}

	async webSocketClose(socket: WebSocket, code: number, reason: string): Promise<void> {
		this.closeQuietly(socket, code, reason);
		const attachment = socketAttachmentOf(socket);
		if (attachment?.role === 'host') return this.onServerClose(socket, attachment);
		if (attachment?.role === 'stream') this.onAppClose(attachment, code, reason);
		if (attachment?.role === 'worker') this.workQueue.forget(attachment.boxKey);
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
		return this.acceptConnectedServer(request);
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

	private acceptConnectedServer(request: Request): Response {
		this.retireCurrentHost('another server connected');
		const { client, server } = newSocketPair();
		this.state.acceptWebSocket(server, [hostTag]);
		const supports = supportedCapabilitiesOf(request.headers);
		server.serializeAttachment({ role: 'host', seenAt: Date.now(), isRetired: false, supports } satisfies HostAttachment);
		return new Response(null, { status: 101, webSocket: client });
	}

	private async takeMediaRead(request: Request): Promise<Response> {
		if (!this.currentHostSupports(messengerMediaStreamCapability)) {
			return new Response(null, { status: 204, headers: { [hostReadsMediaByCallHeader]: 'true' } });
		}
		const mediaRequest = parseMediaRequest(await readJSONBody(request));
		if (!mediaRequest) return jsonResponse({ error: 'a media read names a method, a path, a host and headers' }, 400);
		if (this.mediaReads.isFull) return jsonResponse({ error: 'too many media reads are open for this company at once' }, 503);
		return this.mediaReads.open(mediaRequest);
	}

	private currentHostSupports(capability: string): boolean {
		const host = this.currentHost();
		const attachment = host ? socketAttachmentOf(host) : null;
		return attachment?.role === 'host' && attachment.supports.includes(capability);
	}

	private isSentToHost(document: string): boolean {
		const host = this.currentHost();
		if (!host) return false;
		try {
			host.send(document);
			return true;
		} catch {
			return false;
		}
	}

	private acceptWorker(request: Request): Response {
		const boxKey = request.headers.get(workerBoxHeader);
		if (!boxKey) return jsonResponse({ error: 'a worker names its box' }, 400);
		for (const earlier of this.state.getWebSockets(workerTagOf(boxKey))) {
			this.closeQuietly(earlier, streamCloseCode.normal, 'the same box connected again');
		}
		const { client, server } = newSocketPair();
		this.state.acceptWebSocket(server, [workerTagOf(boxKey)]);
		server.serializeAttachment({ role: 'worker', boxKey });
		return new Response(null, { status: 101, webSocket: client });
	}

	private async takeWork(request: Request): Promise<Response> {
		const call = parseWork(await readJSONBody(request));
		if (!call) return jsonResponse({ error: 'work names a request identifier, whose it is and what to do' }, 400);
		if (this.workerCalls.isFull) return answerResponse(tooManyWaitingCallsAnswer(call.requestID));
		const waited = this.workerCalls.waitFor(call.requestID);
		const dispatched = this.workQueue.post({ ...call, deadline: Date.now() + workHoldMilliseconds });
		if (dispatched) this.sendWork(dispatched.boxKey, call.requestID, call.work);
		else this.letGoIfNoBoxTakesIt(call.requestID);
		return answerResponse(await waited);
	}

	private letGoIfNoBoxTakesIt(requestID: string): void {
		setTimeout(() => {
			if (!this.workQueue.withdraw(requestID)) return;
			this.workerCalls.settle({ kind: 'result', requestID, status: noBoxTookTheWorkStatus, body: { error: 'no box had room for this work' } });
		}, workHoldMilliseconds);
	}

	private sendWork(boxKey: string, requestID: string, work: unknown): void {
		if (this.isSentTo(boxKey, JSON.stringify({ kind: 'work', requestID, work }))) return;
		this.workerCalls.settle({ kind: 'result', requestID, status: noBoxTookTheWorkStatus, body: { error: 'the box went away before it took the work' } });
	}

	private isSentTo(boxKey: string, document: string): boolean {
		const worker = this.state.getWebSockets(workerTagOf(boxKey))[0];
		if (!worker) return false;
		try {
			worker.send(document);
			return true;
		} catch {
			return false;
		}
	}

	private onWorkerMessage(boxKey: string, message: string | ArrayBuffer): void {
		const payload = readJSON(message);
		const free = freeSlotsOf(payload);
		if (free !== null) {
			for (const dispatched of this.workQueue.setFree(boxKey, free)) {
				this.sendWork(dispatched.boxKey, dispatched.item.requestID, dispatched.item.work);
			}
			return;
		}
		const parsed = parseServerMessage(payload);
		if (parsed?.kind === 'result') this.workerCalls.settle(parsed);
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
		if (isMediaKind(recordKindOf(payload))) return this.onHostMediaMessage(payload);
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

	private onHostMediaMessage(payload: unknown): void {
		const message = parseHostMediaMessage(payload);
		if (message) this.mediaReads.take(message);
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
		this.mediaReads.failEverything('the company computer went away');
		await this.state.storage.put(serverSeenAtStorageKey, this.seenAtOf(socket, attachment));
		this.deliver(JSON.stringify(await this.presence()));
	}

	private retireCurrentHost(reason: string): void {
		const current = this.currentHost();
		if (!current) return;
		const attachment = socketAttachmentOf(current);
		if (attachment?.role === 'host') current.serializeAttachment({ ...attachment, isRetired: true });
		this.closeEveryStream(streamCloseCode.hostGone, reason);
		this.mediaReads.failEverything(reason);
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

type Work = { requestID: string; affinityKey: string; work: unknown };

function parseWork(payload: unknown): Work | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const record: Record<string, unknown> = { ...payload };
	const { requestID, affinityKey, work } = record;
	if (typeof requestID !== 'string' || requestID === '') return null;
	if (typeof affinityKey !== 'string' || affinityKey === '') return null;
	if (typeof work !== 'object' || work === null) return null;
	return { requestID, affinityKey, work };
}

function supportedCapabilitiesOf(headers: Headers): string[] {
	const announced = headers.get(hostSupportsHeader) ?? '';
	return announced.split(',').map((capability) => capability.trim()).filter((capability) => capability !== '');
}

function parseMediaRequest(payload: unknown): MediaRequest | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const record: Record<string, unknown> = { ...payload };
	const { method, path, host, headers } = record;
	if (typeof method !== 'string' || method === '') return null;
	if (typeof path !== 'string' || !path.startsWith('/')) return null;
	if (typeof host !== 'string' || host === '') return null;
	if (typeof headers !== 'object' || headers === null) return null;
	const strings = Object.entries(headers).filter((entry): entry is [string, string] => typeof entry[1] === 'string');
	return { method, path, host, headers: Object.fromEntries(strings) };
}

function freeSlotsOf(payload: unknown): number | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const record: Record<string, unknown> = { ...payload };
	const { kind, free } = record;
	if (kind !== 'ready' || typeof free !== 'number' || !Number.isFinite(free)) return null;
	return free;
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
