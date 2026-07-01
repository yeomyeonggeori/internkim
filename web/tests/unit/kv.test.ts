import { describe, expect, test } from 'bun:test';
import { kv } from '../../src/lib/kv';
import type { KVStore } from '../../src/lib/kv';
import type { UserRecord } from '../../src/lib/types';

class MemoryKV implements KVStore {
	private readonly values = new Map<string, string>();

	async get<T = unknown>(key: string, type: 'json'): Promise<T | null>;
	async get(key: string): Promise<string | null>;
	async get(key: string, type?: 'json'): Promise<unknown> {
		const value = this.values.get(key);
		if (value === undefined) return null;
		if (type === 'json') return JSON.parse(value);
		return value;
	}

	async put(key: string, value: string): Promise<void> {
		this.values.set(key, value);
	}

	async delete(key: string): Promise<void> {
		this.values.delete(key);
	}
}

function userRecord(role: UserRecord['role']): UserRecord {
	return {
		userID: `user-${role}`,
		handle: role.toLowerCase(),
		email: `${role.toLowerCase()}@example.com`,
		role
	};
}

describe('kv user records', () => {
	test('preserves operations admin role when user records are stored and loaded', async () => {
		const store = new MemoryKV();

		await kv.putUserRecords(store, 'fleet-1', [
			userRecord('admin'),
			userRecord('operationsAdmin')
		]);

		const records = await kv.getUserRecords(store, 'fleet-1');

		expect(records.map((record) => record.role)).toEqual(['admin', 'operationsAdmin']);
	});
});
