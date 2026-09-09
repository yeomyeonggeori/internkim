import { describe, expect, test } from 'bun:test';
import { pushKeysFromVault, pushKeysOf, reachesSomeDevice } from '../../../supabase/functions/_shared/push-keys.ts';
import type { ApnsKey } from '../../../supabase/functions/_shared/apns.ts';
import type { SupabaseClient } from '../../../supabase/functions/_shared/service-client.ts';

const vapid = { publicKey: 'vault-public', privateKey: 'vault-private', subject: 'mailto:vault@example.com' };
const apns: ApnsKey = {
	keyID: 'ABC123DEFG',
	teamID: 'HIJ456KLMN',
	bundleID: 'kim.intern.app',
	privateKey: '-----BEGIN PRIVATE KEY-----\nMIG\n-----END PRIVATE KEY-----',
	environment: 'production'
};
const fcm = {
	projectID: 'example-project',
	clientEmail: 'sender@example-project.iam.gserviceaccount.com',
	privateKey: '-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----'
};

function answering(answer: { data?: unknown; error?: { message: string } | null }): SupabaseClient {
	return {
		rpc: async () => ({ data: answer.data ?? null, error: answer.error ?? null })
	} as unknown as SupabaseClient;
}

describe('the keys a deployment sends pushes with', () => {
	test('come from the vault, each kind on its own', async () => {
		expect(await pushKeysFromVault(answering({ data: { vapid, apns, fcm } }))).toEqual({ vapid, apns, fcm });
	});

	test('are nothing when the vault refuses the read', async () => {
		expect(await pushKeysFromVault(answering({ error: { message: 'permission denied' } }))).toEqual({
			vapid: null,
			apns: null,
			fcm: null
		});
	});

	test('are nothing at all when the vault holds none', async () => {
		expect(await pushKeysFromVault(answering({ data: null }))).toEqual({ vapid: null, apns: null, fcm: null });
	});

	test('one kind missing leaves the others usable', () => {
		expect(pushKeysOf({ vapid, apns: null, fcm })).toEqual({ vapid, apns: null, fcm });
	});

	test('a half-filled kind is nothing rather than half a key', () => {
		expect(pushKeysOf({ vapid: { ...vapid, privateKey: '' } }).vapid).toBeNull();
		expect(pushKeysOf({ apns: { ...apns, bundleID: null } }).apns).toBeNull();
		expect(pushKeysOf({ fcm: { ...fcm, clientEmail: '   ' } }).fcm).toBeNull();
	});

	test('an APNs key names production unless it says sandbox', () => {
		expect(pushKeysOf({ apns: { ...apns, environment: 'sandbox' } }).apns?.environment).toBe('sandbox');
		expect(pushKeysOf({ apns: { ...apns, environment: null } }).apns?.environment).toBe('production');
	});

	test('a deployment reaches nobody only when it holds no key at all', () => {
		expect(reachesSomeDevice({ vapid: null, apns: null, fcm: null })).toBe(false);
		expect(reachesSomeDevice({ vapid: null, apns, fcm: null })).toBe(true);
		expect(reachesSomeDevice({ vapid, apns: null, fcm: null })).toBe(true);
	});
});
