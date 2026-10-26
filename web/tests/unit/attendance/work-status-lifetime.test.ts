import { expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('disposed or globally invalidated work-status reads cannot restore rows or storage', async () => {
	const output = await runInItsOwnProcess(new URL('./work-status-lifetime.fixture.ts', import.meta.url), 'browser');
	expect(JSON.parse(output)).toEqual({ disposedEmpty: true, invalidatedEmpty: true, stored: 0 });
}, ownProcessTimeoutMilliseconds);
