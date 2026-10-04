import { afterAll, describe, expect, mock, test } from 'bun:test';

const published = 'BPublishedApplicationServerKey';

let carriedKey = '';
let lookups = 0;

const centralPlane = { ...(await import('../../src/lib/supabase')) };

mock.module('$lib/supabase', () => ({
	...centralPlane,
	vapidPublicKey: () => {
		lookups += 1;
		return carriedKey;
	}
}));

function subscriptionUnder(key: string | null): PushSubscription {
	return {
		options: key === null ? {} : { applicationServerKey: new TextEncoder().encode(key).buffer }
	} as unknown as PushSubscription;
}

let heldSubscription: PushSubscription | null = null;
const registration = { pushManager: { getSubscription: () => Promise.resolve(heldSubscription) } };
const heldNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');

Object.defineProperty(globalThis, 'navigator', {
	configurable: true,
	value: { serviceWorker: { ready: new Promise(() => {}), getRegistration: async () => registration } }
});
Object.defineProperty(globalThis, 'Notification', { configurable: true, value: { permission: 'granted' } });
Object.defineProperty(globalThis, 'window', {
	configurable: true,
	value: { PushManager: class {}, Notification: { permission: 'granted' } }
});

const { answersTo, reachability, stale } = await import('../../src/lib/notifications/subscribe');
const { encodeBase64URL } = await import('../../src/lib/notifications/base64url');

afterAll(() => {
	mock.module('$lib/supabase', () => centralPlane);
	Reflect.deleteProperty(globalThis, 'window');
	Reflect.deleteProperty(globalThis, 'Notification');
	if (heldNavigator) Object.defineProperty(globalThis, 'navigator', heldNavigator);
	else Reflect.deleteProperty(globalThis, 'navigator');
});

describe('the application server key', () => {
	test('a lookup that answered nothing is made again, and a key that answered is kept', async () => {
		carriedKey = '';

		expect(await reachability()).toBe('unconfigured');
		expect(lookups).toBe(1);

		expect(await reachability()).toBe('unconfigured');
		expect(lookups).toBe(2);

		carriedKey = published;

		expect(await reachability()).toBe('off');
		expect(lookups).toBe(3);

		expect(await reachability()).toBe('off');
		expect(lookups).toBe(3);
	});
});

describe('a subscription made under another key does not count as reachable', () => {
	const current = 'BJnB-current-key';
	const other = 'BJnB-other-key';

	const encoded = (key: string) => encodeBase64URL(new TextEncoder().encode(key).buffer);

	test('the key it was made with is the one that counts', () => {
		expect(answersTo(subscriptionUnder(current), encoded(current))).toBe(true);
		expect(answersTo(subscriptionUnder(other), encoded(current))).toBe(false);
	});

	test('a browser that will not say which key it used is taken at its word', () => {
		expect(answersTo(subscriptionUnder(null), encoded(current))).toBe(true);
	});

	test('only a key the vault answered condemns a subscription', () => {
		const held = subscriptionUnder(other);
		expect(stale(held, { key: encoded(current), vaulted: true })).toBe(true);
		expect(stale(held, { key: encoded(current), vaulted: false })).toBe(false);
		expect(stale(subscriptionUnder(current), { key: encoded(current), vaulted: true })).toBe(false);
		expect(stale(null, { key: encoded(current), vaulted: true })).toBe(false);
	});
});
