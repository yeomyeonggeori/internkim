import { afterAll, describe, expect, mock, test } from 'bun:test';

const published = 'BPublishedApplicationServerKey';

let carriedKey = '';
let lookups = 0;

const centralPlane = await import('../../src/lib/supabase');

mock.module('$lib/supabase', () => ({
	...centralPlane,
	vapidPublicKey: () => {
		lookups += 1;
		return carriedKey;
	}
}));

const registration = { pushManager: { getSubscription: () => Promise.resolve(null) } };
const heldNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');

Object.defineProperty(globalThis, 'navigator', {
	configurable: true,
	value: { serviceWorker: { ready: Promise.resolve(registration) } }
});
Object.defineProperty(globalThis, 'Notification', { configurable: true, value: { permission: 'granted' } });
Object.defineProperty(globalThis, 'window', {
	configurable: true,
	value: { PushManager: class {}, Notification: { permission: 'granted' } }
});

const { reachability } = await import('../../src/lib/notifications/subscribe');

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
