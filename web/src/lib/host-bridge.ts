import { gatewayURL, supabase } from '$lib/supabase';

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

let joined: Promise<WebSocket> | undefined;
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
	joined = undefined;
	isServerConnected = false;
}

async function openWire(): Promise<WebSocket> {
	const address = gatewayURL();
	if (!address) throw new HostUnreachableError();

	const client = supabase();
	const { data } = await client.auth.getSession();
	const token = data.session?.access_token;
	if (!token) throw new Error('sign in first');

	const member = await client
		.from('member')
		.select('company_id')
		.eq('user_id', data.session?.user.id ?? '')
		.single<{ company_id: string }>();
	if (member.error) throw new Error(member.error.message);

	const url = `${address.replace(/\/+$/, '')}/company/${encodeURIComponent(member.data.company_id)}/client`;
	const socket = new WebSocket(url, [tokenProtocol + token]);
	socket.addEventListener('message', (message) => receive(message.data));
	socket.addEventListener('close', dropTheWire);

	await new Promise<void>((resolve, reject) => {
		socket.addEventListener('open', () => resolve(), { once: true });
		socket.addEventListener('error', () => reject(new HostUnreachableError()), { once: true });
	});
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
	if (payload.kind !== 'result' || typeof payload.requestID !== 'string') return;
	waiting.get(payload.requestID)?.({
		status: typeof payload.status === 'number' ? payload.status : 500,
		body: payload.body
	});
	waiting.delete(payload.requestID);
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
