import { describe, expect, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { pushDeviceAddressOf, pushDeviceClaimOf } from '../../../src/lib/server/push-device';

const subscription = 'https://push.example.test/subscription-1';

function refusalOf(read: () => unknown): { status: number; message: string } {
	try {
		read();
	} catch (thrown) {
		if (isHttpError(thrown)) return { status: thrown.status, message: thrown.body.message };
		throw thrown;
	}
	throw new Error('the body was read without a refusal');
}

describe('a claim the app carries', () => {
	test('is a browser subscription with its keys when it names no kind', () => {
		expect(
			pushDeviceClaimOf({ endpoint: subscription, publicKey: 'a-public-key', authenticationSecret: 'a-secret' })
		).toEqual({
			kind: 'web-push',
			endpoint: subscription,
			encryptionKeys: { p256dh: 'a-public-key', auth: 'a-secret' }
		});
	});

	test('is refused when a browser subscription carries no keys to encrypt to', () => {
		const refused = refusalOf(() => pushDeviceClaimOf({ endpoint: subscription }));

		expect(refused.status).toBe(400);
		expect(refused.message).toContain('keys');
	});

	test('is an app token that carries no keys', () => {
		expect(pushDeviceClaimOf({ endpoint: 'a-device-token', kind: 'apns' })).toEqual({
			kind: 'apns',
			endpoint: 'a-device-token',
			encryptionKeys: null
		});
	});

	test('is refused when it names a kind the record does not store', () => {
		const refused = refusalOf(() => pushDeviceClaimOf({ endpoint: 'a-device-token', kind: 'pager' }));

		expect(refused.status).toBe(400);
		expect(refused.message).toContain('web-push, apns, fcm');
	});

	test('is refused when it is not a json object', () => {
		expect(refusalOf(() => pushDeviceClaimOf(null)).status).toBe(400);
		expect(refusalOf(() => pushDeviceClaimOf([subscription])).status).toBe(400);
	});
});

describe('a release the app asks for', () => {
	test('names the device by its address and kind', () => {
		expect(pushDeviceAddressOf({ endpoint: 'a-device-token', kind: 'fcm' })).toEqual({
			kind: 'fcm',
			endpoint: 'a-device-token'
		});
	});

	test('is refused when it names no device', () => {
		const refused = refusalOf(() => pushDeviceAddressOf({ kind: 'apns' }));

		expect(refused.status).toBe(400);
		expect(refused.message).toContain('subscription');
	});
});
