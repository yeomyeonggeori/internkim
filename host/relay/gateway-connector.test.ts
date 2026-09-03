import { describe, expect, test } from 'bun:test';
import { parseRoutedCall, serveRoutedCall } from './gateway-connector';
import type { AdmindCall, Dispatch, PublicAPIRequest } from './forward';

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
		tellAdmindTheDirectoryChanged: async () => ({ status: 202, body: null }),
		emailOfMember: async () => 'someone@example.com',
		messengerCredentialOf: async () => ({ kind: 'buzz-secret', secret: 'a-held-secret' }),
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

describe('a call from the public API', () => {
	const apiCall = {
		kind: 'call' as const,
		requestID: 'r2',
		capability: 'person.api.request',
		body: {
			method: 'POST',
			path: '/tools/message_send/invoke',
			query: '',
			permission: 'write',
			requester: 'sample@example.test',
			payload: { conversationID: 'channel-1' }
		}
	};

	test('names no member on the envelope, and is taken all the same', () => {
		expect(parseRoutedCall(apiCall)).toEqual({ ...apiCall, memberID: null });
	});

	test('a member named as nothing at all is still refused', () => {
		expect(parseRoutedCall({ ...apiCall, memberID: '' })).toBeNull();
		expect(parseRoutedCall({ ...apiCall, memberID: 7 })).toBeNull();
	});

	test('is served under the requester the body names', async () => {
		const asked: { requester?: string; permission?: string; path?: string } = {};
		const dispatch = {
			...dispatchAnswering(null),
			askAdmindAPI: async (request: PublicAPIRequest) => {
				asked.requester = request.requester;
				asked.permission = request.permission;
				asked.path = request.path;
				return { status: 200, body: { invoked: true } };
			}
		};

		const result = await serveRoutedCall({ ...apiCall, memberID: null }, dispatch, ceiling);

		expect(result).toEqual({ kind: 'result', requestID: 'r2', status: 200, body: { invoked: true } });
		expect(asked).toEqual({
			requester: 'sample@example.test',
			permission: 'write',
			path: '/tools/message_send/invoke'
		});
	});

	test('arriving over a member connection, it is refused whatever the body claims', async () => {
		const asked: { path?: string } = {};
		const dispatch = {
			...dispatchAnswering(null),
			askAdmindAPI: async (request: PublicAPIRequest) => {
				asked.path = request.path;
				return { status: 200, body: { invoked: true } };
			}
		};

		const result = await serveRoutedCall({ ...apiCall, memberID: 'm1' }, dispatch, ceiling);

		expect(result.status).toBe(403);
		expect(asked.path).toBeUndefined();
	});

	test('anything else arriving for no member is told so rather than served as nobody', async () => {
		const result = await serveRoutedCall(
			{ ...call, memberID: null },
			dispatchAnswering({ channels: [] }),
			ceiling
		);

		expect(result.status).toBe(400);
	});
});

describe('a telling the plane sends as the bot', () => {
	const telling = {
		kind: 'call' as const,
		requestID: 'r3',
		capability: 'person.message.tell',
		body: { recipientEmail: 'sample@example.test', message: '결재를 기다리는 건이 있습니다' }
	};

	test('names no member on the envelope, and is taken all the same', () => {
		expect(parseRoutedCall(telling)).toEqual({ ...telling, memberID: null });
	});

	test('is delivered to admind though the recipient holds no messenger credential', async () => {
		const calls: AdmindCall[] = [];
		const dispatch = {
			...dispatchAnswering(null),
			messengerCredentialOf: async () => null,
			askAdmindAsRequester: async (call: AdmindCall) => {
				calls.push(call);
				return { status: 200, body: { delivered: true } };
			}
		} as unknown as Dispatch;

		const result = await serveRoutedCall({ ...telling, memberID: null }, dispatch, ceiling);

		expect(result).toEqual({ kind: 'result', requestID: 'r3', status: 200, body: { delivered: true } });
		expect(calls[0]?.url).toBe('http://internkim/tell/api/direct-message');
	});

	test('arriving over a member connection, it is refused rather than sent as the bot', async () => {
		const calls: AdmindCall[] = [];
		const dispatch = {
			...dispatchAnswering(null),
			askAdmindAsRequester: async (call: AdmindCall) => {
				calls.push(call);
				return { status: 200, body: { delivered: true } };
			}
		} as unknown as Dispatch;

		const result = await serveRoutedCall({ ...telling, memberID: 'm1' }, dispatch, ceiling);

		expect(result.status).toBe(403);
		expect(calls).toHaveLength(0);
	});
});
