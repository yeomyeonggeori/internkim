import { describe, expect, test } from 'bun:test';
import { confirmStillServing, keepHostSession, NoLongerServes, type HostSession } from './host-session';

function sessionFor(credential: string, expiresAt: number): HostSession {
	return { companyID: 'company', accessToken: `session-of-${credential}`, expiresAt };
}

describe('keepHostSession', () => {
	test('keeps the session while its credential is unchanged and far from expiring', async () => {
		const exchanged: string[] = [];
		const keeper = await keepHostSession({
			credential: async () => 'box-a',
			exchange: async (credential) => {
				exchanged.push(credential);
				return sessionFor(credential, 10_000);
			},
			nowSeconds: () => 1_000
		});

		expect(await keeper.keepFresh()).toBe(false);
		expect(exchanged).toEqual(['box-a']);
	});

	test('asks again for the credential the box was given in place of the one it started with', async () => {
		let kept = 'box-a';
		const keeper = await keepHostSession({
			credential: async () => kept,
			exchange: async (credential) => sessionFor(credential, 10_000),
			nowSeconds: () => 1_000
		});

		kept = 'box-b';

		expect(await keeper.keepFresh()).toBe(true);
		expect(keeper.current().accessToken).toBe('session-of-box-b');
	});

	test('asks again when the session is about to expire', async () => {
		const keeper = await keepHostSession({
			credential: async () => 'box-a',
			exchange: async (credential) => sessionFor(credential, 1_200),
			nowSeconds: () => 1_000
		});

		expect(await keeper.keepFresh()).toBe(true);
	});
});

describe('confirmStillServing', () => {
	test('passes while the record answers that this computer serves its company', async () => {
		await confirmStillServing(async () => ({ data: 'service', error: null, status: 200 }));
	});

	test('a standby computer no longer serves', async () => {
		await expect(confirmStillServing(async () => ({ data: 'standby', error: null, status: 200 }))).rejects.toBeInstanceOf(NoLongerServes);
	});

	test('a computer its company let go no longer serves', async () => {
		await expect(confirmStillServing(async () => ({ data: null, error: null, status: 200 }))).rejects.toBeInstanceOf(NoLongerServes);
	});

	test('a refused session no longer serves', async () => {
		await expect(confirmStillServing(async () => ({ data: null, error: { message: 'JWT expired' }, status: 401 }))).rejects.toBeInstanceOf(NoLongerServes);
	});

	test('a record that cannot be reached says nothing about serving', async () => {
		const unreachable = confirmStillServing(async () => ({ data: null, error: { message: 'fetch failed' }, status: 0 }));
		await expect(unreachable).rejects.toThrow('fetch failed');
		await expect(unreachable).rejects.not.toBeInstanceOf(NoLongerServes);
	});
});
