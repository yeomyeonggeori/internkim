import { expect, test } from 'bun:test';

test('own current state shares fresh reads, forces mutation reads and rejects late authority results', async () => {
	const fixture = new URL('./my-current-state.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, '--conditions', 'browser', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors] = await Promise.all([new Response(run.stdout).text(), new Response(run.stderr).text()]);
	expect(await run.exited, errors).toBe(0);
	expect(JSON.parse(output)).toEqual({ freshShared: true, forceRefreshes: true, undoRefreshes: true, lateReadIgnored: true, lateWriteIgnored: true, partialFailure: true, partialCloseRefreshed: true });
});
