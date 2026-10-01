import { WorkerEntrypoint } from 'cloudflare:workers';
import { JSONWebKeyCache, TokenRefused, resolveMember, verifyToken } from './identity';
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
	type OneShotAnswer,
	type OneShotCall,
	type RoutedCall,
	type ServerAnswer
} from './routing';

export type WorkerEnvironment = {
	COMPANY_CONNECTIONS: DurableObjectNamespace;
	SUPABASE_URL: string;
	SUPABASE_PUBLISHABLE_KEY: string;
	SUPABASE_JWKS_URL?: string;
	GATEWAY_ADMIN_TOKEN?: string;
};

const memberHeader = 'x-internkim-member';

export default {
	fetch(request: Request, environment: WorkerEnvironment): Promise<Response> {
		return route(request, environment);
	}
} satisfies ExportedHandler<WorkerEnvironment>;

async function route(request: Request, environment: WorkerEnvironment): Promise<Response> {
	const url = new URL(request.url);
	const path = url.pathname.split('/').filter(Boolean);
	if (path.length !== 3 || path[0] !== 'company') return jsonResponse({ error: 'not found' }, 404);
	const companyID = decodeURIComponent(path[1]);

	if (path[2] === 'client') return joinAsClient(request, environment, companyID);
	if (path[2] === 'host') return joinAsHost(request, environment, companyID);
	if (path[2] === 'server') return joinAsServer(request, environment, companyID);
	if (path[2] === 'server-key') return storeServerKey(request, environment, companyID);
	if (path[2] === 'call') return takeCompanyCall(request, environment, companyID);
	return jsonResponse({ error: 'not found' }, 404);
}

async function joinAsClient(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	const token = bearerOf(request) ?? tokenOfferedByBrowser(request);
	if (!token) return jsonResponse({ error: 'this call carried no token' }, 401);
	try {
		const claims = await verifyToken(
			token,
			keyCacheFor(environment),
			issuerOf(environment),
			Math.floor(Date.now() / 1000)
		);
		const identity = await resolveMember(
			environment.SUPABASE_URL,
			environment.SUPABASE_PUBLISHABLE_KEY,
			token,
			claims.sub
		);
		if (identity.companyID !== companyID) {
			return jsonResponse({ error: 'this account belongs to another company' }, 403);
		}
		return connectionFor(environment, companyID).fetch(
			new Request(request.url, { headers: { ...headersOf(request), [memberHeader]: identity.memberID } })
		);
	} catch (refusal) {
		if (refusal instanceof TokenRefused) return jsonResponse({ error: refusal.message }, 401);
		throw refusal;
	}
}

function joinAsServer(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	return connectionFor(environment, companyID).fetch(request);
}

async function joinAsHost(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	const token = bearerOf(request);
	if (!token) return jsonResponse({ error: 'this call carried no token' }, 401);
	try {
		const claims = await verifyToken(
			token,
			keyCacheFor(environment),
			issuerOf(environment),
			Math.floor(Date.now() / 1000)
		);
		if (claims.hostCompanyID !== companyID) {
			return jsonResponse({ error: 'this token is not the company host' }, 403);
		}
		return connectionFor(environment, companyID).fetch(
			new Request(`https://connection-gateway/company/${encodeURIComponent(companyID)}/host-session`, {
				method: request.method,
				headers: headersOf(request)
			})
		);
	} catch (refusal) {
		if (refusal instanceof TokenRefused) return jsonResponse({ error: refusal.message }, 401);
		throw refusal;
	}
}

function takeCompanyCall(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> | Response {
	if (!environment.GATEWAY_ADMIN_TOKEN || bearerOf(request) !== environment.GATEWAY_ADMIN_TOKEN) {
		return jsonResponse({ error: 'this call may not speak to a company' }, 401);
	}
	return connectionFor(environment, companyID).fetch(request);
}

async function storeServerKey(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	if (!environment.GATEWAY_ADMIN_TOKEN || bearerOf(request) !== environment.GATEWAY_ADMIN_TOKEN) {
		return jsonResponse({ error: 'this call may not set a server key' }, 401);
	}
	return connectionFor(environment, companyID).fetch(request);
}

function issuerOf(environment: WorkerEnvironment): string {
	return `${environment.SUPABASE_URL.replace(/\/+$/, '')}/auth/v1`;
}

let sharedKeyCache: JSONWebKeyCache | undefined;

function keyCacheFor(environment: WorkerEnvironment): JSONWebKeyCache {
	sharedKeyCache ??= new JSONWebKeyCache(
		environment.SUPABASE_JWKS_URL ?? `${issuerOf(environment)}/.well-known/jwks.json`
	);
	return sharedKeyCache;
}

function connectionFor(environment: WorkerEnvironment, companyID: string): DurableObjectStub {
	const namespace = environment.COMPANY_CONNECTIONS;
	return namespace.get(namespace.idFromName(companyID));
}

function bearerOf(request: Request): string | null {
	const offered = request.headers.get('Authorization') ?? '';
	return offered.startsWith('Bearer ') ? offered.slice('Bearer '.length).trim() || null : null;
}

const browserTokenProtocol = 'internkim.bearer.';

// A browser cannot put a header on a websocket handshake, so it carries the
// token as a subprotocol instead. The chosen protocol has to be echoed back or
// the browser closes the connection it just opened.
function tokenOfferedByBrowser(request: Request): string | null {
	const offered = request.headers.get('Sec-WebSocket-Protocol') ?? '';
	for (const protocol of offered.split(',')) {
		const trimmed = protocol.trim();
		if (trimmed.startsWith(browserTokenProtocol)) return trimmed.slice(browserTokenProtocol.length) || null;
	}
	return null;
}

function headersOf(request: Request): Record<string, string> {
	const headers: Record<string, string> = {};
	request.headers.forEach((value, name) => {
		headers[name] = value;
	});
	return headers;
}

function jsonResponse(document: unknown, status: number): Response {
	return new Response(JSON.stringify(document), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

// A Cloudflare WorkerEntrypoint is reachable only through a service binding.
export class CompanyCalls extends WorkerEntrypoint<WorkerEnvironment> {
	async callCompany(companyID: string, call: OneShotCall): Promise<OneShotAnswer> {
		const answered = await connectionFor(this.env, companyID).fetch(
			new Request(`https://connection-gateway/company/${encodeURIComponent(companyID)}/call`, {
				method: 'POST',
				body: JSON.stringify(call)
			})
		);
		if (!answered.ok) throw new TypeError('a company call names a requestID and a capability');
		return (await answered.json()) as OneShotAnswer;
	}
}

const bearerPrefix = 'Bearer ';
const serverKeyDigestStorageKey = 'serverKeyDigest';
const legacyServerKeyStorageKey = 'serverKey';

async function digestOf(value: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value));
	return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
}

function digestsMatch(first: string, second: string): boolean {
	const encoder = new TextEncoder();
	const firstBytes = encoder.encode(first);
	const secondBytes = encoder.encode(second);
	if (firstBytes.byteLength !== secondBytes.byteLength) return false;
	return crypto.subtle.timingSafeEqual(firstBytes, secondBytes);
}

export class CompanyConnectionObject {
	private serverSocket: WebSocket | null = null;
	private serverSeenAt = 0;
	private readonly clientSockets = new Map<string, Set<WebSocket>>();
	private readonly ledger = new CallLedger();
	private readonly askedBy = new Map<string, string>();
	private readonly waitingCalls = new WaitingCalls();

	constructor(private readonly state: DurableObjectState) {}

	async fetch(request: Request): Promise<Response> {
		const url = new URL(request.url);
		if (url.pathname.endsWith('/server-key')) return this.keepServerKey(request);
		if (url.pathname.endsWith('/call')) return this.takeOneShotCall(request);
		if (url.pathname.endsWith('/host-session')) return this.acceptHost(request);
		if (request.headers.get('Upgrade') !== 'websocket') {
			return jsonResponse({ error: 'this endpoint speaks websocket' }, 426);
		}
		if (url.pathname.endsWith('/server')) return this.acceptServer(request);
		return this.acceptClient(request);
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

	private async takeOneShotCall(request: Request): Promise<Response> {
		const call = parseOneShotCall(await readJSONBody(request));
		if (!call) return jsonResponse({ error: 'a request identifier and a capability are required' }, 400);

		const decision = decideCall(call, undefined, this.ledger, this.serverSocket !== null);
		if (decision.action === 'answer') return answerResponse(decision.answer);
		if (decision.action === 'ignore') return answerResponse(callInFlightAnswer(call.requestID));
		if (this.waitingCalls.isFull) return answerResponse(tooManyWaitingCallsAnswer(call.requestID));

		const waited = this.waitingCalls.waitFor(call.requestID);
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

	private acceptHost(request: Request): Response {
		if (request.headers.get('Upgrade') !== 'websocket') {
			return jsonResponse({ error: 'this endpoint speaks websocket' }, 426);
		}
		return this.acceptConnectedServer(request);
	}

	private acceptConnectedServer(request: Request): Response {
		const { client, server } = newSocketPair();
		server.accept();
		this.serverSocket?.close(1012, 'another server connected');
		this.serverSocket = server;
		this.serverSeenAt = Date.now();
		server.addEventListener('message', (message) => this.onServerMessage(message));
		server.addEventListener('close', () => this.onServerClose(server));
		return new Response(null, { status: 101, webSocket: client });
	}

	private acceptClient(request: Request): Response {
		const memberID = request.headers.get(memberHeader);
		if (!memberID) return jsonResponse({ error: 'this connection names no member' }, 401);
		const { client, server } = newSocketPair();
		server.accept();
		this.socketsOf(memberID).add(server);
		server.addEventListener('message', (message) => this.onClientMessage(memberID, server, message));
		server.addEventListener('close', () => this.forgetClient(memberID, server));
		server.send(JSON.stringify(this.presence()));
		const spoken = request.headers.get('Sec-WebSocket-Protocol')?.split(',')[0]?.trim();
		return new Response(null, {
			status: 101,
			webSocket: client,
			headers: spoken ? { 'Sec-WebSocket-Protocol': spoken } : undefined
		});
	}

	private onClientMessage(memberID: string, socket: WebSocket, message: MessageEvent): void {
		const call = parseClientCall(readJSON(message.data));
		if (!call) return;
		const decision = decideCall(call, memberID, this.ledger, this.serverSocket !== null);
		if (decision.action === 'ignore') return;
		if (decision.action === 'answer') {
			socket.send(JSON.stringify(decision.answer));
			return;
		}
		this.forward(decision.routed, memberID);
	}

	private forward(routed: RoutedCall, memberID?: string): void {
		this.ledger.markPending(routed.requestID);
		if (memberID) this.askedBy.set(routed.requestID, memberID);
		try {
			this.serverSocket?.send(JSON.stringify(routed));
		} catch {
			this.ledger.forgetPending(routed.requestID);
			this.askedBy.delete(routed.requestID);
		}
	}

	private onServerMessage(message: MessageEvent): void {
		const parsed = parseServerMessage(readJSON(message.data));
		if (!parsed) return;
		this.serverSeenAt = Date.now();
		if (parsed.kind === 'result') {
			this.answer(parsed);
			return;
		}
		this.deliver(JSON.stringify({ kind: 'deliver', event: parsed.event }), parsed.audienceMemberIDs);

	}

	private answer(answer: ServerAnswer): void {
		this.ledger.recordAnswer(answer);
		if (this.waitingCalls.settle(answer)) return;
		const memberID = this.askedBy.get(answer.requestID);
		this.askedBy.delete(answer.requestID);
		if (!memberID) return;
		this.sendTo(memberID, JSON.stringify(answer));
	}

	private deliver(document: string, audienceMemberIDs?: string[]): void {
		const audience = audienceMemberIDs ?? [...this.clientSockets.keys()];
		for (const memberID of audience) this.sendTo(memberID, document);
	}

	private sendTo(memberID: string, document: string): void {
		for (const socket of this.socketsOf(memberID)) {
			try {
				socket.send(document);
			} catch {
				this.forgetClient(memberID, socket);
			}
		}
	}

	private onServerClose(socket: WebSocket): void {
		if (this.serverSocket !== socket) return;
		this.serverSocket = null;
		this.deliver(JSON.stringify(this.presence()));
	}

	private presence(): { kind: 'presence'; isServerConnected: boolean; serverSeenAt: number } {
		return { kind: 'presence', isServerConnected: this.serverSocket !== null, serverSeenAt: this.serverSeenAt };
	}

	private socketsOf(memberID: string): Set<WebSocket> {
		let sockets = this.clientSockets.get(memberID);
		if (!sockets) {
			sockets = new Set<WebSocket>();
			this.clientSockets.set(memberID, sockets);
		}
		return sockets;
	}

	private forgetClient(memberID: string, socket: WebSocket): void {
		const sockets = this.clientSockets.get(memberID);
		if (!sockets) return;
		sockets.delete(socket);
		if (sockets.size === 0) this.clientSockets.delete(memberID);
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

function readJSON(data: unknown): unknown {
	if (typeof data !== 'string') return null;
	try {
		return JSON.parse(data);
	} catch {
		return null;
	}
}
