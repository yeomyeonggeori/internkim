import { expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('own current state shares fresh reads, forces mutation reads and rejects late authority results', async () => {
	const output = await runInItsOwnProcess(new URL('./my-current-state.fixture.ts', import.meta.url), 'browser');
	expect(JSON.parse(output)).toEqual({ freshShared: true, forceRefreshes: true, undoRefreshes: true, lateReadIgnored: true, lateWriteIgnored: true, partialFailure: true, partialCloseRefreshed: true, mutationClearsReadBusy: true, oldReadKeepsBusyCleared: true });
}, ownProcessTimeoutMilliseconds);
