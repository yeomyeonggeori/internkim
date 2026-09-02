import { describe, expect, test } from 'bun:test';
import { hasThePasswordStepExpired } from '../../src/routes/auth/claim/password-step';

const minute = 60 * 1000;
const openedAt = 1_700_000_000_000;

describe('how long a verified code keeps the password step open', () => {
	test('stays open while somebody is choosing a password', () => {
		expect(hasThePasswordStepExpired(openedAt, openedAt)).toBe(false);
		expect(hasThePasswordStepExpired(openedAt, openedAt + 9 * minute)).toBe(false);
		expect(hasThePasswordStepExpired(openedAt, openedAt + 10 * minute)).toBe(false);
	});

	test('closes on a form left standing, which is a machine somebody walked away from', () => {
		expect(hasThePasswordStepExpired(openedAt, openedAt + 11 * minute)).toBe(true);
		expect(hasThePasswordStepExpired(openedAt, openedAt + 8 * 60 * minute)).toBe(true);
	});

	test('counts a step that was never opened as closed', () => {
		expect(hasThePasswordStepExpired(0, openedAt)).toBe(true);
	});
});
