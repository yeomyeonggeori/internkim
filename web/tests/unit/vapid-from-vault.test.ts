import { describe, expect, test } from 'bun:test';
import { vapidKeysFromVault } from '../../../supabase/functions/_shared/vapid-from-vault.ts';
import type { SupabaseClient } from '../../../supabase/functions/_shared/service-client.ts';

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
		expect(await vapidKeysFromVault(record)).toEqual({
			publicKey: 'vault-public',
			privateKey: 'vault-private',
			subject: 'mailto:vault@example.com'
		});
	});

	test('are nothing when the vault refuses the read', async () => {
		expect(await vapidKeysFromVault(answering({ error: { message: 'permission denied' } }))).toBeNull();
	});

	test('are nothing rather than half a pair', async () => {
		const halves = [
			{ publicKey: 'vault-public', privateKey: null, subject: 'mailto:vault@example.com' },
			{ publicKey: 'vault-public', privateKey: 'vault-private', subject: '' },
			{ publicKey: '', privateKey: 'vault-private', subject: 'mailto:vault@example.com' }
		];
		for (const half of halves) {
			expect(await vapidKeysFromVault(answering({ data: half }))).toBeNull();
		}
	});

	test('are nothing at all when the vault holds none', async () => {
		expect(await vapidKeysFromVault(answering({ data: null }))).toBeNull();
	});
});
