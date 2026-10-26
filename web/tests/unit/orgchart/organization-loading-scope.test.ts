import { test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('organization cache authority and late-read guards run in isolation', async () => {
	await runInItsOwnProcess(new URL('organization-loading-scope.fixture.ts', import.meta.url), 'tests');
}, ownProcessTimeoutMilliseconds);
