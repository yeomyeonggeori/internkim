import type { SupabaseClient } from './service-client.ts';
import type { VapidKeys } from './web-push-vapid.ts';

export async function vapidKeysFromVault(client: SupabaseClient): Promise<VapidKeys | null> {
	const { data, error } = await client.rpc('vapid_keys_read');
	if (error) return null;
	const keys = data as { publicKey?: unknown; privateKey?: unknown; subject?: unknown } | null;
	if (!keys) return null;
	if (typeof keys.publicKey !== 'string' || keys.publicKey === '') return null;
	if (typeof keys.privateKey !== 'string' || keys.privateKey === '') return null;
	if (typeof keys.subject !== 'string' || keys.subject === '') return null;
	return { publicKey: keys.publicKey, privateKey: keys.privateKey, subject: keys.subject };
}
