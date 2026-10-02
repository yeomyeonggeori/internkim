import { expect, test } from 'bun:test';

test('publishing an identity does not restart the open dialog identity effect', async () => {
	const fixture = new URL('./buzz-connect-effect.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, '--conditions', 'browser', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors, exitCode] = await Promise.all([
		new Response(run.stdout).text(), new Response(run.stderr).text(), run.exited
	]);
	expect(exitCode, errors).toBe(0);
	expect(JSON.parse(output)).toEqual({ callsAfterPublish: 2, callsAfterRevalidation: 2, stopped: 1 });
});
