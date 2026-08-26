import { afterEach, describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { notifyMember } from '../../src/lib/server/notify-member';
import { encodeBase64URL } from '../../src/lib/notifications/base64url';

const vapid = {
	publicKey: 'BG0w6CuCogoJKa593BzjeAk_VAOmSYtz4Crk7OBQPEYa3_peOcMJEln_GG6LyW-0nl82LPHDClzU8_0nB4Z5dcs',
	privateKey: 'NNK7ZJuRBBHnpKs9X0R0aM4Tff6BaVUfPwmnYTdPuWA',
	subject: 'mailto:support@example.com'
};

const notification = { title: '이샘플', body: '보냈어요', openPath: '/messenger/', tag: 'message:channel-a' };

async function reachableKeys() {
	const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
	return {
		p256dh: encodeBase64URL(await crypto.subtle.exportKey('raw', pair.publicKey)),
		auth: encodeBase64URL(crypto.getRandomValues(new Uint8Array(16)).buffer)
	};
}

function clientWhere(options: {
	settings: unknown;
	mutedConversations: string[];
	unmutedConversations?: string[];
	deviceKeys: unknown;
}) {
	const client = {
		from(table: string) {
			if (table === 'member') {
				return {
					select: () => ({
						eq: () => ({ maybeSingle: async () => ({ data: { notification_settings: options.settings }, error: null }) })
					})
				};
			}
			if (table === 'notification') {
				return {
					select: () => ({
						eq: () => ({
							eq: (_column: string, conversationID: string) => ({
								maybeSingle: async () => ({
									data: options.mutedConversations.includes(conversationID)
										? { is_muted: true }
										: (options.unmutedConversations ?? []).includes(conversationID)
											? { is_muted: false }
											: null,
									error: null
								})
							})
						})
					})
				};
			}
			return {
				select: () => ({
					eq: () => ({
						eq: () => ({
							returns: async () => ({
								data: [{ kind: 'web-push', address: 'https://push.example.com/one', keys: options.deviceKeys }],
								error: null
							})
						})
					})
				}),
				delete: () => ({ eq: async () => ({ error: null }) })
			};
		}
	} as unknown as SupabaseClient;
	return client;
}

const realFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = realFetch;
});

function answerWith(status: number) {
	const asked: string[] = [];
	globalThis.fetch = Object.assign(
		async (input: RequestInfo | URL) => {
			asked.push(String(input));
			return new Response(null, { status });
		},
		{ preconnect: realFetch.preconnect }
	);
	return asked;
}

describe('a conversation a member has muted', () => {
	test('is silent even though the category is on', async () => {
		const client = clientWhere({
			settings: { message: true },
			mutedConversations: ['channel-a'],
			deviceKeys: await reachableKeys()
		});
		const asked = answerWith(201);

		const delivery = await notifyMember(client, 'member-1', 'message', notification, vapid, 1_700_000_000, 'channel-a');

		expect(delivery).toEqual({ reached: 0, pruned: 0, silent: true });
		expect(asked).toEqual([]);
	});

	test('does not silence the conversations beside it', async () => {
		const client = clientWhere({
			settings: { message: true },
			mutedConversations: ['channel-a'],
			deviceKeys: await reachableKeys()
		});
		answerWith(201);

		const delivery = await notifyMember(client, 'member-1', 'message', notification, vapid, 1_700_000_000, 'channel-b');

		expect(delivery.silent).toBe(false);
		expect(delivery.reached).toBe(1);
	});

	test('leaves a notification that names no conversation alone', async () => {
		const client = clientWhere({
			settings: { task: true },
			mutedConversations: ['channel-a'],
			deviceKeys: await reachableKeys()
		});
		answerWith(201);

		const delivery = await notifyMember(client, 'member-1', 'task', notification, vapid, 1_700_000_000);

		expect(delivery.silent).toBe(false);
		expect(delivery.reached).toBe(1);
	});

	test('is heard again once unmuted, though the row that recorded the mute remains', async () => {
		const client = clientWhere({
			settings: { message: true },
			mutedConversations: [],
			unmutedConversations: ['channel-a'],
			deviceKeys: await reachableKeys()
		});
		answerWith(201);

		const delivery = await notifyMember(client, 'member-1', 'message', notification, vapid, 1_700_000_000, 'channel-a');

		expect(delivery.silent).toBe(false);
		expect(delivery.reached).toBe(1);
	});

	test('is beside the category switch rather than instead of it', async () => {
		const client = clientWhere({
			settings: { message: false },
			mutedConversations: [],
			deviceKeys: await reachableKeys()
		});
		const asked = answerWith(201);

		const delivery = await notifyMember(client, 'member-1', 'message', notification, vapid, 1_700_000_000, 'channel-b');

		expect(delivery.silent).toBe(true);
		expect(asked).toEqual([]);
	});
});
