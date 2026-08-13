import { parseRoutedCall, serveRoutedCall, type BuzzPublisher, type GatewayAnswer } from './gateway-connector';

const firstRetryMilliseconds = 500;
const longestRetryMilliseconds = 30_000;
const publishDeadlineMilliseconds = 8_000;

export function retryDelayMilliseconds(consecutiveFailures: number): number {
	const doubled = firstRetryMilliseconds * 2 ** Math.max(consecutiveFailures - 1, 0);
	return Math.min(doubled, longestRetryMilliseconds);
}

export type PublishOutcome = { eventID: string; isStored: boolean; refusal: string };

// A nostr relay answers a published event with ["OK", <id>, <stored>, <reason>].
export function publishOutcomeOf(frame: unknown): PublishOutcome | null {
	if (!Array.isArray(frame) || frame[0] !== 'OK') return null;
	const [, eventID, isStored, refusal] = frame;
	if (typeof eventID !== 'string' || typeof isStored !== 'boolean') return null;
	return { eventID, isStored, refusal: typeof refusal === 'string' ? refusal : '' };
}

export function serverSocketURL(gatewayURL: string, companyID: string): string {
	return `${gatewayURL.replace(/\/+$/, '')}/company/${encodeURIComponent(companyID)}/server`;
}

export function buzzPublisherOn(buzzRelayURL: string, pubkeyOfMember: BuzzPublisher['pubkeyOfMember']): BuzzPublisher {
	return {
		pubkeyOfMember,
		publish: (event) => publishToBuzz(buzzRelayURL, event)
	};
}

function publishToBuzz(buzzRelayURL: string, event: Record<string, unknown>): Promise<void> {
	return new Promise((resolve, reject) => {
		const socket = new WebSocket(buzzRelayURL);
		const giveUp = setTimeout(() => {
			socket.close();
			reject(new Error(`the buzz relay did not answer within ${publishDeadlineMilliseconds}ms`));
		}, publishDeadlineMilliseconds);

		const settle = (refusal?: Error) => {
			clearTimeout(giveUp);
			socket.close();
			if (refusal) reject(refusal);
			else resolve();
		};

		socket.addEventListener('open', () => socket.send(JSON.stringify(['EVENT', event])));
		socket.addEventListener('error', () => settle(new Error('the buzz relay refused the connection')));
		socket.addEventListener('message', (message) => {
			const outcome = publishOutcomeOf(readJSON(message.data));
			if (!outcome || outcome.eventID !== event.id) return;
			settle(outcome.isStored ? undefined : new Error(outcome.refusal || 'the buzz relay rejected the event'));
		});
	});
}

export type GatewayConnection = { close: () => void };

// Bun takes headers on the client handshake; the DOM type it is checked against
// does not describe that argument, and the server key belongs in a header
// rather than in a url that ends up in logs.
type SocketTakingHeaders = new (url: string, options: { headers: Record<string, string> }) => WebSocket;
const HeaderWebSocket = WebSocket as unknown as SocketTakingHeaders;

export function connectToGateway(settings: {
	gatewayURL: string;
	companyID: string;
	serverKey: string;
	publisher: BuzzPublisher;
	report?: (line: string) => void;
}): GatewayConnection {
	const report = settings.report ?? ((line: string) => console.error(line));
	const url = serverSocketURL(settings.gatewayURL, settings.companyID);
	let consecutiveFailures = 0;
	let socket: WebSocket | null = null;
	let isClosed = false;

	const dial = () => {
		if (isClosed) return;
		socket = new HeaderWebSocket(url, { headers: { Authorization: `Bearer ${settings.serverKey}` } });
		socket.addEventListener('open', () => {
			consecutiveFailures = 0;
			report(`gateway connected for company ${settings.companyID}`);
		});
		socket.addEventListener('message', (message) => void answerOne(message.data));
		socket.addEventListener('close', redial);
		socket.addEventListener('error', () => report(`gateway connection for ${settings.companyID} failed`));
	};

	const redial = () => {
		if (isClosed) return;
		consecutiveFailures += 1;
		setTimeout(dial, retryDelayMilliseconds(consecutiveFailures));
	};

	const answerOne = async (data: unknown) => {
		const call = parseRoutedCall(readJSON(data));
		if (!call) return;
		const answer = await serveRoutedCall(call, settings.publisher);
		send(answer);
	};

	const send = (answer: GatewayAnswer) => {
		try {
			socket?.send(JSON.stringify(answer));
		} catch {
			report(`answer for ${answer.requestID} never reached the gateway`);
		}
	};

	dial();
	return {
		close: () => {
			isClosed = true;
			socket?.close();
		}
	};
}

function readJSON(data: unknown): unknown {
	if (typeof data !== 'string') return null;
	try {
		return JSON.parse(data);
	} catch {
		return null;
	}
}
