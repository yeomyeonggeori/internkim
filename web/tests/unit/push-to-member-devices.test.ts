import { afterEach, describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { pushToMemberDevices } from '../../../supabase/functions/_shared/push-to-member-devices.ts';
import { encodeBase64URL } from '../../src/lib/notifications/base64url';

const vapid = {
	publicKey: 'BG0w6CuCogoJKa593BzjeAk_VAOmSYtz4Crk7OBQPEYa3_peOcMJEln_GG6LyW-0nl82LPHDClzU8_0nB4Z5dcs',
	privateKey: 'NNK7ZJuRBBHnpKs9X0R0aM4Tff6BaVUfPwmnYTdPuWA',
	subject: 'mailto:support@example.com'
};

const notification = { title: 'internkim', body: '', openPath: '/settings', tag: 'notification-self-test' };

async function reachableKeys() {
	const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
	return {
		p256dh: encodeBase64URL(await crypto.subtle.exportKey('raw', pair.publicKey)),
		auth: encodeBase64URL(crypto.getRandomValues(new Uint8Array(16)).buffer)
	};
}

type DeviceRow = { kind: string; address: string; keys: unknown };

function clientHolding(rows: DeviceRow[]) {
	const deleted: string[] = [];
	const client = {
		from() {
			return {
				select: () => ({
					eq: () => ({
						eq: () => ({ returns: async () => ({ data: rows, error: null }) })
					})
				}),
				delete: () => ({
					eq: async (_column: string, address: string) => {
						deleted.push(address);
						return { error: null };
					}
				})
			};
		}
	} as unknown as SupabaseClient;
	return { client, deleted };
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

describe('pushing to the devices a member reads on', () => {
	test('every reachable device counts once', async () => {
		const keys = await reachableKeys();
		const { client } = clientHolding([
			{ kind: 'web-push', address: 'https://push.example.com/one', keys },
			{ kind: 'web-push', address: 'https://push.example.com/two', keys: await reachableKeys() }
		]);
		const asked = answerWith(201);

		expect(await pushToMemberDevices(client, 'member-1', notification, vapid, 1_700_000_000)).toEqual({
			reached: 2,
			pruned: 0
		});
		expect(asked).toEqual(['https://push.example.com/one', 'https://push.example.com/two']);
	});

	test('a device the push service has forgotten is dropped from the record', async () => {
		const { client, deleted } = clientHolding([
			{ kind: 'web-push', address: 'https://push.example.com/gone', keys: await reachableKeys() }
		]);
		answerWith(410);

		expect(await pushToMemberDevices(client, 'member-1', notification, vapid, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 1
		});
		expect(deleted).toEqual(['https://push.example.com/gone']);
	});

	test('a row whose keys are not a pair of strings never reaches the network', async () => {
		const { client, deleted } = clientHolding([
			{ kind: 'web-push', address: 'https://push.example.com/shapeless', keys: { p256dh: 42 } },
			{ kind: 'web-push', address: 'https://push.example.com/empty', keys: null }
		]);
		const asked = answerWith(201);

		expect(await pushToMemberDevices(client, 'member-1', notification, vapid, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 0
		});
		expect(asked).toEqual([]);
		expect(deleted).toEqual([]);
	});

	test('a member with no device is reached zero times rather than failing', async () => {
		const { client } = clientHolding([]);
		const asked = answerWith(201);

		expect(await pushToMemberDevices(client, 'member-1', notification, vapid, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 0
		});
		expect(asked).toEqual([]);
	});
});
