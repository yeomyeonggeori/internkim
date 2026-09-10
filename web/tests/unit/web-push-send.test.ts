import { afterEach, describe, expect, test } from 'bun:test';
import { outcomeOfStatus, sendWebPush } from '../../../supabase/functions/_shared/web-push.ts';
import { encodeBase64URL } from '../../src/lib/notifications/base64url';
import { aVapidPair } from '../support/a-vapid-pair';

const keys = aVapidPair;

const subscriberPair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
const target = {
	address: 'https://fcm.googleapis.com/fcm/send/abc123',
	keys: {
		p256dh: encodeBase64URL(await crypto.subtle.exportKey('raw', subscriberPair.publicKey)),
		auth: encodeBase64URL(crypto.getRandomValues(new Uint8Array(16)).buffer)
	}
};

const realFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = realFetch;
});

function answerWith(status: number) {
	const asked: { url: string; headers: Headers; body: unknown }[] = [];
	globalThis.fetch = Object.assign(
		async (input: RequestInfo | URL, options?: RequestInit) => {
			asked.push({
				url: String(input),
				headers: new Headers(options?.headers),
				body: options?.body
			});
			return new Response(null, { status });
		},
		{ preconnect: realFetch.preconnect }
	);
	return asked;
}

describe('outcomeOfStatus', () => {
	test('a push service that has forgotten the browser says so, and the row must go', () => {
		expect(outcomeOfStatus(404)).toBe('gone');
		expect(outcomeOfStatus(410)).toBe('gone');
	});

	test('anything else that is not a success is kept and retried another day', () => {
		expect(outcomeOfStatus(429)).toBe('refused');
		expect(outcomeOfStatus(500)).toBe('refused');
		expect(outcomeOfStatus(401)).toBe('refused');
		expect(outcomeOfStatus(413)).toBe('refused');
	});

	test('the push services answer 201, not 200', () => {
		expect(outcomeOfStatus(201)).toBe('delivered');
		expect(outcomeOfStatus(200)).toBe('delivered');
		expect(outcomeOfStatus(202)).toBe('delivered');
	});
});

describe('sendWebPush', () => {
	test('it posts the sealed body to the address the browser gave', async () => {
		const asked = answerWith(201);

		expect(await sendWebPush(target, { title: '이샘플' }, keys, 1_786_000_000)).toBe('delivered');
		expect(asked).toHaveLength(1);
		expect(asked[0].url).toBe(target.address);
		expect(asked[0].body).toBeInstanceOf(Uint8Array);
	});

	test('the headers are the ones a push service refuses without', async () => {
		const asked = answerWith(201);

		await sendWebPush(target, { title: '이샘플' }, keys, 1_786_000_000);

		expect(asked[0].headers.get('content-encoding')).toBe('aes128gcm');
		expect(asked[0].headers.get('content-type')).toBe('application/octet-stream');
		expect(asked[0].headers.get('ttl')).toBe('86400');
		expect(asked[0].headers.get('authorization')?.startsWith('vapid t=')).toBe(true);
	});

	test('nothing readable leaves this process', async () => {
		const asked = answerWith(201);

		await sendWebPush(target, { title: '이샘플', body: '연봉 협상 건' }, keys, 1_786_000_000);

		const sent = new TextDecoder().decode(asked[0].body as Uint8Array);
		expect(sent.includes('연봉')).toBe(false);
		expect(sent.includes('이샘플')).toBe(false);
	});

	test('a browser the push service has forgotten is reported as gone', async () => {
		answerWith(410);

		expect(await sendWebPush(target, { title: 'x' }, keys, 1_786_000_000)).toBe('gone');
	});

	test('a key that could never work is gone rather than an exception that silences the other devices', async () => {
		const asked = answerWith(201);
		const corrupt = { address: target.address, keys: { p256dh: 'AQID', auth: target.keys.auth } };

		expect(await sendWebPush(corrupt, { title: 'x' }, keys, 1_786_000_000)).toBe('gone');
		expect(asked).toHaveLength(0);
	});
});
