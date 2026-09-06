import { afterEach, describe, expect, jest, mock, test } from 'bun:test';

const sockets: FakeWebSocket[] = [];

class FakeWebSocket {
	static readonly OPEN = 1;
	readonly listeners = new Map<string, ((event: { data?: unknown }) => void)[]>();
	readonly url: string;
	readyState = 0;

	constructor(url: string) {
		this.url = url;
		sockets.push(this);
	}

	addEventListener(name: string, listener: (event: { data?: unknown }) => void): void {
		this.listeners.set(name, [...(this.listeners.get(name) ?? []), listener]);
	}

	close(): void {
		this.readyState = 3;
		this.emit('close');
	}

	send(): void {}

	emit(name: string, event: { data?: unknown } = {}): void {
		for (const listener of this.listeners.get(name) ?? []) listener(event);
	}
}

mock.module('$lib/supabase', () => ({
	gatewayURL: () => 'wss://gateway.example.com',
	supabase: () => ({ auth: { getSession: async () => ({ data: { session: { access_token: 'member-token' } } }) } })
}));
mock.module('$lib/supabase-session', () => ({ supabaseMember: async () => ({ companyID: 'company-1' }) }));
Object.assign(globalThis, { WebSocket: FakeWebSocket });

const { HostUnreachableError, isCompanyAppRunning } = await import('../../src/lib/host-bridge');

afterEach(() => {
	sockets.at(-1)?.close();
	sockets.splice(0);
});

async function openSocket(): Promise<FakeWebSocket> {
	for (let turn = 0; turn < 4; turn += 1) await Promise.resolve();
	const socket = sockets.at(-1);
	if (!socket) throw new Error('the test opened no socket');
	socket.emit('open');
	return socket;
}

describe('isCompanyAppRunning', () => {
	test('waits for delayed presence and reports the connected server', async () => {
		const running = isCompanyAppRunning();
		const socket = await openSocket();
		let settled = false;
		running.then(() => {
			settled = true;
		});
		await Promise.resolve();
		expect(settled).toBe(false);
		socket.emit('message', { data: JSON.stringify({ kind: 'presence', isServerConnected: true, serverSeenAt: 1 }) });
		expect(await running).toBe(true);
	});

	test('waits for delayed presence and reports an offline server', async () => {
		const running = isCompanyAppRunning();
		const socket = await openSocket();
		socket.emit('message', { data: JSON.stringify({ kind: 'presence', isServerConnected: false, serverSeenAt: 0 }) });
		expect(await running).toBe(false);
	});

	test('rejects when the socket closes before presence', async () => {
		const running = isCompanyAppRunning();
		const socket = await openSocket();
		socket.close();
		expect(running).rejects.toBeInstanceOf(HostUnreachableError);
	});

	test('rejects when the socket errors before presence', async () => {
		const running = isCompanyAppRunning();
		const socket = await openSocket();
		socket.emit('error');
		expect(running).rejects.toBeInstanceOf(HostUnreachableError);
	});

	test('rejects when presence never arrives', async () => {
		jest.useFakeTimers();
		const running = isCompanyAppRunning();
		await openSocket();
		jest.advanceTimersByTime(5_001);
		expect(running).rejects.toBeInstanceOf(HostUnreachableError);
		jest.useRealTimers();
	});
});
