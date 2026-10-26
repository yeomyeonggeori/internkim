import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

describe('a messenger picture whose first read failed', () => {
	test('is read again the next time the conversation asks for it, and only until it succeeds', async () => {
		const outcome = JSON.parse(await runInItsOwnProcess(new URL('./attachment-retry.fixture.ts', import.meta.url), 'browser'));
		expect(outcome).toEqual({
			afterFirstRead: { status: 'failed', failure: 'Object not found' },
			whileReadingAgain: 'loading',
			afterNextRead: { status: 'ready', openable: 'signed-2' },
			signings: 2
		});
	}, ownProcessTimeoutMilliseconds);
});
