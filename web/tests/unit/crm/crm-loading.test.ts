import { test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('CRM loading and mutation lifecycle fixtures run in isolation', async () => {
	await runInItsOwnProcess(new URL('crm-loading.fixture.ts', import.meta.url), 'tests');
}, ownProcessTimeoutMilliseconds);
