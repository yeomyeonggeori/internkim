import { afterEach, describe, expect, test } from 'bun:test';
import type { ServerWebSocket } from 'bun';
import { FairOutbox } from './fair-outbox';
import { MessengerStreams } from './messenger-streams';

type Dialled = { host: string | null; path: string; forwardedFor: string | null };

type FakeRelay = {
	url: string;
	dialled: Dialled[];
	heard: string[];
	sockets: ServerWebSocket<Dialled>[];
	closed: number[];
	stop: () => void;
};

const running: FakeRelay[] = [];

afterEach(() => {
	for (const relay of running.splice(0)) relay.stop();
});

function fakeRelay(): FakeRelay {
	const dialled: Dialled[] = [];
	const heard: string[] = [];
	const sockets: ServerWebSocket<Dialled>[] = [];
	const closed: number[] = [];
	const server = Bun.serve<Dialled>({
		hostname: '127.0.0.1',
		port: 0,
		fetch(request, serving) {
			const url = new URL(request.url);
			const data = {
				host: request.headers.get('host'),
				path: `${url.pathname}${url.search}`,
				forwardedFor: request.headers.get('x-forwarded-for')
			};
			if (serving.upgrade(request, { data })) return;
			return new Response('not a socket', { status: 400 });
		},
		websocket: {
			open(socket) {
				dialled.push(socket.data);
				sockets.push(socket);
			},
			message(_socket, message) {
				heard.push(String(message));
			},
			close(_socket, code) {
				closed.push(code);
			}
		}
	});
	const relay = { url: `ws://127.0.0.1:${server.port}`, dialled, heard, sockets, closed, stop: () => server.stop(true) };
	running.push(relay);
	return relay;
}

function gatewayOutlet(): { outbox: FairOutbox; sent: Record<string, unknown>[] } {
	const sent: Record<string, unknown>[] = [];
	const outbox = new FairOutbox(() => ({ send: (document) => sent.push(JSON.parse(document)), bufferedBytes: () => 0 }));
	return { outbox, sent };
}

async function until(condition: () => boolean): Promise<void> {
	for (let round = 0; round < 200; round += 1) {
		if (condition()) return;
		await Bun.sleep(5);
	}
	throw new Error('the condition never held');
}

describe('MessengerStreams', () => {
	test('dials the relay on loopback presenting the address the app dialled, and passes frames sent before it opened', async () => {
		const relay = fakeRelay();
		const { outbox } = gatewayOutlet();
		const streams = new MessengerStreams(relay.url, outbox, () => undefined);
		streams.take({ kind: 'stream.open', streamID: 's1', host: 'acme.example.test', path: '/huddle/c1/audio', forwardedFor: '192.0.2.4' });
		streams.take({ kind: 'stream.frame', streamID: 's1', data: '["AUTH",{}]' });

		await until(() => relay.heard.length === 1);
		expect(relay.dialled).toEqual([{ host: 'acme.example.test', path: '/huddle/c1/audio', forwardedFor: '192.0.2.4' }]);
		expect(relay.heard).toEqual(['["AUTH",{}]']);
	});

	test('carries what the relay says back to the gateway, keyed by the stream', async () => {
		const relay = fakeRelay();
		const { outbox, sent } = gatewayOutlet();
		const streams = new MessengerStreams(relay.url, outbox, () => undefined);
		streams.take({ kind: 'stream.open', streamID: 's1', host: 'acme.example.test', path: '/' });
		await until(() => relay.sockets.length === 1);

		relay.sockets[0]?.send('["AUTH","challenge"]');
		relay.sockets[0]?.send(new Uint8Array([7, 8]));
		await until(() => sent.length === 2);
		expect(sent).toEqual([
			{ kind: 'stream.frame', streamID: 's1', data: '["AUTH","challenge"]' },
			{ kind: 'stream.frame', streamID: 's1', data: 'Bwg=', isBinary: true }
		]);
	});

	test('refuses a relay frame over 512 KiB before it enters the envelope, and closes both sides', async () => {
		const relay = fakeRelay();
		const { outbox, sent } = gatewayOutlet();
		const streams = new MessengerStreams(relay.url, outbox, () => undefined);
		streams.take({ kind: 'stream.open', streamID: 's1', host: 'acme.example.test', path: '/' });
		await until(() => relay.sockets.length === 1);

		relay.sockets[0]?.send('x'.repeat(512 * 1024 + 1));
		await until(() => sent.length === 1 && relay.closed.length === 1);
		expect(sent).toEqual([{ kind: 'stream.close', streamID: 's1', code: 1009, reason: 'a frame may carry at most 512 KiB' }]);
		expect(streams.openCount).toBe(0);
	});

	test('tells the gateway when the relay closes a stream', async () => {
		const relay = fakeRelay();
		const { outbox, sent } = gatewayOutlet();
		const streams = new MessengerStreams(relay.url, outbox, () => undefined);
		streams.take({ kind: 'stream.open', streamID: 's1', host: 'acme.example.test', path: '/' });
		await until(() => relay.sockets.length === 1);

		relay.sockets[0]?.close(4003, 'restricted');
		await until(() => sent.length === 1);
		expect(sent).toEqual([{ kind: 'stream.close', streamID: 's1', code: 4003, reason: 'restricted' }]);
	});

	test('closes every relay connection when the gateway connection goes', async () => {
		const relay = fakeRelay();
		const { outbox, sent } = gatewayOutlet();
		const streams = new MessengerStreams(relay.url, outbox, () => undefined);
		streams.take({ kind: 'stream.open', streamID: 's1', host: 'acme.example.test', path: '/' });
		streams.take({ kind: 'stream.open', streamID: 's2', host: 'acme.example.test', path: '/' });
		await until(() => relay.sockets.length === 2);

		streams.closeEverything();
		await until(() => relay.closed.length === 2);
		expect(streams.openCount).toBe(0);
		expect(sent).toEqual([]);
	});

	test('a frame for a stream it no longer holds is answered with a close', () => {
		const { outbox, sent } = gatewayOutlet();
		const streams = new MessengerStreams('ws://127.0.0.1:1', outbox, () => undefined);
		streams.take({ kind: 'stream.frame', streamID: 'gone', data: '[]' });
		expect(sent).toEqual([{ kind: 'stream.close', streamID: 'gone', code: 1000, reason: 'the relay connection has gone' }]);
	});
});
