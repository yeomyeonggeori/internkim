import { describe, expect, test } from 'bun:test';
import { kv } from '../../src/lib/kv';
import type { UserRecord } from '../../src/lib/types';

function createMemoryKV(): { namespace: KVNamespace; values: Map<string, string> } {
	const values = new Map<string, string>();
	const namespace = {
		async get(key: string, type?: string): Promise<unknown> {
			const value = values.get(key);
			if (type === 'json') return value ? JSON.parse(value) : null;
			return value ?? null;
		},
		async put(key: string, value: string): Promise<void> {
			values.set(key, value);
		},
		async delete(key: string): Promise<void> {
			values.delete(key);
		}
	} as unknown as KVNamespace;
	return { namespace, values };
}

describe('user record normalization', () => {
	test('preserves trimmed admin notes in fleet user records', async () => {
		const { namespace, values } = createMemoryKV();
		const record: UserRecord = {
			userID: 'user-1',
			handle: 'chanhee',
			name: '이찬희',
			email: 'chanhee@example.com',
			role: 'member',
			note: '  HR compensation follow-up  '
		};

		await kv.putUserRecords(namespace, 'fleet-1', [record]);

		const storedRecords = JSON.parse(values.get('fleet-users:fleet-1') ?? '[]') as UserRecord[];
		expect(storedRecords[0]?.note).toBe('HR compensation follow-up');
		const loadedRecords = await kv.getUserRecords(namespace, 'fleet-1');
		expect(loadedRecords).toMatchObject([
			{
				email: 'chanhee@example.com',
				note: 'HR compensation follow-up'
			}
		]);
	});
});
