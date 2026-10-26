import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

async function outcome() {
	return JSON.parse(await runInItsOwnProcess(new URL('./attachment-renewal.fixture.ts', import.meta.url), 'browser'));
}

describe('a messenger picture whose signed address runs out', () => {
	test('is signed again once the address is about to expire, and not before', async () => {
		expect(await outcome()).toEqual({
			first: 'signed-1',
			whileFresh: 'signed-1',
			afterBrokenImage: 'signed-2',
			afterReopen: 'signed-3',
			signings: 3
		});
	}, ownProcessTimeoutMilliseconds);

});
