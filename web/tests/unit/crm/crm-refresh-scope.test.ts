import { test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('CRM refresh scope checks isolate API fixtures from other suites', async () => {
	await runInItsOwnProcess(new URL('crm-refresh-scope.fixture.ts', import.meta.url), 'tests');
}, ownProcessTimeoutMilliseconds);
