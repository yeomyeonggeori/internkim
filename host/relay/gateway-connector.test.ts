import { describe, expect, test } from 'bun:test';
import { parseRoutedCall, serveRoutedCall } from './gateway-connector';
import type { Dispatch } from './forward';

const ceiling = 3_000_000;
const call = {
	kind: 'call' as const,
	requestID: 'r1',
	memberID: 'm1',
	capability: 'person.messenger.channels',
	body: { limit: 20 }
};

function dispatchAnswering(answer: unknown, seen: { capability?: string } = {}): Dispatch {
	return {
		serveAsset: async () => answer,
		askChatd: async (capability: string) => {
			seen.capability = capability;
			return { status: 200, body: answer };
		},
		askMaild: async () => ({ status: 200, body: null }),
		askAdmind: async () => ({ status: 200, body: null }),
		mailAccountOf: async () => null,
		emailOfMember: async () => 'someone@example.com',
		connectMessengerAccount: async () => {}
	} as unknown as Dispatch;
}

describe('parseRoutedCall', () => {
	test('takes a call naming a member and a capability', () => {
		expect(parseRoutedCall(call)).toEqual(call);
	});

	test('refuses a call missing what it needs', () => {
		expect(parseRoutedCall({ ...call, memberID: '' })).toBeNull();
		expect(parseRoutedCall({ ...call, capability: '  ' })).toBeNull();
		expect(parseRoutedCall({ ...call, kind: 'result' })).toBeNull();
		expect(parseRoutedCall('call')).toBeNull();
	});
});

describe('serveRoutedCall', () => {
	test('serves the capability as the member the gateway named', async () => {
		const seen: { capability?: string } = {};
		const result = await serveRoutedCall(call, dispatchAnswering({ channels: [] }, seen), ceiling);
		expect(result).toEqual({ kind: 'result', requestID: 'r1', status: 200, body: { channels: [] } });
		expect(seen.capability).toBe('person.messenger.channels');
	});

	test('needs no credential in the body to know who is asking', async () => {
		const result = await serveRoutedCall({ ...call, body: {} }, dispatchAnswering({ channels: [] }), ceiling);
		expect(result.status).toBe(200);
	});

	test('reports a capability that threw rather than dropping the caller', async () => {
		const dispatch = dispatchAnswering(null);
		const throwing = {
			...dispatch,
			askChatd: () => Promise.reject(new Error('chatd is not running'))
		} as unknown as Dispatch;
		expect(await serveRoutedCall(call, throwing, ceiling)).toEqual({
			kind: 'result',
			requestID: 'r1',
			status: 500,
			body: { error: 'chatd is not running' }
		});
	});
});

describe('an answer too big for the socket', () => {
	test('comes back as a refusal naming the size, rather than vanishing', async () => {
		const wide = { rows: 'x'.repeat(2_000) };
		const result = await serveRoutedCall(call, dispatchAnswering(wide), 500);

		expect(result.status).toBe(413);
		expect(String((result.body as { error: string }).error)).toContain('500');
	});
})
