import { describe, expect, test } from 'bun:test';
import { confirmStillServing, keepHostSession, letGoUnlessStillServing, NoLongerServes, type HostSession, type RoleAnswer } from './host-session';

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

describe('letGoUnlessStillServing', () => {
	async function reasonsLetGoAfter(answer: () => Promise<RoleAnswer>): Promise<string[]> {
		const reasons: string[] = [];
		await letGoUnlessStillServing(answer, (reason) => reasons.push(reason));
		return reasons;
	}

	test('keeps serving when the record cannot be reached', async () => {
		const unreachable = async () => ({ data: null, error: { message: 'TypeError: Unable to connect' }, status: 0 });
		expect(await reasonsLetGoAfter(unreachable)).toEqual([]);
	});

	test('keeps serving when asking the record throws', async () => {
		const failing = async (): Promise<RoleAnswer> => {
			throw new TypeError('Unable to connect');
		};
		expect(await reasonsLetGoAfter(failing)).toEqual([]);
	});

	test('lets go once the record refuses the session', async () => {
		const refused = async () => ({ data: null, error: { message: 'JWT expired' }, status: 401 });
		expect(await reasonsLetGoAfter(refused)).toHaveLength(1);
	});

	test('lets go once the record answers that this computer stands by', async () => {
		const standby = async () => ({ data: 'standby', error: null, status: 200 });
		expect(await reasonsLetGoAfter(standby)).toHaveLength(1);
	});
});
