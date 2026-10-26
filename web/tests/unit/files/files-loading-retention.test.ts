import { test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('workspace loading and retained refresh lifecycle runs in isolation', async () => {
	await runInItsOwnProcess(new URL('files-loading-retention.fixture.ts', import.meta.url), 'tests');
}, ownProcessTimeoutMilliseconds);
