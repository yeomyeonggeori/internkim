import type { SupabaseClient } from '@supabase/supabase-js';
import type { Environment } from './agent-request';
import type { VapidKeys } from './web-push-vapid';

export async function vapidKeysInUse(
	record: SupabaseClient,
	environment: Environment
): Promise<VapidKeys | null> {
	return (await vaultedKeys(record)) ?? environmentKeys(environment);
}

async function vaultedKeys(record: SupabaseClient): Promise<VapidKeys | null> {
	const { data, error } = await record.rpc('vapid_keys_read');
	if (error) return null;
	const kept = data as { publicKey?: unknown; privateKey?: unknown; subject?: unknown } | null;
	if (!kept) return null;
	return usable(kept.publicKey, kept.privateKey, kept.subject);
}

function environmentKeys(environment: Environment): VapidKeys | null {
	return usable(environment.VAPID_PUBLIC_KEY, environment.VAPID_PRIVATE_KEY, environment.VAPID_SUBJECT);
}

function usable(publicKey: unknown, privateKey: unknown, subject: unknown): VapidKeys | null {
	if (typeof publicKey !== 'string' || publicKey === '') return null;
	if (typeof privateKey !== 'string' || privateKey === '') return null;
	if (typeof subject !== 'string' || subject === '') return null;
	return { publicKey, privateKey, subject };
}
