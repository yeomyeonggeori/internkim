import { describe, expect, test } from 'bun:test';
import { waitingForPasskeyApproval, type PasskeyClock } from '../../src/lib/passkey-first-launch';

const refusal = new Error('NotAllowedError');
const isRefusal = (error: unknown) => error === refusal;

function fakeClock(): PasskeyClock & { advance: (milliseconds: number) => void } {
	let current = 0;
	return {
		now: () => current,
		sleep: async (milliseconds) => {
			current += milliseconds;
		},
		advance: (milliseconds) => {
			current += milliseconds;
		}
	};
}

function ceremonyRefusing(times: number, clock: ReturnType<typeof fakeClock>, refusalTakes = 50) {
	let attempts = 0;
	return {
		attempts: () => attempts,
		run: async () => {
			attempts += 1;
			clock.advance(refusalTakes);
			if (attempts <= times) throw refusal;
			return 'signed in';
		}
	};
}

describe('waitingForPasskeyApproval', () => {
	test('the desktop app keeps asking until macOS approves it', async () => {
		const clock = fakeClock();
		const ceremony = ceremonyRefusing(5, clock);
		expect(await waitingForPasskeyApproval(ceremony.run, true, isRefusal, clock)).toBe('signed in');
		expect(ceremony.attempts()).toBe(6);
	});

	test('a browser shows the first refusal', async () => {
		const clock = fakeClock();
		const ceremony = ceremonyRefusing(1, clock);
		await expect(waitingForPasskeyApproval(ceremony.run, false, isRefusal, clock)).rejects.toBe(refusal);
		expect(ceremony.attempts()).toBe(1);
	});

	test('a person cancelling the sheet is not asked again', async () => {
		const clock = fakeClock();
		const ceremony = ceremonyRefusing(1, clock, 4000);
		await expect(waitingForPasskeyApproval(ceremony.run, true, isRefusal, clock)).rejects.toBe(refusal);
		expect(ceremony.attempts()).toBe(1);
	});

	test('any other failure is shown at once', async () => {
		const clock = fakeClock();
		const failure = new Error('the network went away');
		const ceremony = async () => {
			throw failure;
		};
		await expect(waitingForPasskeyApproval(ceremony, true, isRefusal, clock)).rejects.toBe(failure);
	});

	test('waiting stops after twenty seconds', async () => {
		const clock = fakeClock();
		const ceremony = ceremonyRefusing(Number.POSITIVE_INFINITY, clock);
		await expect(waitingForPasskeyApproval(ceremony.run, true, isRefusal, clock)).rejects.toBe(refusal);
		expect(clock.now()).toBeGreaterThanOrEqual(20000);
		expect(clock.now()).toBeLessThan(22000);
	});
});
