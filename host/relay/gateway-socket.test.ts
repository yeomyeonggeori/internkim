import { describe, expect, test } from 'bun:test';
import { connectToGateway, hostSocketURL, reasonOf, retryDelayMilliseconds, serverSocketURL } from './gateway-socket';
import type { Dispatch } from './forward';

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
