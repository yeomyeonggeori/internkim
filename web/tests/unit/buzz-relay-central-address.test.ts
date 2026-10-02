import { describe, expect, mock, test } from 'bun:test';

type HostAnswer = { status: number; body: unknown };

const asked: string[] = [];
let answer: () => Promise<HostAnswer> = async () => ({ status: 200, body: {} });

mock.module('../../src/lib/host-bridge', () => ({
	onCompanyEvent: () => () => undefined,
	callCompanyApp: async ({ capability }: { capability: string }) => {
		asked.push(capability);
		return answer();
	}
}));

const { centralBuzzRelayURL } = await import('../../src/lib/buzz-relay-central-address');

describe('a company browser asking where the relay is', () => {
	test('asks the machine that carries the configuration', async () => {
		answer = async () => ({ status: 200, body: { relayURL: ' wss://relay.example.test ' } });

		expect(await centralBuzzRelayURL()).toBe('wss://relay.example.test');
		expect(asked.at(-1)).toBe('person.buzz.relay');
	});

	test('a refused ask leaves the field empty rather than throwing at the dialog', async () => {
		answer = async () => ({ status: 404, body: {} });

		expect(await centralBuzzRelayURL()).toBe('');
	});

	test('an unreachable company does not break opening the dialog', async () => {
		answer = async () => {
			throw new Error('the company app is unreachable');
		};

		expect(await centralBuzzRelayURL()).toBe('');
	});
});
