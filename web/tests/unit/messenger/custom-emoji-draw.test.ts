import { expect, test } from 'bun:test';
import { ownProcessTimeoutMilliseconds, runInItsOwnProcess } from '../../support/in-its-own-process';

test('custom emoji failures retry on a later draw while pending work stays deduplicated across scope changes', async () => {
	const output = await runInItsOwnProcess(new URL('./custom-emoji-draw.fixture.ts', import.meta.url), 'script');
	expect(JSON.parse(output)).toEqual({
		failedImageWasNotStored: true, retriedImage: 'retry-image', retryCalls: 2,
		inFlightCalls: 1, requestsAfterOldFailure: 2, scopedImage: 'new-scope-image'
	});
}, ownProcessTimeoutMilliseconds);
