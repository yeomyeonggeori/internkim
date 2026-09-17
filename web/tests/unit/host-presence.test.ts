import { afterEach, describe, expect, jest, test } from 'bun:test';
import { HostUnreachableError, frameOf, isPresence, readyWithPresence, type Frame } from '../../src/lib/host-presence';

class FakeWebSocket {
	readonly listeners = new Map<string, ((event: { data?: unknown }) => void)[]>();
	isClosed = false;

	addEventListener(name: string, listener: (event: { data?: unknown }) => void): void {
		this.listeners.set(name, [...(this.listeners.get(name) ?? []), listener]);
	}

	close(): void {
		this.isClosed = true;
		this.emit('close');
	}

	emit(name: string, event: { data?: unknown } = {}): void {
		for (const listener of this.listeners.get(name) ?? []) listener(event);
	}
}

function presenceFrame(isServerConnected: boolean): { data: string } {
	return { data: JSON.stringify({ kind: 'presence', isServerConnected, serverSeenAt: 1 }) };
}

function watch(socket: FakeWebSocket): { ready: Promise<void>; received: Frame[]; drops: number } {
	const outcome = { received: [] as Frame[], drops: 0, ready: Promise.resolve() };
	outcome.ready = readyWithPresence(
		socket as unknown as WebSocket,
		(frame) => outcome.received.push(frame),
		() => {
			outcome.drops += 1;
		}
	);
	return outcome;
}

afterEach(() => {
	jest.useRealTimers();
});

describe('readyWithPresence', () => {
	test('settles only once the socket is open and the gateway has said whether the server is there', async () => {
		const socket = new FakeWebSocket();
		const { ready, received } = watch(socket);
		let isSettled = false;
		void ready.then(() => {
			isSettled = true;
		});
		socket.emit('open');
		await Promise.resolve();
		expect(isSettled).toBe(false);
		socket.emit('message', presenceFrame(true));
		await ready;
		expect(received).toEqual([{ kind: 'presence', isServerConnected: true, serverSeenAt: 1 }]);
	});

	test('accepts presence that arrives before the open event', async () => {
		const socket = new FakeWebSocket();
		const { ready } = watch(socket);
		socket.emit('message', presenceFrame(false));
		socket.emit('open');
		await ready;
	});

	test('hands every frame on, presence or not, and ignores what is not a frame', async () => {
		const socket = new FakeWebSocket();
		const { ready, received } = watch(socket);
		socket.emit('open');
		socket.emit('message', { data: 'not json' });
		socket.emit('message', { data: JSON.stringify({ kind: 'deliver', event: {} }) });
		socket.emit('message', presenceFrame(true));
		await ready;
		expect(received.map((frame) => frame.kind)).toEqual(['deliver', 'presence']);
	});

	test('rejects and reports the drop when the socket closes before presence', async () => {
		const socket = new FakeWebSocket();
		const outcome = watch(socket);
		socket.emit('open');
		socket.close();
		await expect(outcome.ready).rejects.toBeInstanceOf(HostUnreachableError);
		expect(outcome.drops).toBe(1);
	});

	test('rejects when the socket errors before presence', async () => {
		const socket = new FakeWebSocket();
		const { ready } = watch(socket);
		socket.emit('error');
		await expect(ready).rejects.toBeInstanceOf(HostUnreachableError);
	});

	test('gives up and closes the socket when presence never arrives', async () => {
		jest.useFakeTimers();
		const socket = new FakeWebSocket();
		const { ready } = watch(socket);
		socket.emit('open');
		jest.advanceTimersByTime(5_001);
		await expect(ready).rejects.toBeInstanceOf(HostUnreachableError);
		expect(socket.isClosed).toBe(true);
	});
});

describe('frameOf and isPresence', () => {
	test('reads only JSON objects and recognises a presence frame by its shape', () => {
		expect(frameOf(42)).toBeNull();
		expect(frameOf('"text"')).toBeNull();
		expect(isPresence({ kind: 'presence', isServerConnected: false })).toBe(true);
		expect(isPresence({ kind: 'presence' })).toBe(false);
		expect(isPresence({ kind: 'deliver' })).toBe(false);
	});
});
