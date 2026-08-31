import { isSupabaseConfigured, supabase, vapidPublicKey } from '$lib/supabase';
import { decodeBase64URL, encodeBase64URL } from './base64url';

export type Reachability = 'unsupported' | 'unconfigured' | 'blocked' | 'off' | 'on';

const webPush = 'web-push';

let cachedServerKey: Promise<string> | undefined;

function applicationServerKey(): Promise<string> {
	cachedServerKey ??= resolveServerKey().then(
		(key) => {
			if (!key) cachedServerKey = undefined;
			return key;
		},
		(failure) => {
			cachedServerKey = undefined;
			throw failure;
		}
	);
	return cachedServerKey;
}

async function resolveServerKey(): Promise<string> {
	if (!isSupabaseConfigured()) return vapidPublicKey();
	const { data, error } = await supabase().rpc('vapid_public_key');
	if (!error && typeof data === 'string' && data !== '') return data;
	return vapidPublicKey();
}

function isSupported(): boolean {
	return (
		typeof navigator !== 'undefined' &&
		'serviceWorker' in navigator &&
		typeof window !== 'undefined' &&
		'PushManager' in window &&
		'Notification' in window
	);
}

async function heldSubscription(): Promise<PushSubscription | null> {
	const registration = await navigator.serviceWorker.ready;
	return registration.pushManager.getSubscription();
}

export function answersTo(subscription: PushSubscription, serverKey: string): boolean {
	const held = subscription.options?.applicationServerKey;
	if (!held) return true;
	return encodeBase64URL(held) === serverKey;
}

export async function reachability(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	const serverKey = await applicationServerKey();
	if (!serverKey) return 'unconfigured';
	if (Notification.permission === 'denied') return 'blocked';
	const held = await heldSubscription();
	return held && answersTo(held, serverKey) ? 'on' : 'off';
}

export async function startBeingReached(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	const serverKey = await applicationServerKey();
	if (!serverKey) return 'unconfigured';
	if ((await Notification.requestPermission()) !== 'granted') return 'blocked';

	const registration = await navigator.serviceWorker.ready;
	const held = await registration.pushManager.getSubscription();
	if (held && !answersTo(held, serverKey)) await held.unsubscribe();
	const subscription =
		(held && answersTo(held, serverKey) ? held : null) ??
		(await registration.pushManager.subscribe({
			userVisibleOnly: true,
			applicationServerKey: decodeBase64URL(serverKey)
		}));

	const { error } = await supabase().rpc('push_device_claim', {
		device_kind: webPush,
		device_address: subscription.endpoint,
		device_keys: {
			p256dh: encodeBase64URL(subscription.getKey('p256dh')),
			auth: encodeBase64URL(subscription.getKey('auth'))
		}
	});
	if (error) throw new Error(error.message);
	return 'on';
}

export async function stopBeingReached(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	const subscription = await heldSubscription();
	if (!subscription) return 'off';

	const { error } = await supabase().rpc('push_device_release', {
		device_kind: webPush,
		device_address: subscription.endpoint
	});
	if (error) throw new Error(error.message);
	await subscription.unsubscribe();
	return 'off';
}
