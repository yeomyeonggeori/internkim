import { test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('the task loader reads every page in its own process', async () => {
	await runInItsOwnProcess(new URL('task-record-pages.fixture.ts', import.meta.url), 'tests');
}, ownProcessTimeoutMilliseconds);
