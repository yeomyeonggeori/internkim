import { gatewayURL, supabase } from '$lib/supabase';
import { supabaseMember } from '$lib/supabase-session';

export type HostCall = {
	capability: string;
	body?: Record<string, unknown>;
};

export type HostAnswer = {
	status: number;
	body: unknown;
};

export class HostUnreachableError extends Error {
	constructor() {
		super('the company app is not running');
		this.name = 'HostUnreachableError';
	}
}

const answerTimeoutMilliseconds = 20_000;
const presenceTimeoutMilliseconds = 5_000;
const tokenProtocol = 'internkim.bearer.';

let joined: Promise<WebSocket> | undefined;
let activeSocket: WebSocket | undefined;
let isServerConnected = false;
const waiting = new Map<string, (answer: HostAnswer) => void>();

async function companyWire(): Promise<WebSocket> {
	if (joined) return joined;
	const attempt = openWire();
	joined = attempt;
	attempt.catch(() => forgetWire(attempt));
	return attempt;
}

function forgetWire(attempt: Promise<WebSocket>): void {
	if (joined !== attempt) return;
	dropTheWire();
}

function dropTheWire(): void {
	activeSocket = undefined;
	joined = undefined;
	isServerConnected = false;
}

function dropSocket(socket: WebSocket): void {
	if (activeSocket !== socket) return;
	dropTheWire();
}

async function openWire(): Promise<WebSocket> {
	const address = gatewayURL();
	if (!address) throw new HostUnreachableError();

	const client = supabase();
	const { data } = await client.auth.getSession();
	const token = data.session?.access_token;
	if (!token) throw new Error('sign in first');

	const { companyID } = await supabaseMember();
	if (!companyID) throw new Error('sign in first');

	const url = `${address.replace(/\/+$/, '')}/company/${encodeURIComponent(companyID)}/client`;
	const socket = new WebSocket(url, [tokenProtocol + token]);
	activeSocket = socket;
	let hasOpened = false;
	let hasPresence = false;
	let settleReadiness: ((error?: HostUnreachableError) => void) | undefined;
	const readiness = new Promise<void>((resolve, reject) => {
		settleReadiness = (error) => (error ? reject(error) : resolve());
	});
	const finishReadiness = (error?: HostUnreachableError): void => {
		if (!settleReadiness) return;
		const settle = settleReadiness;
		settleReadiness = undefined;
		settle(error);
	};
	const readinessTimer = setTimeout(() => {
		finishReadiness(new HostUnreachableError());
		socket.close();
	}, presenceTimeoutMilliseconds);
	socket.addEventListener('message', (message) => {
		const payload = payloadOf(message.data);
		if (!payload) return;
		receivePayload(payload);
		if (isPresence(payload)) {
			hasPresence = true;
			if (hasOpened) finishReadiness();
		}
	});
	socket.addEventListener('close', () => {
		dropSocket(socket);
		finishReadiness(new HostUnreachableError());
	});

	await new Promise<void>((resolve, reject) => {
		readiness.then(resolve, reject).finally(() => clearTimeout(readinessTimer));
		socket.addEventListener('open', () => {
			hasOpened = true;
			if (hasPresence) finishReadiness();
		}, { once: true });
		socket.addEventListener('error', () => finishReadiness(new HostUnreachableError()), { once: true });
	});
	return socket;
}

function payloadOf(data: unknown): Record<string, unknown> | null {
	if (typeof data !== 'string') return null;
	let payload: unknown;
	try {
		payload = JSON.parse(data);
	} catch {
		return null;
	}
	return typeof payload === 'object' && payload !== null ? payload as Record<string, unknown> : null;
}

function receivePayload(payload: Record<string, unknown>): void {
	if (payload.kind === 'presence') {
		isServerConnected = payload.isServerConnected === true;
		return;
	}
	if (payload.kind !== 'result' || typeof payload.requestID !== 'string') return;
	waiting.get(payload.requestID)?.({
		status: typeof payload.status === 'number' ? payload.status : 500,
		body: payload.body
	});
	waiting.delete(payload.requestID);
}

function isPresence(payload: Record<string, unknown>): boolean {
	return payload.kind === 'presence' && typeof payload.isServerConnected === 'boolean';
}

export async function isCompanyAppRunning(): Promise<boolean> {
	await companyWire();
	return isServerConnected;
}

export async function callCompanyApp(call: HostCall): Promise<HostAnswer> {
	const wire = await companyWire();
	const requestID = crypto.randomUUID();

	const answered = new Promise<HostAnswer | null>((resolve) => {
		waiting.set(requestID, resolve);
		setTimeout(() => {
			if (!waiting.delete(requestID)) return;
			resolve(null);
		}, answerTimeoutMilliseconds);
	});

	wire.send(
		JSON.stringify({ kind: 'call', requestID, capability: call.capability, body: call.body ?? {} })
	);

	const answer = await answered;
	if (!answer) throw new HostUnreachableError();
	return answer;
}
