import { expect, test } from 'bun:test';

test('disposed or globally invalidated work-status reads cannot restore rows or storage', async () => {
	const fixture = new URL('./work-status-lifetime.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, '--conditions', 'browser', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, errors).toBe(0);
	expect(JSON.parse(output)).toEqual({ disposedEmpty: true, invalidatedEmpty: true, stored: 0 });
});
