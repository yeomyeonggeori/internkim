import { describe, expect, test } from 'bun:test';
import { keepHostSession, type HostSession } from './host-session';

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
