import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { sendApns, forgetApnsAuthorization, type ApnsKey } from '../../../supabase/functions/_shared/apns.ts';
import { sendWebPush } from '../../../supabase/functions/_shared/web-push.ts';
import { shortAddress } from '../../../supabase/functions/_shared/push-diagnostics.ts';
import { aBrowserThatSubscribed } from '../support/read-as-the-browser-would';
import { aVapidPair } from '../support/a-vapid-pair';

async function aSigningKeyPEM(): Promise<string> {
	const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, ['sign', 'verify']);
	const pkcs8 = new Uint8Array(await crypto.subtle.exportKey('pkcs8', pair.privateKey));
	const base64 = btoa(String.fromCharCode(...pkcs8));
	const lines = base64.match(/.{1,64}/g) ?? [];
	return ['-----BEGIN PRIVATE KEY-----', ...lines, '-----END PRIVATE KEY-----'].join('\n');
}

const apnsKey: ApnsKey = {
	keyID: 'TESTKEY123',
	teamID: 'TESTTEAM45',
	bundleID: 'app.example.test',
	privateKey: await aSigningKeyPEM(),
	environment: 'production'
};

const notification = { title: 'a title', body: 'a body', openPath: '/', tag: 'a-tag' };
const deviceToken = 'A11CE5A11P1E'.repeat(5) + 'ABCD';
const subscriptionAddress = 'https://web.push.apple.com/a-subscription';

type Said = { level: 'warn' | 'error'; line: Record<string, unknown> };

const said: Said[] = [];
let realFetch: typeof fetch;
let realWarn: typeof console.warn;
let realError: typeof console.error;

beforeEach(() => {
	said.length = 0;
	realFetch = globalThis.fetch;
	realWarn = console.warn;
	realError = console.error;
	console.warn = (line: unknown) => void said.push({ level: 'warn', line: JSON.parse(String(line)) });
	console.error = (line: unknown) => void said.push({ level: 'error', line: JSON.parse(String(line)) });
	forgetApnsAuthorization();
});

afterEach(() => {
	globalThis.fetch = realFetch;
	console.warn = realWarn;
	console.error = realError;
});

function answerWith(answer: () => Response): void {
	globalThis.fetch = Object.assign(async () => answer(), { preconnect: realFetch.preconnect });
}

async function aSubscription(): Promise<{ address: string; keys: { p256dh: string; auth: string } }> {
	const subscriber = await aBrowserThatSubscribed();
	return { address: subscriptionAddress, keys: subscriber.keys };
}

describe('a push that was not delivered says why', () => {
	test('an apns refusal is an error naming the status and the reason', async () => {
		answerWith(() => new Response(JSON.stringify({ reason: 'TopicDisallowed' }), { status: 403 }));

		expect(await sendApns(deviceToken, notification, apnsKey, 1_700_000_000)).toBe('refused');

		expect(said).toHaveLength(1);
		expect(said[0].level).toBe('error');
		expect(said[0].line).toMatchObject({
			event: 'push.not-delivered',
			channel: 'apns',
			stage: 'answer',
			outcome: 'refused',
			status: 403,
			reason: 'TopicDisallowed'
		});
	});

	test('an apns request that throws names the failure', async () => {
		answerWith(() => {
			throw new Error('Malformed_HTTP_Response');
		});

		expect(await sendApns(deviceToken, notification, apnsKey, 1_700_000_000)).toBe('refused');

		expect(said[0].level).toBe('error');
		expect(said[0].line).toMatchObject({ channel: 'apns', stage: 'request', outcome: 'refused' });
		expect(String(said[0].line.failure)).toContain('Malformed_HTTP_Response');
	});

	test('a device the push service has forgotten is a warning, not an error', async () => {
		answerWith(() => new Response(JSON.stringify({ reason: 'Unregistered' }), { status: 410 }));

		expect(await sendApns(deviceToken, notification, apnsKey, 1_700_000_000)).toBe('gone');

		expect(said).toHaveLength(1);
		expect(said[0].level).toBe('warn');
		expect(said[0].line).toMatchObject({ channel: 'apns', outcome: 'gone', status: 410 });
	});

	test('a delivered push says nothing', async () => {
		answerWith(() => new Response(null, { status: 200 }));

		expect(await sendApns(deviceToken, notification, apnsKey, 1_700_000_000)).toBe('delivered');
		expect(said).toEqual([]);
	});

	test('a web-push refusal names the status', async () => {
		const target = await aSubscription();
		answerWith(() => new Response(null, { status: 502 }));

		expect(await sendWebPush(target, notification, aVapidPair, 1_700_000_000)).toBe('refused');

		expect(said[0].level).toBe('error');
		expect(said[0].line).toMatchObject({ channel: 'web-push', stage: 'answer', outcome: 'refused', status: 502 });
	});

	test('a subscription the record cannot use says so instead of vanishing', async () => {
		const unusable = { address: subscriptionAddress, keys: { p256dh: 'not-a-key', auth: 'not-a-secret' } };

		expect(await sendWebPush(unusable, notification, aVapidPair, 1_700_000_000)).toBe('gone');

		expect(said).toHaveLength(1);
		expect(said[0].level).toBe('warn');
		expect(said[0].line).toMatchObject({ channel: 'web-push', stage: 'subscription', outcome: 'gone' });
	});

	test('a vapid pair that cannot sign refuses this device rather than the whole run', async () => {
		const target = await aSubscription();
		const broken = { ...aVapidPair, privateKey: 'not-a-private-key' };

		expect(await sendWebPush(target, notification, broken, 1_700_000_000)).toBe('refused');

		expect(said[0].level).toBe('error');
		expect(said[0].line).toMatchObject({ channel: 'web-push', stage: 'authorization', outcome: 'refused' });
	});
});

describe('a diagnostic never carries the whole address', () => {
	test('a device token is cut to its first characters', () => {
		expect(shortAddress(deviceToken)).toBe(`${deviceToken.slice(0, 8)}… (${deviceToken.length} chars)`);
		expect(shortAddress(deviceToken)).not.toContain(deviceToken.slice(8));
	});

	test('a subscription URL is reduced to its host', () => {
		expect(shortAddress(subscriptionAddress)).toBe('web.push.apple.com');
	});

	test('an empty address says so', () => {
		expect(shortAddress('')).toBe('(empty)');
	});
});
