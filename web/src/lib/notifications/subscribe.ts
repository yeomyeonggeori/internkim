import { isSupabaseConfigured, supabase, vapidPublicKey } from '$lib/supabase';
import { decodeBase64URL, encodeBase64URL } from './base64url';

export type Reachability = 'unsupported' | 'unconfigured' | 'blocked' | 'off' | 'on';

const webPush = 'web-push';

type ServerKey = { key: string; vaulted: boolean };

let cachedServerKey: Promise<ServerKey> | undefined;

function applicationServerKey(): Promise<ServerKey> {
	cachedServerKey ??= resolveServerKey().then(
		(resolved) => {
			if (!resolved.key) cachedServerKey = undefined;
			return resolved;
		},
		(failure) => {
			cachedServerKey = undefined;
			throw failure;
		}
	);
	return cachedServerKey;
}

async function resolveServerKey(): Promise<ServerKey> {
	if (!isSupabaseConfigured()) return { key: vapidPublicKey(), vaulted: false };
	const { data, error } = await supabase().rpc('vapid_public_key');
	if (!error && typeof data === 'string' && data !== '') return { key: data, vaulted: true };
	return { key: vapidPublicKey(), vaulted: false };
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

export function stale(subscription: PushSubscription | null, serverKey: ServerKey): boolean {
	if (!subscription) return false;
	return serverKey.vaulted && !answersTo(subscription, serverKey.key);
}

export async function reachability(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	const serverKey = await applicationServerKey();
	if (!serverKey.key) return 'unconfigured';
	if (Notification.permission === 'denied') return 'blocked';
	const held = await heldSubscription();
	return held && !stale(held, serverKey) ? 'on' : 'off';
}

export async function startBeingReached(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	const serverKey = await applicationServerKey();
	if (!serverKey.key) return 'unconfigured';
	if ((await Notification.requestPermission()) !== 'granted') return 'blocked';

	const registration = await navigator.serviceWorker.ready;
	const held = await registration.pushManager.getSubscription();
	if (stale(held, serverKey)) await held?.unsubscribe();
	const subscription =
		(stale(held, serverKey) ? null : held) ??
		(await registration.pushManager.subscribe({
			userVisibleOnly: true,
			applicationServerKey: decodeBase64URL(serverKey.key)
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
