import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

describe('a messenger picture this device could not show', () => {
	test('is opened again once by itself, then keeps its address and waits for a tap to try again', async () => {
		const outcome = JSON.parse(await runInItsOwnProcess(new URL('./attachment-not-shown.fixture.ts', import.meta.url), 'browser'));
		expect(outcome).toEqual({
			first: { status: 'ready', openable: 'signed-1', notShown: false },
			whileOpeningAgain: { status: 'loading', openable: '', notShown: false },
			afterFirstFailure: { status: 'ready', openable: 'signed-2', notShown: false },
			afterSecondFailure: { status: 'ready', openable: 'signed-2', notShown: true },
			afterTap: { status: 'ready', openable: 'signed-3', notShown: false },
			signings: 3
		});
	}, ownProcessTimeoutMilliseconds);
});
