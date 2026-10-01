import { describe, expect, test } from 'bun:test';
import { isTheSameSecret } from '$lib/server/same-secret';

describe('isTheSameSecret', () => {
	test('accepts the secret it was given', async () => {
		expect(await isTheSameSecret('a-long-shared-secret', 'a-long-shared-secret')).toBe(true);
	});

	test('refuses a secret that differs in one character', async () => {
		expect(await isTheSameSecret('a-long-shared-secreT', 'a-long-shared-secret')).toBe(false);
	});

	test('refuses a prefix of the secret', async () => {
		expect(await isTheSameSecret('a-long-shared', 'a-long-shared-secret')).toBe(false);
	});

	test('refuses everything when no secret is held, an empty offer included', async () => {
		expect(await isTheSameSecret('', '')).toBe(false);
	});
});
