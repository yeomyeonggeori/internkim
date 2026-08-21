import { describe, expect, mock, test } from 'bun:test';

type HostAnswer = { status: number; body: unknown };

const asked: string[] = [];
let answer: () => Promise<HostAnswer> = async () => ({ status: 200, body: {} });

mock.module('../../src/lib/host-bridge', () => ({
	callCompanyApp: async ({ capability }: { capability: string }) => {
		asked.push(capability);
		return answer();
	}
}));

const { claimCentralBuzzSecret } = await import('../../src/lib/buzz-identity-central-login');

describe('a company browser claiming its Buzz key', () => {
	test('asks the machine that holds the seed rather than minting one', async () => {
		answer = async () => ({ status: 200, body: { secretHex: 'c'.repeat(64), publicHex: 'd'.repeat(64) } });

		expect(await claimCentralBuzzSecret()).toBe('c'.repeat(64));
		expect(asked.at(-1)).toBe('person.buzz.claim');
	});

	test('a refused claim costs the Buzz app, not the sign-in', async () => {
		answer = async () => ({ status: 404, body: {} });

		expect(await claimCentralBuzzSecret()).toBeNull();
	});

	test('an answer carrying no key does not stop somebody signing in', async () => {
		answer = async () => ({ status: 200, body: {} });

		expect(await claimCentralBuzzSecret()).toBeNull();
	});

	test('a company with nothing to ask does not lock its people out', async () => {
		answer = async () => {
			throw new Error('the company app is unreachable');
		};

		expect(await claimCentralBuzzSecret()).toBeNull();
	});
});
