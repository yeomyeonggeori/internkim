import { test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('attendance summary reuse and employee leave reads keep their API fixtures isolated', async () => {
	for (const filename of ['attendance-summary-reuse.fixture.ts', 'employee-leave-parallel-reads.fixture.ts']) {
		await runInItsOwnProcess(new URL(filename, import.meta.url), 'tests');
	}
}, ownProcessTimeoutMilliseconds);
