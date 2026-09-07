import { gatewayURL, supabase } from '$lib/supabase';
import { supabaseMember } from '$lib/supabase-session';
import { companyEventOf, type CompanyEvent } from '$lib/company-event';

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
const tokenProtocol = 'internkim.bearer.';
const firstRedialMilliseconds = 1_000;
const longestRedialMilliseconds = 30_000;

let joined: Promise<WebSocket> | undefined;
let isServerConnected = false;
let redialsInARow = 0;
const waiting = new Map<string, (answer: HostAnswer) => void>();
const listeners = new Set<(event: CompanyEvent) => void>();

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
	joined = undefined;
	isServerConnected = false;
	if (listeners.size > 0) redialLater();
}

function redialLater(): void {
	redialsInARow += 1;
	const delay = Math.min(firstRedialMilliseconds * 2 ** (redialsInARow - 1), longestRedialMilliseconds);
	setTimeout(() => {
		if (joined || listeners.size === 0) return;
		void companyWire().catch(() => undefined);
	}, delay);
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
	socket.addEventListener('message', (message) => receive(message.data));
	socket.addEventListener('close', dropTheWire);

	await new Promise<void>((resolve, reject) => {
		socket.addEventListener('open', () => resolve(), { once: true });
		socket.addEventListener('error', () => reject(new HostUnreachableError()), { once: true });
	});
	redialsInARow = 0;
	return socket;
}

function receive(data: unknown): void {
	if (typeof data !== 'string') return;
	let payload: Record<string, unknown>;
	try {
		payload = JSON.parse(data) as Record<string, unknown>;
	} catch {
		return;
	}
	if (payload.kind === 'presence') {
		isServerConnected = payload.isServerConnected === true;
		return;
	}
	if (payload.kind === 'deliver') {
		const event = companyEventOf(payload.event);
		if (event) for (const listener of listeners) listener(event);
		return;
	}
	if (payload.kind !== 'result' || typeof payload.requestID !== 'string') return;
	waiting.get(payload.requestID)?.({
		status: typeof payload.status === 'number' ? payload.status : 500,
		body: payload.body
	});
	waiting.delete(payload.requestID);
}

export function onCompanyEvent(listener: (event: CompanyEvent) => void): () => void {
	listeners.add(listener);
	void companyWire().catch(() => undefined);
	return () => {
		listeners.delete(listener);
	};
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
