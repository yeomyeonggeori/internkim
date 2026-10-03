import { parseRoutedCall, serveRoutedCall, type GatewayAnswer } from './gateway-connector';
import type { Dispatch } from './forward';
import { FairOutbox } from './fair-outbox';
import { MessengerStreams } from './messenger-streams';
import { isStreamKind, parseStreamMessage, pingFrame } from '../../workers/connection-gateway/src/host-protocol';

const firstRetryMilliseconds = 500;
const longestRetryMilliseconds = 30_000;
export const pingEveryMilliseconds = 30_000;
export const silenceBeforeRedialMilliseconds = 75_000;

export function retryDelayMilliseconds(consecutiveFailures: number): number {
	const doubled = firstRetryMilliseconds * 2 ** Math.max(consecutiveFailures - 1, 0);
	return Math.min(doubled, longestRetryMilliseconds);
}

export function serverSocketURL(gatewayURL: string, companyID: string): string {
	return `${gatewayURL.replace(/\/+$/, '')}/company/${encodeURIComponent(companyID)}/server`;
}

export type GatewayConnection = {
	close: () => void;
	deliver: (event: Record<string, unknown>, audienceMemberIDs?: string[]) => void;
};

export function deliveryOf(
	event: Record<string, unknown>,
	audienceMemberIDs?: string[]
): { kind: 'deliver'; event: Record<string, unknown>; audienceMemberIDs?: string[] } {
	return { kind: 'deliver', event, ...(audienceMemberIDs?.length ? { audienceMemberIDs } : {}) };
}

export function hostSocketURL(gatewayURL: string, companyID: string): string {
	return `${gatewayURL.replace(/\/+$/, '')}/company/${encodeURIComponent(companyID)}/host`;
}

// Bun takes headers on the client handshake; the DOM type it is checked against
// does not describe that argument, and the server key belongs in a header
// rather than in a url that ends up in logs.
type SocketTakingHeaders = new (url: string, options: { headers: Record<string, string> }) => WebSocket;

function openHeaderWebSocket(url: string, headers: Record<string, string>): WebSocket {
	const HeaderWebSocket = WebSocket as unknown as SocketTakingHeaders;
	return new HeaderWebSocket(url, { headers });
}

export function connectToGateway(settings: {
	gatewayURL: string;
	companyID: string;
	serverKey?: string;
	hostAccessToken?: () => Promise<string>;
	dispatch: Dispatch;
	byteCeiling: number;
	messengerRelayURL: string;
	report?: (line: string) => void;
	pingMilliseconds?: number;
	silenceMilliseconds?: number;
}): GatewayConnection {
	const report = settings.report ?? ((line: string) => console.error(line));
	if (!settings.serverKey && !settings.hostAccessToken) {
		throw new Error('a gateway server key or host access token callback is required');
	}
	const url = settings.hostAccessToken
		? hostSocketURL(settings.gatewayURL, settings.companyID)
		: serverSocketURL(settings.gatewayURL, settings.companyID);
	let consecutiveFailures = 0;
	let socket: WebSocket | null = null;
	let isClosed = false;
	let lastHeardAt = Date.now();
	const outbox = new FairOutbox(() => {
		const open = socket;
		if (open?.readyState !== WebSocket.OPEN) return null;
		return { send: (document) => open.send(document), bufferedBytes: () => open.bufferedAmount };
	});
	const streams = new MessengerStreams(settings.messengerRelayURL, outbox, report);
	const pingMilliseconds = settings.pingMilliseconds ?? pingEveryMilliseconds;
	const silenceMilliseconds = settings.silenceMilliseconds ?? silenceBeforeRedialMilliseconds;
	const keepAlive = setInterval(() => {
		if (socket?.readyState !== WebSocket.OPEN) return;
		if (Date.now() - lastHeardAt > silenceMilliseconds) {
			report(`the gateway said nothing for ${silenceMilliseconds}ms; dialling again`);
			socket.close();
			return;
		}
		socket.send(pingFrame);
	}, pingMilliseconds);

	const dial = async () => {
		if (isClosed) return;
		try {
			const accessToken = settings.hostAccessToken
				? await settings.hostAccessToken()
				: settings.serverKey;
			if (!accessToken) throw new Error('the gateway access token was empty');
			if (isClosed) return;
			socket = openHeaderWebSocket(url, { Authorization: `Bearer ${accessToken}` });
		} catch (error) {
			report(`gateway authentication for ${settings.companyID} failed: ${error instanceof Error ? error.message : error}`);
			redial();
			return;
		}
		socket.addEventListener('open', () => {
			consecutiveFailures = 0;
			lastHeardAt = Date.now();
			report(`gateway connected for company ${settings.companyID}`);
		});
		socket.addEventListener('message', (message) => hear(message.data));
		socket.addEventListener('close', () => {
			streams.closeEverything();
			redial();
		});
		socket.addEventListener('error', () => report(`gateway connection for ${settings.companyID} failed`));
	};

	const redial = () => {
		if (isClosed) return;
		consecutiveFailures += 1;
		setTimeout(dial, retryDelayMilliseconds(consecutiveFailures));
	};

	// A refused call reaches the person as an empty screen and nothing else, so
	// the reason is said here. Without it the only evidence that the messenger
	// was even asked for anything is on the other side of the gateway.
	const hear = (data: unknown) => {
		lastHeardAt = Date.now();
		const payload = readJSON(data);
		const streamMessage = isStreamKind(kindOf(payload)) ? parseStreamMessage(payload) : null;
		if (streamMessage) {
			streams.take(streamMessage);
			return;
		}
		void answerOne(payload);
	};

	const answerOne = async (payload: unknown) => {
		const call = parseRoutedCall(payload);
		if (!call) return;
		const answer = await serveRoutedCall(call, settings.dispatch, settings.byteCeiling);
		if (answer.status >= 400) {
			const asked = call.memberID ? `member ${call.memberID}` : 'the plane';
			report(`${call.capability} for ${asked} answered ${answer.status}: ${reasonOf(answer.body)}`);
		}
		send(answer);
	};

	const send = (answer: GatewayAnswer) => {
		try {
			socket?.send(JSON.stringify(answer));
		} catch {
			report(`answer for ${answer.requestID} never reached the gateway`);
		}
	};

	const deliver = (event: Record<string, unknown>, audienceMemberIDs?: string[]) => {
		if (socket?.readyState !== WebSocket.OPEN) return;
		try {
			socket.send(JSON.stringify(deliveryOf(event, audienceMemberIDs)));
		} catch {
			report(`${String(event.kind)} never reached the gateway`);
		}
	};

	dial();
	return {
		close: () => {
			isClosed = true;
			clearInterval(keepAlive);
			socket?.close();
		},
		deliver
	};

}

export function reasonOf(body: unknown): string {
	if (typeof body === 'object' && body !== null) {
		const said = (body as { error?: unknown }).error;
		if (typeof said === 'string' && said.trim() !== '') return said;
	}
	return 'no reason given';
}

function kindOf(payload: unknown): unknown {
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
