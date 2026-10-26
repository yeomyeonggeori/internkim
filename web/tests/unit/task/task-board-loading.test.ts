import { test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('scoped task reads keep their API mocks isolated', async () => {
	await runInItsOwnProcess(new URL('task-board-loading.fixture.ts', import.meta.url), 'tests');
}, ownProcessTimeoutMilliseconds);
