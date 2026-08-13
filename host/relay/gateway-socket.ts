import { parseRoutedCall, serveRoutedCall, type GatewayAnswer } from './gateway-connector';
import type { Dispatch } from './forward';

const firstRetryMilliseconds = 500;
const longestRetryMilliseconds = 30_000;

export function retryDelayMilliseconds(consecutiveFailures: number): number {
	const doubled = firstRetryMilliseconds * 2 ** Math.max(consecutiveFailures - 1, 0);
	return Math.min(doubled, longestRetryMilliseconds);
}

export function serverSocketURL(gatewayURL: string, companyID: string): string {
	return `${gatewayURL.replace(/\/+$/, '')}/company/${encodeURIComponent(companyID)}/server`;
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
	dispatch: Dispatch;
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
		const answer = await serveRoutedCall(call, settings.dispatch);
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
