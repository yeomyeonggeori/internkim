import { describe, expect, test } from 'bun:test';
import { isHttpError } from '@sveltejs/kit';
import { isTheRegisterSecret } from '$lib/device-auth';
import { kv, type KVStore } from '$lib/kv';
import { GET as readFleet } from '../../../src/routes/api/fleet/+server';

function storeHolding(records: Record<string, unknown>): KVStore {
	const store = {
		async get(key: string) {
			return records[key] ?? null;
		},
		async put() {},
		async delete() {}
	};
	return store as KVStore;
}

async function statusOf(answer: () => Promise<Response>): Promise<number> {
	try {
		return (await answer()).status;
	} catch (refusal) {
		if (isHttpError(refusal)) return refusal.status;
		throw refusal;
	}
}

describe('the register secret', () => {
	test('is refused when none is configured, whatever the bearer says', async () => {
		expect(await isTheRegisterSecret('Bearer undefined', undefined)).toBe(false);
		expect(await isTheRegisterSecret('Bearer ', '  ')).toBe(false);
	});

	test('is accepted as the bearer the configuration names', async () => {
		expect(await isTheRegisterSecret('Bearer register-secret', 'register-secret\n')).toBe(true);
		expect(await isTheRegisterSecret('Bearer someone-elses', 'register-secret')).toBe(false);
	});

	test('does not let an empty admin token read fleet metadata when the secret is empty', async () => {
		const store = storeHolding({ 'fleet:fleet-one': { fleet_id: 'fleet-one', fleet_secret_hash: 'f'.repeat(64) } });
		expect(await kv.getDevice(store, 'fleet-one')).not.toBeNull();
		const url = new URL('https://intern.example.com/api/fleet?fleet_id=fleet-one&admin_token=');
		const event = { request: new Request(url), url, platform: { env: { KV: store, INTERNKIM_REGISTER_SECRET: '' } } };
		expect(await statusOf(async () => readFleet(event as unknown as Parameters<typeof readFleet>[0]))).toBe(403);
	});
});
