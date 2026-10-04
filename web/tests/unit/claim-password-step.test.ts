import { describe, expect, test } from 'bun:test';
import { hasThePasswordStepExpired, whenTheMailboxWasProved } from '../../src/routes/auth/claim/password-step';

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

describe('when a session last proved it holds the mailbox', () => {
	const provedInSeconds = openedAt / 1000;

	test('a mailed link or code proves it, at the moment it was used', () => {
		expect(whenTheMailboxWasProved([{ method: 'email/signup', timestamp: provedInSeconds }])).toBe(openedAt);
		expect(whenTheMailboxWasProved([{ method: 'otp', timestamp: provedInSeconds }])).toBe(openedAt);
	});

	test('takes the latest proof when the session holds several', () => {
		expect(
			whenTheMailboxWasProved([
				{ method: 'magiclink', timestamp: provedInSeconds - 3600 },
				{ method: 'otp', timestamp: provedInSeconds }
			])
		).toBe(openedAt);
	});

	test('a password or passkey sign-in proves nothing about the mailbox', () => {
		expect(whenTheMailboxWasProved([{ method: 'password', timestamp: provedInSeconds }])).toBe(0);
		expect(whenTheMailboxWasProved([{ method: 'mfa/webauthn', timestamp: provedInSeconds }])).toBe(0);
	});

	test('references without a time prove nothing', () => {
		expect(whenTheMailboxWasProved(['otp'])).toBe(0);
		expect(whenTheMailboxWasProved(undefined)).toBe(0);
	});
});
