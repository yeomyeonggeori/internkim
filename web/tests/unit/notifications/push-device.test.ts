import { afterAll, beforeEach, describe, expect, mock, test } from 'bun:test';
import { createMockFetch } from '../test-fetch';

const publicAPICall = await import('../../../src/lib/public-api-call');

mock.module('$lib/public-api-call', () => ({
	...publicAPICall,
	memberAccessToken: async () => 'a-session'
}));

const { askPushReachability, claimPushDevice, releasePushDevice } = await import(
	'../../../src/lib/notifications/push-device'
);

type Asked = { address: string; method: string; authorization: string; body: string };

const reachable = { serverKey: 'a-server-key', isServerKeyVaulted: true, hasClaimedDevice: true };
const heldFetch = globalThis.fetch;
const asked: Asked[] = [];
let answer = new Response(JSON.stringify(reachable), { status: 200 });

globalThis.fetch = createMockFetch(async (input, init) => {
	asked.push({
		address: String(input),
		method: init?.method ?? 'GET',
		authorization: new Headers(init?.headers).get('Authorization') ?? '',
		body: typeof init?.body === 'string' ? init.body : ''
	});
	return answer;
});

beforeEach(() => {
	asked.length = 0;
	answer = new Response(JSON.stringify(reachable), { status: 200 });
});

afterAll(() => {
	globalThis.fetch = heldFetch;
	mock.module('$lib/public-api-call', () => publicAPICall);
});

describe('the push device the app registers', () => {
	test('is read as the signed-in member', async () => {
		expect(await askPushReachability()).toEqual(reachable);
		expect(asked).toEqual([
			{ address: '/api/member/push-device', method: 'GET', authorization: 'Bearer a-session', body: '' }
		]);
	});

	test('is claimed with the fields the subscription carries', async () => {
		await claimPushDevice({ endpoint: 'https://push.example.test/1', publicKey: 'a-key', authenticationSecret: 'a-secret' });

		expect(asked[0].method).toBe('PUT');
		expect(JSON.parse(asked[0].body)).toEqual({
			endpoint: 'https://push.example.test/1',
			publicKey: 'a-key',
			authenticationSecret: 'a-secret'
		});
	});

	test('is released by the address and kind in the query', async () => {
		await releasePushDevice({ endpoint: 'a-device-token', kind: 'apns' });

		expect(asked[0].method).toBe('DELETE');
		expect(asked[0].address).toBe('/api/member/push-device?endpoint=a-device-token&kind=apns');
	});

	test('says why the endpoint refused', async () => {
		answer = new Response(JSON.stringify({ message: 'a claimed subscription carries both of the keys' }), {
			status: 400
		});

		await expect(claimPushDevice({ endpoint: 'https://push.example.test/1' })).rejects.toThrow(
			'a claimed subscription carries both of the keys'
		);
	});
});
