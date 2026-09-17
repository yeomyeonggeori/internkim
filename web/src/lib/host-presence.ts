export class HostUnreachableError extends Error {
	constructor() {
		super('the company app is not running');
		this.name = 'HostUnreachableError';
	}
}

export const presenceTimeoutMilliseconds = 5_000;

export type Frame = Record<string, unknown>;

export function frameOf(data: unknown): Frame | null {
	if (typeof data !== 'string') return null;
	let payload: unknown;
	try {
		payload = JSON.parse(data);
	} catch {
		return null;
	}
	return typeof payload === 'object' && payload !== null ? (payload as Frame) : null;
}

export function isPresence(frame: Frame): boolean {
	return frame.kind === 'presence' && typeof frame.isServerConnected === 'boolean';
}

export function readyWithPresence(
	socket: WebSocket,
	receive: (frame: Frame) => void,
	dropped: () => void
): Promise<void> {
	return new Promise<void>((resolve, reject) => {
		let hasOpened = false;
		let hasPresence = false;
		let isSettled = false;
		const settle = (error?: HostUnreachableError): void => {
			if (isSettled) return;
			isSettled = true;
			clearTimeout(presenceTimer);
			if (error) reject(error);
			else resolve();
		};
		const presenceTimer = setTimeout(() => {
			settle(new HostUnreachableError());
			socket.close();
		}, presenceTimeoutMilliseconds);
		socket.addEventListener('message', (message) => {
			const frame = frameOf(message.data);
			if (!frame) return;
			receive(frame);
			if (!isPresence(frame)) return;
			hasPresence = true;
			if (hasOpened) settle();
		});
		socket.addEventListener('close', () => {
			dropped();
			settle(new HostUnreachableError());
		});
		socket.addEventListener(
			'open',
			() => {
				hasOpened = true;
				if (hasPresence) settle();
			},
			{ once: true }
		);
		socket.addEventListener('error', () => settle(new HostUnreachableError()), { once: true });
	});
}
