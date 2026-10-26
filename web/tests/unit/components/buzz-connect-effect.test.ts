import { expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('publishing an identity does not restart the open dialog identity effect', async () => {
	const output = await runInItsOwnProcess(new URL('./buzz-connect-effect.fixture.ts', import.meta.url), 'browser');
	expect(JSON.parse(output)).toEqual({ callsAfterPublish: 2, callsAfterRevalidation: 2, stopped: 1 });
}, ownProcessTimeoutMilliseconds);
