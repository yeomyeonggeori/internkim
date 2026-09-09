import { afterEach, describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { pushToMemberDevices } from '../../../supabase/functions/_shared/push-to-member-devices.ts';
import { forgetApnsAuthorization } from '../../../supabase/functions/_shared/apns.ts';
import { forgetFcmAccessToken } from '../../../supabase/functions/_shared/fcm.ts';
import type { PushKeys } from '../../../supabase/functions/_shared/push-keys.ts';
import { encodeBase64URL } from '../../src/lib/notifications/base64url';

const vapid = {
	publicKey: 'BG0w6CuCogoJKa593BzjeAk_VAOmSYtz4Crk7OBQPEYa3_peOcMJEln_GG6LyW-0nl82LPHDClzU8_0nB4Z5dcs',
	privateKey: 'NNK7ZJuRBBHnpKs9X0R0aM4Tff6BaVUfPwmnYTdPuWA',
	subject: 'mailto:support@example.com'
};

const pushKeys: PushKeys = { vapid, apns: null, fcm: null };

const notification = { title: 'internkim', body: '', openPath: '/settings', tag: 'notification-self-test' };

async function reachableKeys() {
	const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
	return {
		p256dh: encodeBase64URL(await crypto.subtle.exportKey('raw', pair.publicKey)),
		auth: encodeBase64URL(crypto.getRandomValues(new Uint8Array(16)).buffer)
	};
}

async function apnsKeys() {
	const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, ['sign', 'verify']);
	const pkcs8 = await crypto.subtle.exportKey('pkcs8', pair.privateKey);
	return {
		keyID: 'ABC123DEFG',
		teamID: 'HIJ456KLMN',
		bundleID: 'kim.intern.app',
		privateKey: `-----BEGIN PRIVATE KEY-----\n${asPEMBody(pkcs8)}\n-----END PRIVATE KEY-----`,
		environment: 'production' as const
	};
}

async function fcmKeys() {
	const pair = await crypto.subtle.generateKey(
		{ name: 'RSASSA-PKCS1-v1_5', modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: 'SHA-256' },
		true,
		['sign', 'verify']
	);
	const pkcs8 = await crypto.subtle.exportKey('pkcs8', pair.privateKey);
	return {
		projectID: 'example-project',
		clientEmail: 'sender@example-project.iam.gserviceaccount.com',
		privateKey: `-----BEGIN PRIVATE KEY-----\n${asPEMBody(pkcs8)}\n-----END PRIVATE KEY-----`
	};
}

function asPEMBody(pkcs8: ArrayBuffer): string {
	return btoa(String.fromCharCode(...new Uint8Array(pkcs8))).replace(/(.{64})/g, '$1\n');
}

type DeviceRow = { kind: string; address: string; keys: unknown };

function clientHolding(rows: DeviceRow[]) {
	const deleted: string[] = [];
	const client = {
		from() {
			return {
				select: () => ({
					eq: () => ({ returns: async () => ({ data: rows, error: null }) })
				}),
				delete: () => ({
					eq: (_kindColumn: string, kind: string) => ({
						eq: async (_addressColumn: string, address: string) => {
							deleted.push(`${kind}:${address}`);
							return { error: null };
						}
					})
				})
			};
		}
	} as unknown as SupabaseClient;
	return { client, deleted };
}

const realFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = realFetch;
	forgetApnsAuthorization();
	forgetFcmAccessToken();
});

function answerWith(status: number, body: unknown = null) {
	const asked: string[] = [];
	globalThis.fetch = Object.assign(
		async (input: RequestInfo | URL) => {
			asked.push(String(input));
			return new Response(body === null ? null : JSON.stringify(body), {
				status,
				headers: body === null ? undefined : { 'content-type': 'application/json' }
			});
		},
		{ preconnect: realFetch.preconnect }
	);
	return asked;
}

function answerByHost(answers: Record<string, { status: number; body?: unknown }>) {
	const asked: string[] = [];
	globalThis.fetch = Object.assign(
		async (input: RequestInfo | URL) => {
			const address = String(input);
			asked.push(address);
			const matched = Object.entries(answers).find(([host]) => address.includes(host));
			const answer = matched?.[1] ?? { status: 500 };
			return new Response(answer.body === undefined ? null : JSON.stringify(answer.body), {
				status: answer.status,
				headers: answer.body === undefined ? undefined : { 'content-type': 'application/json' }
			});
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

		expect(await pushToMemberDevices(client, 'member-1', notification, pushKeys, 1_700_000_000)).toEqual({
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

		expect(await pushToMemberDevices(client, 'member-1', notification, pushKeys, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 1
		});
		expect(deleted).toEqual(['web-push:https://push.example.com/gone']);
	});

	test('a row whose keys are not a pair of strings never reaches the network', async () => {
		const { client, deleted } = clientHolding([
			{ kind: 'web-push', address: 'https://push.example.com/shapeless', keys: { p256dh: 42 } },
			{ kind: 'web-push', address: 'https://push.example.com/empty', keys: null }
		]);
		const asked = answerWith(201);

		expect(await pushToMemberDevices(client, 'member-1', notification, pushKeys, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 0
		});
		expect(asked).toEqual([]);
		expect(deleted).toEqual([]);
	});

	test('a member with no device is reached zero times rather than failing', async () => {
		const { client } = clientHolding([]);
		const asked = answerWith(201);

		expect(await pushToMemberDevices(client, 'member-1', notification, pushKeys, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 0
		});
		expect(asked).toEqual([]);
	});

	test('each kind of device is reached the way that kind is reached', async () => {
		const { client } = clientHolding([
			{ kind: 'web-push', address: 'https://push.example.com/one', keys: await reachableKeys() },
			{ kind: 'apns', address: 'apns-device-token', keys: {} },
			{ kind: 'fcm', address: 'fcm-device-token', keys: {} }
		]);
		const asked = answerByHost({
			'push.example.com': { status: 201 },
			'api.push.apple.com': { status: 200 },
			'oauth2.googleapis.com': { status: 200, body: { access_token: 'google-token' } },
			'fcm.googleapis.com': { status: 200, body: { name: 'projects/example-project/messages/1' } }
		});

		const keys: PushKeys = { vapid, apns: await apnsKeys(), fcm: await fcmKeys() };
		expect(await pushToMemberDevices(client, 'member-1', notification, keys, 1_700_000_000)).toEqual({
			reached: 3,
			pruned: 0
		});
		expect(asked).toEqual([
			'https://push.example.com/one',
			'https://api.push.apple.com/3/device/apns-device-token',
			'https://oauth2.googleapis.com/token',
			'https://fcm.googleapis.com/v1/projects/example-project/messages:send'
		]);
	});

	test('a device of a kind this deployment holds no key for is left alone', async () => {
		const { client, deleted } = clientHolding([
			{ kind: 'apns', address: 'apns-device-token', keys: {} },
			{ kind: 'fcm', address: 'fcm-device-token', keys: {} }
		]);
		const asked = answerWith(200);

		expect(await pushToMemberDevices(client, 'member-1', notification, pushKeys, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 0
		});
		expect(asked).toEqual([]);
		expect(deleted).toEqual([]);
	});

	test('an apple token the device no longer answers for is dropped', async () => {
		const { client, deleted } = clientHolding([{ kind: 'apns', address: 'stale-token', keys: {} }]);
		answerByHost({ 'api.push.apple.com': { status: 410, body: { reason: 'Unregistered' } } });

		const keys: PushKeys = { vapid: null, apns: await apnsKeys(), fcm: null };
		expect(await pushToMemberDevices(client, 'member-1', notification, keys, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 1
		});
		expect(deleted).toEqual(['apns:stale-token']);
	});

	test('a google token the app no longer holds is dropped', async () => {
		const { client, deleted } = clientHolding([{ kind: 'fcm', address: 'stale-token', keys: {} }]);
		answerByHost({
			'oauth2.googleapis.com': { status: 200, body: { access_token: 'google-token' } },
			'fcm.googleapis.com': {
				status: 404,
				body: { error: { status: 'NOT_FOUND', details: [{ errorCode: 'UNREGISTERED' }] } }
			}
		});

		const keys: PushKeys = { vapid: null, apns: null, fcm: await fcmKeys() };
		expect(await pushToMemberDevices(client, 'member-1', notification, keys, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 1
		});
		expect(deleted).toEqual(['fcm:stale-token']);
	});

	test('a google refusal that is not about the token keeps the device', async () => {
		const { client, deleted } = clientHolding([{ kind: 'fcm', address: 'live-token', keys: {} }]);
		answerByHost({
			'oauth2.googleapis.com': { status: 200, body: { access_token: 'google-token' } },
			'fcm.googleapis.com': { status: 503, body: { error: { status: 'UNAVAILABLE' } } }
		});

		const keys: PushKeys = { vapid: null, apns: null, fcm: await fcmKeys() };
		expect(await pushToMemberDevices(client, 'member-1', notification, keys, 1_700_000_000)).toEqual({
			reached: 0,
			pruned: 0
		});
		expect(deleted).toEqual([]);
	});
});
