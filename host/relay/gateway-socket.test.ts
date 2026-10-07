import { describe, expect, test } from 'bun:test';
import {
	connectToGateway,
	deliveryOf,
	hostSocketURL,
	reasonOf,
	retryDelayMilliseconds,
	serverSocketURL
} from './gateway-socket';
import type { Dispatch } from './forward';

describe('deliveryOf', () => {
	test('is the frame the gateway fans out, naming an audience only when there is one', () => {
		expect(deliveryOf({ kind: 'message.arrived', conversationID: 'channel-1' })).toEqual({
			kind: 'deliver',
			event: { kind: 'message.arrived', conversationID: 'channel-1' }
		});
		expect(deliveryOf({ kind: 'message.arrived' }, [])).toEqual({ kind: 'deliver', event: { kind: 'message.arrived' } });
		expect(deliveryOf({ kind: 'message.arrived' }, ['m1'])).toEqual({
			kind: 'deliver',
			event: { kind: 'message.arrived' },
			audienceMemberIDs: ['m1']
		});
	});
});


describe('retryDelayMilliseconds', () => {
	test('backs off and then stops growing', () => {
		expect(retryDelayMilliseconds(1)).toBe(500);
		expect(retryDelayMilliseconds(3)).toBe(2000);
		expect(retryDelayMilliseconds(20)).toBe(30_000);
	});
});

describe('reasonOf', () => {
	test('carries what the answer said', () => {
		expect(reasonOf({ error: 'this member has no messenger account' })).toBe('this member has no messenger account');
	});

	test('says so rather than printing an object nobody can read', () => {
		expect(reasonOf({ conversations: [] })).toBe('no reason given');
		expect(reasonOf('')).toBe('no reason given');
		expect(reasonOf(null)).toBe('no reason given');
	});
});

describe('serverSocketURL', () => {
	test('names the company whichever way the gateway url ends', () => {
		expect(serverSocketURL('wss://gateway.test/', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
		expect(serverSocketURL('wss://gateway.test', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
	});
});

describe('host gateway connections', () => {
	test('uses a fresh host token after the first socket closes', async () => {
		const authorizations: string[] = [];
		let opened = 0;
		let resolveSecondOpen: (() => void) | undefined;
		const secondOpen = new Promise<void>((resolve) => {
			resolveSecondOpen = resolve;
		});
		const server = Bun.serve<{ authorization: string | null }>({
			hostname: '127.0.0.1',
			port: 0,
			fetch(request, serverInstance) {
				if (serverInstance.upgrade(request, { data: { authorization: request.headers.get('Authorization') } })) return;
				return new Response('not found', { status: 404 });
			},
			websocket: {
				open(socket) {
					authorizations.push(socket.data.authorization ?? '');
					opened += 1;
					if (opened === 1) socket.close();
					if (opened === 2) resolveSecondOpen?.();
				},
				message() {}
			}
		});
		let tokenNumber = 0;
		const connection = connectToGateway({
			gatewayURL: server.url.toString().replace('http://', 'ws://'),
			companyID: 'company-1',
			hostAccessToken: async () => `token-${++tokenNumber}`,
			dispatch: {} as Dispatch,
			byteCeiling: 1024,
			messengerRelayURL: 'ws://127.0.0.1:1',
			report: () => undefined
		});
		await secondOpen;
		connection.close();
		await Bun.sleep(600);
		server.stop();
		expect(hostSocketURL(server.url.toString(), 'company-1')).toBe(`${server.url}company/company-1/host`);
		expect(authorizations).toEqual(['Bearer token-1', 'Bearer token-2']);
		expect(tokenNumber).toBe(2);
	});
});

describe('the host socket keeping itself alive', () => {
	function gatewayThat(answersPings: boolean): { url: string; opened: () => number; pings: () => number; stop: () => void } {
		let opened = 0;
		let pings = 0;
		const server = Bun.serve({
			hostname: '127.0.0.1',
			port: 0,
			fetch(request, serving) {
				if (serving.upgrade(request, { data: undefined })) return;
				return new Response('not found', { status: 404 });
			},
			websocket: {
				open() {
					opened += 1;
				},
				message(socket, message) {
					if (message !== '{"kind":"ping"}') return;
					pings += 1;
					if (answersPings) socket.send('{"kind":"pong"}');
				}
			}
		});
		return { url: `ws://127.0.0.1:${server.port}`, opened: () => opened, pings: () => pings, stop: () => server.stop(true) };
	}

	function connect(gatewayURL: string) {
		return connectToGateway({
			gatewayURL,
			companyID: 'company-1',
			hostAccessToken: async () => 'token',
			dispatch: {} as Dispatch,
			byteCeiling: 1024,
			messengerRelayURL: 'ws://127.0.0.1:1',
			report: () => undefined,
			pingMilliseconds: 20,
			silenceMilliseconds: 70
		});
	}

	test('pings, and stays on a gateway that answers', async () => {
		const gateway = gatewayThat(true);
		const connection = connect(gateway.url);
		await Bun.sleep(250);
		connection.close();
		gateway.stop();
		expect(gateway.pings()).toBeGreaterThan(3);
		expect(gateway.opened()).toBe(1);
	});

	test('dials again when the gateway has gone silent', async () => {
		const gateway = gatewayThat(false);
		const connection = connect(gateway.url);
		await Bun.sleep(1500);
		connection.close();
		gateway.stop();
		expect(gateway.opened()).toBeGreaterThan(1);
	});
});

describe('app streams over the host socket', () => {
	test('reach the relay on loopback and close when the gateway connection drops', async () => {
		const relayClosed: number[] = [];
		const relayHosts: (string | null)[] = [];
		const relay = Bun.serve<{ host: string | null }>({
			hostname: '127.0.0.1',
			port: 0,
			fetch(request, serving) {
				if (serving.upgrade(request, { data: { host: request.headers.get('host') } })) return;
				return new Response('not found', { status: 404 });
			},
			websocket: {
				open(socket) {
					relayHosts.push(socket.data.host);
					socket.send('["AUTH","challenge"]');
				},
				message() {},
				close(_socket, code) {
					relayClosed.push(code);
				}
			}
		});
		const heardByGateway: Record<string, unknown>[] = [];
		let gatewaySide: { close: () => void } | null = null;
		const gateway = Bun.serve({
			hostname: '127.0.0.1',
			port: 0,
			fetch(request, serving) {
				if (serving.upgrade(request, { data: undefined })) return;
				return new Response('not found', { status: 404 });
			},
			websocket: {
				open(socket) {
					gatewaySide = socket;
					socket.send(JSON.stringify({ kind: 'stream.open', streamID: 's1', host: 'acme.example.test', path: '/' }));
				},
				message(_socket, message) {
					heardByGateway.push(JSON.parse(String(message)));
				}
			}
		});
		const connection = connectToGateway({
			gatewayURL: `ws://127.0.0.1:${gateway.port}`,
			companyID: 'company-1',
			hostAccessToken: async () => 'token',
			dispatch: {} as Dispatch,
			byteCeiling: 1024,
			messengerRelayURL: `ws://127.0.0.1:${relay.port}`,
			report: () => undefined
		});
		for (let round = 0; round < 500 && heardByGateway.length === 0; round += 1) await Bun.sleep(10);
		expect(relayHosts).toEqual(['acme.example.test']);
		expect(heardByGateway).toEqual([{ kind: 'stream.frame', streamID: 's1', data: '["AUTH","challenge"]' }]);

		(gatewaySide as { close: () => void } | null)?.close();
		for (let round = 0; round < 500 && relayClosed.length === 0; round += 1) await Bun.sleep(10);
		connection.close();
		gateway.stop(true);
		relay.stop(true);
		expect(relayClosed).toHaveLength(1);
	});
});
