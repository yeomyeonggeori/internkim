import { describe, expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

async function outcome() {
	return JSON.parse(await runInItsOwnProcess(new URL('./attachment-renewal.fixture.ts', import.meta.url), 'browser'));
}

describe('a messenger attachment whose signed address runs out', () => {
	test('is signed again once the address is about to expire, and not before', async () => {
		expect(await outcome()).toEqual({
			first: 'signed-1',
			whileFresh: 'signed-1',
			afterBrokenImage: 'signed-2',
			afterReopen: 'signed-3',
			signings: 3
		});
	}, ownProcessTimeoutMilliseconds);

	test('is signed again when it is about to be read, once for readers asking together', async () => {
		const read = JSON.parse(await runInItsOwnProcess(new URL('./attachment-readable.fixture.ts', import.meta.url), 'browser'));
		expect(read).toEqual({
			unknown: 'blob:sent',
			whileFresh: 'signed-1',
			signingsWhileFresh: 1,
			afterAnHour: 'signed-2',
			together: 'signed-2',
			signingsAfterAnHour: 2,
			refused: 'the store would not sign for this copy'
		});
	}, ownProcessTimeoutMilliseconds);

	test('signs the copy the company computer already made, and asks for a new one only once that copy is gone', async () => {
		const read = JSON.parse(await runInItsOwnProcess(new URL('./attachment-host-copy.fixture.ts', import.meta.url), 'browser'));
		expect(read).toEqual({
			first: 'copy-1-signed-1',
			afterAnHour: 'copy-1-signed-2',
			copiesAfterAnHour: 1,
			afterTheCopyExpired: 'copy-2-signed-3',
			copies: 2
		});
	}, ownProcessTimeoutMilliseconds);
});
