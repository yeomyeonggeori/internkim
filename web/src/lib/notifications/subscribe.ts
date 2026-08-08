import { supabase, vapidPublicKey } from '$lib/supabase';

export type Reachability = 'unsupported' | 'unconfigured' | 'blocked' | 'off' | 'on';

const webPush = 'web-push';

export function decodeBase64URL(encoded: string): Uint8Array<ArrayBuffer> {
	const padded = encoded.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(encoded.length / 4) * 4, '=');
	const binary = atob(padded);
	return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

export function encodeBase64URL(bytes: ArrayBufferLike | null): string {
	if (!bytes) return '';
	const binary = String.fromCharCode(...new Uint8Array(bytes));
	return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
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
	if (!vapidPublicKey()) return 'unconfigured';
	if (Notification.permission === 'denied') return 'blocked';
	return (await heldSubscription()) ? 'on' : 'off';
}

export async function startBeingReached(): Promise<Reachability> {
	if (!isSupported()) return 'unsupported';
	if (!vapidPublicKey()) return 'unconfigured';
	if ((await Notification.requestPermission()) !== 'granted') return 'blocked';

	const registration = await navigator.serviceWorker.ready;
	const subscription =
		(await registration.pushManager.getSubscription()) ??
		(await registration.pushManager.subscribe({
			userVisibleOnly: true,
			applicationServerKey: decodeBase64URL(vapidPublicKey())
		}));

	const { error } = await supabase().rpc('claim_push_device', {
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

	const { error } = await supabase().rpc('release_push_device', {
		device_kind: webPush,
		device_address: subscription.endpoint
	});
	if (error) throw new Error(error.message);
	await subscription.unsubscribe();
	return 'off';
}
