import { gatewayURL, supabase } from '$lib/supabase';
import { supabaseMember } from '$lib/supabase-session';
import { companyEventOf, type CompanyEvent } from '$lib/company-event';
import { HostUnreachableError, readyWithPresence, type Frame } from '$lib/host-presence';

export { HostUnreachableError };

export type HostCall = {
	capability: string;
	body?: Record<string, unknown>;
};

export type HostAnswer = {
	status: number;
	body: unknown;
};

const answerTimeoutMilliseconds = 20_000;
const tokenProtocol = 'internkim.bearer.';
const firstRedialMilliseconds = 1_000;
const longestRedialMilliseconds = 30_000;

let joined: Promise<WebSocket> | undefined;
let activeSocket: WebSocket | undefined;
export type CompanyAppOwner = { accountID: string; companyID: string; accessToken: string };
let wireOwner: CompanyAppOwner | undefined;
let followsAuth = false;
let isServerConnected = false;
let redialsInARow = 0;
const waiting = new Map<string, (answer: HostAnswer | null) => void>();
const listeners = new Set<(event: CompanyEvent) => void>();

function isAnyoneListening(): boolean {
	return listeners.size > 0;
}

async function companyWire(expectedOwner?: CompanyAppOwner): Promise<WebSocket> {
	followAuthChanges();
	const client = supabase();
	const { data } = await client.auth.getSession();
	const session = data.session;
	if (!session?.access_token || !session.user.id) throw new Error('sign in first');
	const { companyID } = await supabaseMember();
	if (!companyID) throw new Error('sign in first');
	const { data: latest } = await client.auth.getSession();
	if (latest.session?.user.id !== session.user.id || latest.session.access_token !== session.access_token) {
		throw new HostUnreachableError();
	}
	const owner = { accountID: session.user.id, companyID, accessToken: session.access_token };
	if (expectedOwner && !sameOwner(owner, expectedOwner)) throw new HostUnreachableError();
	if (wireOwner && !sameOwner(wireOwner, owner)) {
		dropTheWire();
	}
	if (joined) return joined;
	wireOwner = owner;
	const attempt = openWire(owner);
	joined = attempt;
	attempt.catch(() => forgetWire(attempt));
	return attempt;
}

function sameOwner(first: CompanyAppOwner, second: CompanyAppOwner): boolean {
	return first.accountID === second.accountID && first.companyID === second.companyID && first.accessToken === second.accessToken;
}

function followAuthChanges(): void {
	if (followsAuth) return;
	followsAuth = true;
	supabase().auth.onAuthStateChange((_event, session) => {
		if (!wireOwner) return;
		if (session?.user.id !== wireOwner.accountID || session.access_token !== wireOwner.accessToken) dropTheWire();
	});
}

function forgetWire(attempt: Promise<WebSocket>): void {
	if (joined !== attempt) return;
	dropTheWire();
}

function dropTheWire(): void {
	const socket = activeSocket;
	activeSocket = undefined;
	joined = undefined;
	wireOwner = undefined;
	isServerConnected = false;
	for (const settle of waiting.values()) settle(null);
	waiting.clear();
	socket?.close();
	if (isAnyoneListening()) redialLater();
}

function dropSocket(socket: WebSocket): void {
	if (activeSocket !== socket) return;
	dropTheWire();
}

function redialLater(): void {
	redialsInARow += 1;
	const delay = Math.min(firstRedialMilliseconds * 2 ** (redialsInARow - 1), longestRedialMilliseconds);
	setTimeout(() => {
		if (joined || !isAnyoneListening()) return;
		void companyWire().catch(() => undefined);
	}, delay);
}

async function openWire(owner: CompanyAppOwner): Promise<WebSocket> {
	const address = gatewayURL();
	if (!address) throw new HostUnreachableError();

	const url = `${address.replace(/\/+$/, '')}/company/${encodeURIComponent(owner.companyID)}/client`;
	const socket = new WebSocket(url, [tokenProtocol + owner.accessToken]);
	activeSocket = socket;
	await readyWithPresence(socket, (payload) => {
		if (activeSocket === socket) receive(payload);
	}, () => dropSocket(socket));
	redialsInARow = 0;
	return socket;
}

function receive(payload: Frame): void {
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

export async function callCompanyApp(call: HostCall, expectedOwner?: CompanyAppOwner): Promise<HostAnswer> {
	const wire = await companyWire(expectedOwner);
	if (wire !== activeSocket || (expectedOwner && (!wireOwner || !sameOwner(wireOwner, expectedOwner)))) throw new HostUnreachableError();
	const requestID = crypto.randomUUID();

	const answered = new Promise<HostAnswer | null>((resolve) => {
		const timer = setTimeout(() => {
			if (!waiting.delete(requestID)) return;
			resolve(null);
		}, answerTimeoutMilliseconds);
		waiting.set(requestID, (answer) => {
			clearTimeout(timer);
			resolve(answer);
		});
	});

	try {
		wire.send(JSON.stringify({ kind: 'call', requestID, capability: call.capability, body: call.body ?? {} }));
	} catch {
		waiting.get(requestID)?.(null);
		waiting.delete(requestID);
	}

	const answer = await answered;
	if (!answer) throw new HostUnreachableError();
	return answer;
}
