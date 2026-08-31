import { isSupabaseConfigured, supabase, vapidPublicKey } from '$lib/supabase';
import { decodeBase64URL, encodeBase64URL } from './base64url';

export type Reachability = 'unsupported' | 'unconfigured' | 'blocked' | 'off' | 'on';

const webPush = 'web-push';

let cachedServerKey: Promise<string> | undefined;

function applicationServerKey(): Promise<string> {
	cachedServerKey ??= resolveServerKey();
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

export async function reachability(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	if (!(await applicationServerKey())) return 'unconfigured';
	if (Notification.permission === 'denied') return 'blocked';
	return (await heldSubscription()) ? 'on' : 'off';
}

export async function startBeingReached(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	const serverKey = await applicationServerKey();
	if (!serverKey) return 'unconfigured';
	if ((await Notification.requestPermission()) !== 'granted') return 'blocked';

	const registration = await navigator.serviceWorker.ready;
	const subscription =
		(await registration.pushManager.getSubscription()) ??
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
