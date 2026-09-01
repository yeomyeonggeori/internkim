import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { vapidKeysInUse } from '../../../src/lib/server/vapid-keys';

const environment = {
	VAPID_PUBLIC_KEY: 'env-public',
	VAPID_PRIVATE_KEY: 'env-private',
	VAPID_SUBJECT: 'mailto:env@example.com'
};

function answering(answer: { data?: unknown; error?: { message: string } | null }): SupabaseClient {
	return {
		rpc: async () => ({ data: answer.data ?? null, error: answer.error ?? null })
	} as unknown as SupabaseClient;
}

describe('the keys a deployment signs pushes with', () => {
	test('come from the vault when it holds all three', async () => {
		const record = answering({
			data: { publicKey: 'vault-public', privateKey: 'vault-private', subject: 'mailto:vault@example.com' }
		});
		expect(await vapidKeysInUse(record, environment)).toEqual({
			publicKey: 'vault-public',
			privateKey: 'vault-private',
			subject: 'mailto:vault@example.com'
		});
	});

	test('fall back to the environment when the vault refuses the read', async () => {
		const record = answering({ error: { message: 'permission denied' } });
		expect(await vapidKeysInUse(record, environment)).toEqual({
			publicKey: 'env-public',
			privateKey: 'env-private',
			subject: 'mailto:env@example.com'
		});
	});

	test('fall back rather than sign with half a vaulted pair', async () => {
		const record = answering({ data: { publicKey: 'vault-public', privateKey: null, subject: '' } });
		expect(await vapidKeysInUse(record, environment)).toEqual({
			publicKey: 'env-public',
			privateKey: 'env-private',
			subject: 'mailto:env@example.com'
		});
	});

	test('are nothing at all when neither the vault nor the environment holds them', async () => {
		const record = answering({ data: null });
		expect(await vapidKeysInUse(record, {})).toBeNull();
		expect(await vapidKeysInUse(record, { VAPID_PUBLIC_KEY: 'env-public' })).toBeNull();
	});
});
