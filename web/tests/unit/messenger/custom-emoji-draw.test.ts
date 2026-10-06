import { expect, test } from 'bun:test';

test('custom emoji failures retry on a later draw while pending work stays deduplicated across scope changes', async () => {
	const fixture = new URL('./custom-emoji-draw.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors, exitCode] = await Promise.all([
		new Response(run.stdout).text(), new Response(run.stderr).text(), run.exited
	]);
	expect(exitCode, errors).toBe(0);
	expect(JSON.parse(output)).toEqual({
		failedImageWasNotStored: true, retriedImage: 'retry-image', retryCalls: 2,
		inFlightCalls: 1, requestsAfterOldFailure: 2, scopedImage: 'new-scope-image'
	});
});
